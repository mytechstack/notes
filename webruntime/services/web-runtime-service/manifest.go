package main

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Reads manifests straight from S3 (MinIO locally) — WCR is not on the
// request path. Two kinds of object:
//
//   pointers/{tenant}/{env}.json   small, mutable: refreshed every few
//                                  seconds, or immediately on WCR's webhook
//   manifests/{tenant}/v{n}.json   immutable and environment-agnostic: fetched
//                                  once, cached forever, then "bound" to this
//                                  environment's hosts (see bind)
//
// If S3 is unreachable the last good pointer and manifests keep serving
// (static stability).

type canary struct {
	Version int `json:"version"`
	Percent int `json:"percent"`
}

type pointers struct {
	Stable    int    `json:"stable"`
	Candidate int    `json:"candidate"`
	Canary    canary `json:"canary"`
	UpdatedAt string `json:"updatedAt"`
}

// errNotPublished means S3 has no pointer for this tenant/env.
var errNotPublished = fmt.Errorf("no manifest published")

type manifestStore struct {
	base string
	http *http.Client

	mu        sync.RWMutex
	pointers  map[string]pointers
	manifests map[string]map[string]interface{}
	bound     map[string]map[string]interface{}
}

func newManifestStore(base string) *manifestStore {
	return &manifestStore{
		base:      base,
		http:      &http.Client{Timeout: 3 * time.Second},
		pointers:  map[string]pointers{},
		manifests: map[string]map[string]interface{}{},
		bound:     map[string]map[string]interface{}{},
	}
}

func ptrKey(tenant, env string) string { return tenant + "|" + env }

// getJSON returns errNotPublished for a 404 so callers can tell "missing"
// from "S3 unreachable".
func (s *manifestStore) getJSON(path string, v interface{}) error {
	resp, err := s.http.Get(s.base + "/" + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
		return errNotPublished
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("s3 returned %d for %s", resp.StatusCode, path)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(v)
}

// refresh re-reads the pointer; on failure the previous value is kept.
func (s *manifestStore) refresh(tenant, env string) (pointers, error) {
	var p pointers
	if err := s.getJSON(fmt.Sprintf("pointers/%s/%s.json", tenant, env), &p); err != nil {
		return p, err
	}
	s.mu.Lock()
	s.pointers[ptrKey(tenant, env)] = p
	s.mu.Unlock()
	return p, nil
}

// pointer returns the cached pointer, loading it on first use.
func (s *manifestStore) pointer(tenant, env string) (pointers, error) {
	s.mu.RLock()
	p, ok := s.pointers[ptrKey(tenant, env)]
	s.mu.RUnlock()
	if ok {
		return p, nil
	}
	return s.refresh(tenant, env)
}

func (s *manifestStore) manifest(tenant string, version int) (map[string]interface{}, error) {
	key := fmt.Sprintf("manifests/%s/v%d.json", tenant, version)
	s.mu.RLock()
	m, ok := s.manifests[key]
	s.mu.RUnlock()
	if ok {
		return m, nil
	}
	if err := s.getJSON(key, &m); err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.manifests[key] = m
	s.mu.Unlock()
	return m, nil
}

// boundManifest returns the manifest with this environment's hosts filled
// in. Cached per (env, tenant, version): both inputs are immutable.
func (s *manifestStore) boundManifest(tenant, env string, version int, b envBinding) (map[string]interface{}, error) {
	key := fmt.Sprintf("%s|%s|%d", env, tenant, version)
	s.mu.RLock()
	m, ok := s.bound[key]
	s.mu.RUnlock()
	if ok {
		return m, nil
	}
	raw, err := s.manifest(tenant, version)
	if err != nil {
		return nil, err
	}
	m = bind(raw, b)
	s.mu.Lock()
	s.bound[key] = m
	s.mu.Unlock()
	return m, nil
}

// poll refreshes every known pointer on an interval — the fallback for a
// missed webhook.
func (s *manifestStore) poll(interval time.Duration) {
	for range time.Tick(interval) {
		s.mu.RLock()
		keys := make([][2]string, 0, len(s.pointers))
		for k := range s.pointers {
			keys = append(keys, splitKey(k))
		}
		s.mu.RUnlock()
		for _, k := range keys {
			if _, err := s.refresh(k[0], k[1]); err != nil && err != errNotPublished {
				log.Printf("pointer refresh %s/%s failed, keeping last good: %v", k[0], k[1], err)
			}
		}
	}
}

func splitKey(k string) [2]string {
	t, e, _ := strings.Cut(k, "|")
	return [2]string{t, e}
}

// bucket maps a user to 0-99, deterministically: the same user always
// lands in the same bucket, so raising the canary % only adds users.
func bucket(tenant, user string) int {
	h := fnv.New32a()
	h.Write([]byte(tenant + ":" + user))
	return int(h.Sum32() % 100)
}

// selectVersion is the single place that decides which manifest version a
// request gets: preview → candidate, canary bucket → canary, else stable.
func selectVersion(p pointers, tenant, user, slot string) (int, string) {
	if slot == "candidate" && p.Candidate > 0 {
		return p.Candidate, "preview"
	}
	if p.Canary.Percent > 0 && p.Canary.Version > 0 && bucket(tenant, user) < p.Canary.Percent {
		return p.Canary.Version, "canary"
	}
	return p.Stable, "stable"
}
