package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// In-process L1 manifest cache. RFC's request flow is "L1 cache -> Redis ->
// origin"; this POC collapses Redis out and goes straight to WRCS as
// origin, invalidated by WRCS's webhook rather than a TTL.

type manifestCache struct {
	mu      sync.RWMutex
	entries map[string]map[string]interface{}
}

func newManifestCache() *manifestCache {
	return &manifestCache{entries: make(map[string]map[string]interface{})}
}

func cacheKey(tenant, env, slot string) string {
	return tenant + "|" + env + "|" + slot
}

func (c *manifestCache) get(tenant, env, slot string) (map[string]interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	m, ok := c.entries[cacheKey(tenant, env, slot)]
	return m, ok
}

func (c *manifestCache) set(tenant, env, slot string, m map[string]interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[cacheKey(tenant, env, slot)] = m
}

// invalidate drops both slots for a tenant/env — simplest correct behaviour
// for a POC-sized cache (WRCS's webhook tells us which slot changed, but
// dropping both keeps the L1 cache trivially never-stale for either).
func (c *manifestCache) invalidate(tenant, env string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, cacheKey(tenant, env, "stable"))
	delete(c.entries, cacheKey(tenant, env, "candidate"))
}

type wrcsClient struct {
	baseURL string
	cache   *manifestCache
	http    *http.Client
}

func newWRCSClient(baseURL string, cache *manifestCache) *wrcsClient {
	return &wrcsClient{baseURL: baseURL, cache: cache, http: &http.Client{Timeout: 5 * time.Second}}
}

// fetchManifest returns (manifest, httpStatusFromWRCS, error). A 404 from
// WRCS (no manifest published for this tenant/env/slot) is surfaced via
// status, not err, so callers can render a clean "not found" page.
func (c *wrcsClient) fetchManifest(tenant, env, slot string) (map[string]interface{}, int, error) {
	if m, ok := c.cache.get(tenant, env, slot); ok {
		return m, http.StatusOK, nil
	}
	url := fmt.Sprintf("%s/manifests/%s/%s/%s", c.baseURL, tenant, env, slot)
	resp, err := c.http.Get(url)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, 0, err
	}
	c.cache.set(tenant, env, slot, m)
	return m, http.StatusOK, nil
}
