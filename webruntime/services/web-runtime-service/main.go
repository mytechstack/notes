package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// Web Runtime Service (Go BFF) — RFC "Key Components".
//
// Request flow:
//  1. validate the caller's JWT (issued by the mock OIDC Shell)
//  2. read the tenant's pointer from S3 (in memory, refreshed by poll/webhook)
//     and pick a version for this user: preview, canary bucket, or stable
//  3. load that immutable manifest version (cached forever)
//  4. evaluate access control in-process via embedded OPA
//  5. render index.html with __PLATFORM_STATE__ injected

var indexTmpl *template.Template

func tokenFromRequest(r *http.Request) string {
	if t := r.URL.Query().Get("token"); t != "" {
		return t
	}
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

func toStringSlice(v interface{}) []string {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// navItem is the server-rendered nav entry. The manifest already carries
// this data into the Go process (via WRCS) before index.html is written,
// so there's no reason to ship an empty <nav> and build it in JS — SSR'ing
// the markup here means the nav is visible before runtime.js even loads.
type navItem struct {
	ID      string
	Label   string
	Path    string
	Remote  string
	DataURL string
}

// buildNavItems extracts nav entries from the manifest and picks the active
// one, mirroring the old client-side selection (last item with
// "default": true wins; falls back to the first item if none is marked).
func buildNavItems(manifest map[string]interface{}) ([]navItem, string) {
	shell, _ := manifest["shell"].(map[string]interface{})
	rawNav, _ := shell["nav"].([]interface{})
	items := make([]navItem, 0, len(rawNav))
	activeID := ""
	for _, raw := range rawNav {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := m["id"].(string)
		label, _ := m["label"].(string)
		path, _ := m["path"].(string)
		isDefault, _ := m["default"].(bool)
		remote, dataURL := "", ""
		if exp, ok := m["experience"].(map[string]interface{}); ok {
			remote, _ = exp["remote"].(string)
			dataURL, _ = exp["dataUrl"].(string)
		}
		items = append(items, navItem{ID: id, Label: label, Path: path, Remote: remote, DataURL: dataURL})
		if isDefault {
			activeID = id
		}
	}
	if activeID == "" && len(items) > 0 {
		activeID = items[0].ID
	}
	return items, activeID
}

func renderMessage(w http.ResponseWriter, status int, title, message string) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><title>%s</title>
<style>body{font-family:system-ui,sans-serif;max-width:560px;margin:80px auto;color:#1a1a2e}
h1{font-size:1.2rem}code{background:#f0f0f4;padding:2px 6px;border-radius:4px}</style>
</head><body><h1>%d — %s</h1><p>%s</p></body></html>`, title, status, title, message)
}

var (
	slugRe     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	defaultEnv = "prod"
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func indexHandler(store *manifestStore, bindings map[string]envBinding, az *authorizer, jwtSecret string, data *experienceDataClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := tokenFromRequest(r)
		if token == "" {
			renderMessage(w, http.StatusUnauthorized, "No token", "Log in via the OIDC Shell first (http://localhost:8081).")
			return
		}
		claims, err := verifyJWT(token, jwtSecret)
		if err != nil {
			renderMessage(w, http.StatusUnauthorized, "Invalid token", err.Error())
			return
		}
		tenant, _ := claims["tenant"].(string)
		if tenant == "" {
			renderMessage(w, http.StatusBadRequest, "Malformed token", "Token has no tenant claim.")
			return
		}
		roles := claimStringSlice(claims, "roles")

		env := r.URL.Query().Get("env")
		if env == "" {
			env = defaultEnv
		}
		if !slugRe.MatchString(tenant) || !slugRe.MatchString(env) {
			renderMessage(w, http.StatusBadRequest, "Bad request", "Invalid tenant or env.")
			return
		}
		binding, ok := bindings[env]
		if !ok {
			renderMessage(w, http.StatusNotFound, "Unknown environment",
				fmt.Sprintf("No binding for env=<code>%s</code>.", env))
			return
		}
		user, _ := claims["sub"].(string)

		p, err := store.pointer(tenant, env)
		if err != nil && err != errNotPublished {
			renderMessage(w, http.StatusBadGateway, "Manifest store unreachable", err.Error())
			return
		}
		version, selectedBy := selectVersion(p, tenant, user, r.URL.Query().Get("slot"))
		if version == 0 {
			hint := "Promote a version into this environment from the control plane."
			if p.Candidate > 0 {
				hint = fmt.Sprintf("v%d is the candidate here — preview it with <code>slot=candidate</code> or promote it to stable.", p.Candidate)
			}
			renderMessage(w, http.StatusNotFound, "Not deployed in "+env+" yet",
				fmt.Sprintf("Tenant <code>%s</code> has nothing stable in <code>%s</code>. %s", tenant, env, hint))
			return
		}
		manifest, err := store.boundManifest(tenant, env, version, binding)
		if err != nil {
			renderMessage(w, http.StatusBadGateway, "Manifest unavailable",
				fmt.Sprintf("Could not load v%d: %v", version, err))
			return
		}

		var required []string
		if shell, ok := manifest["shell"].(map[string]interface{}); ok {
			if auth, ok := shell["auth"].(map[string]interface{}); ok {
				required = toStringSlice(auth["roles"])
			}
		}

		allowed, err := az.allow(r.Context(), roles, required)
		if err != nil {
			renderMessage(w, http.StatusInternalServerError, "Policy evaluation failed", err.Error())
			return
		}
		if !allowed {
			renderMessage(w, http.StatusForbidden, "Access denied",
				fmt.Sprintf("Caller roles <code>%v</code> do not satisfy required roles <code>%v</code> for tenant <code>%s</code>.", roles, required, tenant))
			return
		}

		state := map[string]interface{}{
			"tenant":          tenant,
			"env":             env,
			"envLabel":        binding.Label,
			"user":            user,
			"roles":           roles,
			"manifestVersion": version,
			"selectedBy":      selectedBy,
			"manifest":        manifest,
		}
		stateBytes, err := json.Marshal(state)
		if err != nil {
			renderMessage(w, http.StatusInternalServerError, "Render failed", err.Error())
			return
		}
		// Prevent the JSON from prematurely closing the <script> tag.
		stateJSON := strings.ReplaceAll(string(stateBytes), "</", "<\\/")

		nav, activeID := buildNavItems(manifest)
		var activeRemote, activeDataURL, preconnectOrigin string
		for _, n := range nav {
			if n.ID == activeID {
				activeRemote = n.Remote
				activeDataURL = n.DataURL
				break
			}
		}
		if activeRemote != "" {
			if u, err := url.Parse(activeRemote); err == nil && u.Scheme != "" && u.Host != "" {
				preconnectOrigin = u.Scheme + "://" + u.Host
			}
		}

		// Server-fetch the active experience's own data, if it declares a
		// dataUrl, so it renders immediately with no client fetch-after-mount
		// round trip. Any failure/timeout here just means no data gets
		// embedded — see experienceDataClient's doc comment.
		var experienceDataJSON template.JS
		if expData, ok := data.fetch(activeDataURL); ok {
			if b, err := json.Marshal(expData); err == nil {
				experienceDataJSON = template.JS(strings.ReplaceAll(string(b), "</", "<\\/"))
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		err = indexTmpl.Execute(w, map[string]interface{}{
			"Frame":              buildFrameView(manifest, tenant),
			"Tenant":             tenant,
			"StateJSON":          template.JS(stateJSON),
			"Nav":                nav,
			"ActiveID":           activeID,
			"ActiveRemote":       activeRemote,
			"PreconnectOrigin":   preconnectOrigin,
			"ExperienceDataJSON": experienceDataJSON,
		})
		if err != nil {
			log.Printf("template execute error: %v", err)
		}
	}
}

// pointerChangedHandler is WCR's webhook: re-read the pointer now instead
// of waiting for the next poll. On failure the last good pointer stays.
func pointerChangedHandler(store *manifestStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			TenantID string `json:"tenantId"`
			Env      string `json:"env"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil ||
			!slugRe.MatchString(payload.TenantID) || !slugRe.MatchString(payload.Env) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		p, err := store.refresh(payload.TenantID, payload.Env)
		if err != nil {
			log.Printf("pointer refresh %s/%s failed, keeping last good: %v", payload.TenantID, payload.Env, err)
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		log.Printf("pointer refreshed %s/%s: stable=v%d candidate=v%d canary=v%d@%d%%",
			payload.TenantID, payload.Env, p.Stable, p.Candidate, p.Canary.Version, p.Canary.Percent)
		w.WriteHeader(http.StatusNoContent)
	}
}

// assignHandler answers "which version would these users get?" through
// the same selectVersion path real requests use. Read-only; demo aid.
func assignHandler(store *manifestStore, bindings map[string]envBinding) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")
		q := r.URL.Query()
		tenant, env := q.Get("tenant"), q.Get("env")
		if env == "" {
			env = defaultEnv
		}
		if _, ok := bindings[env]; !ok || !slugRe.MatchString(tenant) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid tenant or unknown environment"})
			return
		}
		p, err := store.pointer(tenant, env)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		type assignment struct {
			User       string `json:"user"`
			Bucket     int    `json:"bucket"`
			Version    int    `json:"version"`
			SelectedBy string `json:"selectedBy"`
		}
		users := strings.Split(q.Get("users"), ",")
		if len(users) > 500 {
			users = users[:500]
		}
		out := make([]assignment, 0, len(users))
		for _, u := range users {
			if u = strings.TrimSpace(u); u == "" {
				continue
			}
			v, by := selectVersion(p, tenant, u, "")
			out = append(out, assignment{User: u, Bucket: bucket(tenant, u), Version: v, SelectedBy: by})
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"pointers": p, "assignments": out})
	}
}

