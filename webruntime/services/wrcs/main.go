package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Web Composition Registry (WCR; service name wrcs)
//
// Generates ONE environment-agnostic manifest per config revision
// (events.go, generate.go) and stores it as an immutable version in S3
// (store.go). Each environment (dev, sit, perf, uat, prod) has its own
// pointer: stable, candidate, canary. A version reaches an environment only
// by promotion from the previous one (envs.go), so the file UAT signed off
// is the file prod runs. WCR is not on the request path: the Web Runtime
// Service reads S3 directly and is pinged after every pointer change.

var requiredTopLevel = []string{"version", "tenantId", "runtime", "shell"}

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

func validTenant(w http.ResponseWriter, tenant string) bool {
	if !slugRe.MatchString(tenant) {
		writeErr(w, 400, "invalid tenant", nil)
		return false
	}
	return true
}

func validTenantEnv(w http.ResponseWriter, tenant, env string) bool {
	if !validTenant(w, tenant) {
		return false
	}
	if !validEnv(env) {
		writeErr(w, 404, fmt.Sprintf("unknown environment %q (have %v)", env, environments), nil)
		return false
	}
	return true
}

// ---- per-environment pointer actions ----

// promoteHandler: within ONE environment, candidate becomes stable.
func promoteHandler(w http.ResponseWriter, r *http.Request) {
	tenant, env := r.PathValue("tenant"), r.PathValue("env")
	if !validTenantEnv(w, tenant, env) {
		return
	}
	writeMu.Lock()
	defer writeMu.Unlock()
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
	p.Canary = Canary{}
	if err := writePointers(tenant, env, p); err != nil {
		writeErr(w, 500, err.Error(), nil)
		return
	}
	_ = recordRelease(tenant, p.Stable, env, "stable")
	emitWebhooks(tenant, env, "stable", p.Stable)
	writeJSON(w, 200, map[string]interface{}{"env": env, "version": p.Stable, "slot": "stable"})
}

