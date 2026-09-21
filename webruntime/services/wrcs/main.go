package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Web Runtime Config Service (WRCS)
//
// Stores versioned manifests per tenant per env. Every publish creates a new
// immutable version. "stable" and "candidate" are pointers to a version
// number (blue-green slots), so promotion/rollback are pointer swaps, not
// data copies. On publish/promote/rollback, WRCS notifies registered
// webhooks so the Web Runtime Service can drop its cache without a restart.

var requiredTopLevel = []string{"version", "tenantId", "env", "runtime", "shell"}

func validateManifest(data []byte) (map[string]interface{}, []string, error) {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, nil, err
	}
	var missing []string
	for _, k := range requiredTopLevel {
		if _, ok := m[k]; !ok {
			missing = append(missing, k)
		}
	}
	if shell, ok := m["shell"].(map[string]interface{}); ok {
		nav, ok := shell["nav"].([]interface{})
		if !ok || len(nav) == 0 {
			missing = append(missing, "shell.nav (must be non-empty array)")
		}
		if _, ok := shell["auth"]; !ok {
			missing = append(missing, "shell.auth")
		}
	}
	return m, missing, nil
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string, extra map[string]interface{}) {
	body := map[string]interface{}{"error": msg}
	for k, v := range extra {
		body[k] = v
	}
	writeJSON(w, status, body)
}

func emitWebhooks(tenant, env, slot string, version int) {
	urls := os.Getenv("WEBHOOK_URLS")
	if urls == "" {
		return
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"tenantId": tenant, "env": env, "slot": slot, "version": version,
	})
	for _, u := range strings.Split(urls, ",") {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		go func(url string) {
			client := http.Client{Timeout: 3 * time.Second}
			resp, err := client.Post(url, "application/json", bytes.NewReader(payload))
			if err != nil {
				log.Printf("webhook %s failed: %v", url, err)
				return
			}
			resp.Body.Close()
		}(u)
	}
}

func publishHandler(w http.ResponseWriter, r *http.Request) {
	tenant, env := r.PathValue("tenant"), r.PathValue("env")
	body := new(bytes.Buffer)
	if _, err := body.ReadFrom(r.Body); err != nil {
		writeErr(w, 400, "failed to read body", nil)
		return
	}
	_, missing, err := validateManifest(body.Bytes())
	if err != nil {
		writeErr(w, 400, "invalid json", nil)
		return
	}
	if len(missing) > 0 {
		writeErr(w, 400, "manifest failed schema validation", map[string]interface{}{"missingFields": missing})
		return
	}
	version, err := nextVersion(tenant, env)
	if err != nil {
		writeErr(w, 500, err.Error(), nil)
		return
	}
	if err := writeVersion(tenant, env, version, body.Bytes()); err != nil {
		writeErr(w, 500, err.Error(), nil)
		return
	}
	p, _ := readPointers(tenant, env)
	p.Candidate = version
	if err := writePointers(tenant, env, p); err != nil {
		writeErr(w, 500, err.Error(), nil)
		return
	}
	emitWebhooks(tenant, env, "candidate", version)
	writeJSON(w, 201, map[string]interface{}{"version": version, "slot": "candidate"})
}

func promoteHandler(w http.ResponseWriter, r *http.Request) {
	tenant, env := r.PathValue("tenant"), r.PathValue("env")
	p, err := readPointers(tenant, env)
	if err != nil {
		writeErr(w, 500, err.Error(), nil)
		return
	}
	if p.Candidate == 0 {
		writeErr(w, 409, "no candidate to promote", nil)
		return
	}
	p.Stable = p.Candidate
	if err := writePointers(tenant, env, p); err != nil {
		writeErr(w, 500, err.Error(), nil)
		return
	}
	emitWebhooks(tenant, env, "stable", p.Stable)
	writeJSON(w, 200, map[string]interface{}{"version": p.Stable, "slot": "stable"})
}