func main() {
	ctx := context.Background()

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET is required")
	}
	manifestBase := os.Getenv("MANIFEST_BASE_URL")
	if manifestBase == "" {
		log.Fatal("MANIFEST_BASE_URL is required")
	}
	policyPath := os.Getenv("POLICY_PATH")
	if policyPath == "" {
		policyPath = "opa/policy.rego"
	}
	templatePath := os.Getenv("TEMPLATE_PATH")
	if templatePath == "" {
		templatePath = "templates/index.html.tmpl"
	}
	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "static"
	}

	var err error
	indexTmpl, err = template.ParseFiles(templatePath)
	if err != nil {
		log.Fatalf("failed to parse template: %v", err)
	}

	az, err := newAuthorizer(ctx, policyPath)
	if err != nil {
		log.Fatalf("failed to load OPA policy: %v", err)
	}

	bindings, err := loadBindings(envOr("ENV_BINDINGS", "/etc/web-runtime/envs.json"))
	if err != nil {
		log.Fatalf("failed to load environment bindings: %v", err)
	}
	defaultEnv = envOr("DEFAULT_ENV", "prod")
	if _, ok := bindings[defaultEnv]; !ok {
		log.Fatalf("DEFAULT_ENV %q has no binding", defaultEnv)
	}

	store := newManifestStore(manifestBase)
	go store.poll(5 * time.Second)
	expData := newExperienceDataClient()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", indexHandler(store, bindings, az, jwtSecret, expData))
	mux.HandleFunc("POST /internal/cache-invalidate", pointerChangedHandler(store))
	mux.HandleFunc("GET /debug/assign", assignHandler(store, bindings))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("web-runtime-service listening on :%s (manifests=%s)", port, manifestBase)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