// canaryHandler sends `percent` of this environment's users to its
// candidate; 0 stops the canary. The Web Runtime Service does the split.
func canaryHandler(w http.ResponseWriter, r *http.Request) {
	tenant, env := r.PathValue("tenant"), r.PathValue("env")
	if !validTenantEnv(w, tenant, env) {
		return
	}
	var req struct {
		Percent int `json:"percent"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Percent < 0 || req.Percent > 100 {
		writeErr(w, 400, "body must be {\"percent\": 0-100}", nil)
		return
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	p, err := readPointers(tenant, env)
	if err != nil {
		writeErr(w, 500, err.Error(), nil)
		return
	}
	if req.Percent == 0 {
		p.Canary = Canary{}
	} else {
		if p.Stable == 0 {
			writeErr(w, 409, "first release in "+env+": nothing is stable yet, so there is no one to compare against — promote the candidate to stable", nil)
			return
		}
		if p.Candidate == 0 || p.Candidate == p.Stable {
			writeErr(w, 409, "candidate is the same as stable — promote a new version into "+env+" first", nil)
			return
		}
		p.Canary = Canary{Version: p.Candidate, Percent: req.Percent}
	}
	if err := writePointers(tenant, env, p); err != nil {
		writeErr(w, 500, err.Error(), nil)
		return
	}
	emitWebhooks(tenant, env, "canary", p.Canary.Version)
	writeJSON(w, 200, p)
}

// rollbackHandler points a slot of ONE environment at any existing version.
// Pointing stable at an older version is the instant rollback.
func rollbackHandler(w http.ResponseWriter, r *http.Request) {
	tenant, env := r.PathValue("tenant"), r.PathValue("env")
	if !validTenantEnv(w, tenant, env) {
		return
	}
	writeMu.Lock()
	defer writeMu.Unlock()
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
	if _, err := readVersion(tenant, req.Version); err != nil {
		writeErr(w, 404, fmt.Sprintf("version %d not found", req.Version), nil)
		return
	}
	p, err := readPointers(tenant, env)
	if err != nil {
		writeErr(w, 500, err.Error(), nil)
		return
	}
	if req.Slot == "stable" {
		p.Stable = req.Version
		p.Canary = Canary{} // a rollback always ends any running canary
	} else {
		p.Candidate = req.Version
	}
	if err := writePointers(tenant, env, p); err != nil {
		writeErr(w, 500, err.Error(), nil)
		return
	}
	_ = recordRelease(tenant, req.Version, env, req.Slot)
	emitWebhooks(tenant, env, req.Slot, req.Version)
	writeJSON(w, 200, map[string]interface{}{"env": env, "version": req.Version, "slot": req.Slot})
}

// ---- cross-environment promotion ----

// promoteAcrossHandler moves a version to the NEXT environment's candidate
// slot. Gate: `to` must directly follow `from`, and the version must have
// been stable in `from` (per its release record).
func promoteAcrossHandler(w http.ResponseWriter, r *http.Request) {
	tenant := r.PathValue("tenant")
	if !validTenant(w, tenant) {
		return
	}
	var req struct {
		Version int    `json:"version"`
		From    string `json:"from"`
		To      string `json:"to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Version < 1 || !validEnv(req.From) || !validEnv(req.To) {
		writeErr(w, 400, fmt.Sprintf("body must be {\"version\": n, \"from\": env, \"to\": env} with envs from %v", environments), nil)
		return
	}
	if next := nextEnv(req.From); next != req.To {
		msg := fmt.Sprintf("%s → %s is not allowed: versions move one environment at a time, and %s comes next after %s",
			req.From, req.To, next, req.From)
		if next == "" {
			msg = req.From + " is the last environment"
		}
		writeErr(w, 409, msg, map[string]interface{}{"order": environments})
		return
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	rel, err := readRelease(tenant, req.Version)
	if err != nil {
		writeErr(w, 500, err.Error(), nil)
		return
	}
	if !contains(rel.StableIn, req.From) {
		writeErr(w, 409, fmt.Sprintf("v%d has not been stable in %s yet — promote it to stable there first", req.Version, req.From),
			map[string]interface{}{"stableIn": rel.StableIn})
		return
	}
	p, err := readPointers(tenant, req.To)
	if err != nil {
		writeErr(w, 500, err.Error(), nil)
		return
	}
	p.Candidate = req.Version
	if p.Canary.Percent > 0 && p.Canary.Version != req.Version {
		p.Canary = Canary{} // a canary of an older candidate no longer applies
	}
	if err := writePointers(tenant, req.To, p); err != nil {
		writeErr(w, 500, err.Error(), nil)
		return
	}
	_ = recordRelease(tenant, req.Version, req.To, "promoted-from-"+req.From)
	emitWebhooks(tenant, req.To, "candidate", req.Version)
	writeJSON(w, 200, map[string]interface{}{"version": req.Version, "env": req.To, "slot": "candidate"})
}

// ---- reads ----

func getManifestHandler(w http.ResponseWriter, r *http.Request) {
	tenant, env, slot := r.PathValue("tenant"), r.PathValue("env"), r.PathValue("slot")
	if !validTenantEnv(w, tenant, env) {
		return
	}
	if slot != "stable" && slot != "candidate" {
		writeErr(w, 400, "slot must be stable or candidate", nil)
		return
	}
	p, err := readPointers(tenant, env)
	if err != nil {
		writeErr(w, 500, err.Error(), nil)
		return
	}
	version := p.Stable
	if slot == "candidate" {
		version = p.Candidate
	}
	if version == 0 {
		writeErr(w, 404, fmt.Sprintf("nothing deployed for tenant=%s env=%s slot=%s", tenant, env, slot), nil)
		return
	}
	data, err := readVersion(tenant, version)
	if err != nil {
		writeErr(w, 500, err.Error(), nil)
		return
	}
	w.Header().Set("X-Manifest-Version", strconv.Itoa(version))
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func listTenantsHandler(w http.ResponseWriter, r *http.Request) {
	tenants, err := listTenants()
	if err != nil {
		writeErr(w, 500, err.Error(), nil)
		return
	}
	writeJSON(w, 200, map[string]interface{}{"tenants": tenants, "environments": environments})
}

// listVersionsHandler: every version of a tenant with its release record,
// plus every environment's pointer — the whole pipeline in one read.
func listVersionsHandler(w http.ResponseWriter, r *http.Request) {
	tenant := r.PathValue("tenant")
	if !validTenant(w, tenant) {
		return
	}
	versions, err := listVersionEntries(tenant)
	if err != nil {
		writeErr(w, 500, err.Error(), nil)
		return
	}
	type entry struct {
		Version        int            `json:"version"`
		PublishedAt    string         `json:"publishedAt"`
		SourceRevision int            `json:"sourceRevision"`
		StableIn       []string       `json:"stableIn"`
		History        []HistoryEntry `json:"history"`
	}
	out := []entry{}
	for _, v := range versions {
		rel, _ := readRelease(tenant, v.N)
		out = append(out, entry{
			Version: v.N, PublishedAt: v.Modified.UTC().Format(time.RFC3339),
			SourceRevision: rel.SourceRevision, StableIn: rel.StableIn, History: rel.History,
		})
	}
	pointers := map[string]Pointers{}
	for _, env := range environments {
		p, err := readPointers(tenant, env)
		if err != nil {
			writeErr(w, 500, err.Error(), nil)
			return
		}
		pointers[env] = p
	}
	writeJSON(w, 200, map[string]interface{}{"environments": environments, "versions": out, "pointers": pointers})
}

func getVersionHandler(w http.ResponseWriter, r *http.Request) {
	tenant := r.PathValue("tenant")
	if !validTenant(w, tenant) {
		return
	}
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil {
		writeErr(w, 400, "version must be a number", nil)
		return
	}
	data, err := readVersion(tenant, n)
	if err != nil {
		writeErr(w, 404, fmt.Sprintf("version %d not found", n), nil)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
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
	tenant := r.PathValue("tenant")
	if !validTenant(w, tenant) {
		return
	}
	fromV, err1 := strconv.Atoi(r.URL.Query().Get("from"))
	toV, err2 := strconv.Atoi(r.URL.Query().Get("to"))
	if err1 != nil || err2 != nil {
		writeErr(w, 400, "from and to query params must be version numbers", nil)
		return
	}
	fromData, err := readVersion(tenant, fromV)
	if err != nil {
		writeErr(w, 404, fmt.Sprintf("version %d not found", fromV), nil)
		return
	}
	toData, err := readVersion(tenant, toV)
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

// withCORS lets the control-plane UI (a separate origin/port) call this
// API directly from the browser; JSON POSTs trigger a preflight OPTIONS.
func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func main() {
	initEnvironments()
	if err := initStore(); err != nil {
		log.Fatalf("S3 store unavailable: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /events", eventsHandler)
	mux.HandleFunc("GET /events", listEventsHandler)
	mux.HandleFunc("GET /tenants", listTenantsHandler)

	// tenant level: versions are shared by all environments
	mux.HandleFunc("GET /manifests/{tenant}/versions", listVersionsHandler)
	mux.HandleFunc("GET /manifests/{tenant}/versions/{n}", getVersionHandler)
	mux.HandleFunc("GET /manifests/{tenant}/diff", diffHandler)
	mux.HandleFunc("POST /manifests/{tenant}/promote", promoteAcrossHandler)

	// environment level: one pointer each
	mux.HandleFunc("POST /manifests/{tenant}/{env}/promote", promoteHandler)
	mux.HandleFunc("POST /manifests/{tenant}/{env}/canary", canaryHandler)
	mux.HandleFunc("POST /manifests/{tenant}/{env}/rollback", rollbackHandler)
	mux.HandleFunc("GET /manifests/{tenant}/{env}/{slot}", getManifestHandler)

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})

	port := envOr("PORT", "8091")
	log.Printf("wrcs listening on :%s (bucket %s, environments %v)", port, bucket, environments)
	log.Fatal(http.ListenAndServe(":"+port, withCORS(mux)))
}