func rollbackHandler(w http.ResponseWriter, r *http.Request) {
	tenant, env := r.PathValue("tenant"), r.PathValue("env")
	var req struct {
		Version int    `json:"version"`
		Slot    string `json:"slot"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "invalid json body", nil)
		return
	}
	if req.Slot == "" {
		req.Slot = "stable"
	}
	if req.Slot != "stable" && req.Slot != "candidate" {
		writeErr(w, 400, "slot must be stable or candidate", nil)
		return
	}
	if _, err := readVersion(tenant, env, req.Version); err != nil {
		writeErr(w, 404, fmt.Sprintf("version %d not found", req.Version), nil)
		return
	}
	p, _ := readPointers(tenant, env)
	if req.Slot == "stable" {
		p.Stable = req.Version
	} else {
		p.Candidate = req.Version
	}
	if err := writePointers(tenant, env, p); err != nil {
		writeErr(w, 500, err.Error(), nil)
		return
	}
	emitWebhooks(tenant, env, req.Slot, req.Version)
	writeJSON(w, 200, map[string]interface{}{"version": req.Version, "slot": req.Slot})
}

func getManifestHandler(w http.ResponseWriter, r *http.Request) {
	tenant, env, slot := r.PathValue("tenant"), r.PathValue("env"), r.PathValue("slot")
	if slot != "stable" && slot != "candidate" {
		writeErr(w, 400, "slot must be stable or candidate", nil)
		return
	}
	p, _ := readPointers(tenant, env)
	version := p.Stable
	if slot == "candidate" {
		version = p.Candidate
	}
	if version == 0 {
		writeErr(w, 404, fmt.Sprintf("no manifest published for tenant=%s env=%s slot=%s", tenant, env, slot), nil)
		return
	}
	data, err := readVersion(tenant, env, version)
	if err != nil {
		writeErr(w, 500, err.Error(), nil)
		return
	}
	w.Header().Set("X-Manifest-Version", strconv.Itoa(version))
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func listVersionsHandler(w http.ResponseWriter, r *http.Request) {
	tenant, env := r.PathValue("tenant"), r.PathValue("env")
	versions, err := listVersions(tenant, env)
	if err != nil {
		writeErr(w, 500, err.Error(), nil)
		return
	}
	type entry struct {
		Version     int    `json:"version"`
		PublishedAt string `json:"publishedAt"`
	}
	var out []entry
	for _, v := range versions {
		path := filepath.Join(versionsDir(tenant, env), fmt.Sprintf("%d.json", v))
		info, err := os.Stat(path)
		ts := ""
		if err == nil {
			ts = info.ModTime().UTC().Format(time.RFC3339)
		}
		out = append(out, entry{Version: v, PublishedAt: ts})
	}
	p, _ := readPointers(tenant, env)
	writeJSON(w, 200, map[string]interface{}{"versions": out, "pointers": p})
}

// diffValues produces a flat list of top-level field changes between two
// manifest versions. Nested objects/arrays are compared by value (not
// deep-diffed field-by-field) which is enough to see *that* a section like
// shell.nav changed between versions.
func diffValues(from, to map[string]interface{}) []map[string]interface{} {
	keys := map[string]bool{}
	for k := range from {
		keys[k] = true
	}
	for k := range to {
		keys[k] = true
	}
	var sorted []string
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)

	var changes []map[string]interface{}
	for _, k := range sorted {
		fv, fok := from[k]
		tv, tok := to[k]
		fj, _ := json.Marshal(fv)
		tj, _ := json.Marshal(tv)
		if !fok {
			changes = append(changes, map[string]interface{}{"path": k, "change": "added", "to": tv})
		} else if !tok {
			changes = append(changes, map[string]interface{}{"path": k, "change": "removed", "from": fv})
		} else if string(fj) != string(tj) {
			changes = append(changes, map[string]interface{}{"path": k, "change": "modified", "from": fv, "to": tv})
		}
	}
	return changes
}

func diffHandler(w http.ResponseWriter, r *http.Request) {
	tenant, env := r.PathValue("tenant"), r.PathValue("env")
	fromV, err1 := strconv.Atoi(r.URL.Query().Get("from"))
	toV, err2 := strconv.Atoi(r.URL.Query().Get("to"))
	if err1 != nil || err2 != nil {
		writeErr(w, 400, "from and to query params must be version numbers", nil)
		return
	}
	fromData, err := readVersion(tenant, env, fromV)
	if err != nil {
		writeErr(w, 404, fmt.Sprintf("version %d not found", fromV), nil)
		return
	}
	toData, err := readVersion(tenant, env, toV)
	if err != nil {
		writeErr(w, 404, fmt.Sprintf("version %d not found", toV), nil)
		return
	}
	var fromM, toM map[string]interface{}
	json.Unmarshal(fromData, &fromM)
	json.Unmarshal(toData, &toM)
	writeJSON(w, 200, map[string]interface{}{
		"from": fromV, "to": toV, "changes": diffValues(fromM, toM),
	})
}

// seedFromDisk publishes any manifest found under /seed on first boot, so
// the demo has data without requiring a manual publish step. Only seeds a
// tenant/env that has no versions yet (idempotent across restarts thanks to
// the wrcs-data volume).
func seedFromDisk() {
	seedDir := os.Getenv("SEED_DIR")
	if seedDir == "" {
		seedDir = "/seed"
	}
	entries, err := os.ReadDir(seedDir)
	if err != nil {
		log.Printf("no seed dir at %s, skipping seed", seedDir)
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(seedDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("seed: failed to read %s: %v", path, err)
			continue
		}
		m, missing, err := validateManifest(data)
		if err != nil || len(missing) > 0 {
			log.Printf("seed: %s failed validation: %v %v", path, err, missing)
			continue
		}
		tenant, _ := m["tenantId"].(string)
		env, _ := m["env"].(string)
		if tenant == "" || env == "" {
			continue
		}
		existing, _ := listVersions(tenant, env)
		if len(existing) > 0 {
			log.Printf("seed: %s/%s already has versions, skipping", tenant, env)
			continue
		}
		if err := writeVersion(tenant, env, 1, data); err != nil {
			log.Printf("seed: failed to write version for %s/%s: %v", tenant, env, err)
			continue
		}
		if err := writePointers(tenant, env, Pointers{Stable: 1, Candidate: 1}); err != nil {
			log.Printf("seed: failed to write pointers for %s/%s: %v", tenant, env, err)
			continue
		}
		log.Printf("seed: published %s/%s v1 (stable+candidate)", tenant, env)
	}
}

func main() {
	seedFromDisk()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /manifests/{tenant}/{env}/publish", publishHandler)
	mux.HandleFunc("POST /manifests/{tenant}/{env}/promote", promoteHandler)
	mux.HandleFunc("POST /manifests/{tenant}/{env}/rollback", rollbackHandler)
	mux.HandleFunc("GET /manifests/{tenant}/{env}/versions", listVersionsHandler)
	mux.HandleFunc("GET /manifests/{tenant}/{env}/diff", diffHandler)
	mux.HandleFunc("GET /manifests/{tenant}/{env}/{slot}", getManifestHandler)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8091"
	}
	log.Printf("wrcs listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
