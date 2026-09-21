package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
)

// Web Runtime Service (Go BFF) — RFC "Key Components".
//
// Request flow (RFC "Target Architecture"):
//  1. validate the caller's JWT (issued by the mock OIDC Shell)
//  2. read the manifest from WRCS (L1 cache -> origin; Redis omitted in POC)
//  3. evaluate access control in-process via embedded OPA
//  4. render index.html with __PLATFORM_STATE__ injected
//  5. serve the Web Runtime browser shell as static assets

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

func renderMessage(w http.ResponseWriter, status int, title, message string) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><title>%s</title>
<style>body{font-family:system-ui,sans-serif;max-width:560px;margin:80px auto;color:#1a1a2e}
h1{font-size:1.2rem}code{background:#f0f0f4;padding:2px 6px;border-radius:4px}</style>
</head><body><h1>%d — %s</h1><p>%s</p></body></html>`, title, status, title, message)
}

func indexHandler(wrcs *wrcsClient, az *authorizer, jwtSecret string) http.HandlerFunc {
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
			env = "production"
		}
		slot := r.URL.Query().Get("slot")
		if slot == "" {
			slot = "stable"
		}

		manifest, status, err := wrcs.fetchManifest(tenant, env, slot)
		if err != nil {
			renderMessage(w, http.StatusBadGateway, "WRCS unreachable", err.Error())
			return
		}
		if status == http.StatusNotFound {
			renderMessage(w, http.StatusNotFound, "No manifest published",
				fmt.Sprintf("No manifest for tenant=<code>%s</code> env=<code>%s</code> slot=<code>%s</code>.", tenant, env, slot))
			return
		}
		if status != http.StatusOK {
			renderMessage(w, http.StatusBadGateway, "WRCS error", fmt.Sprintf("WRCS returned status %d", status))
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
			"tenant":   tenant,
			"env":      env,
			"slot":     slot,
			"roles":    roles,
			"manifest": manifest,
		}
		stateBytes, err := json.Marshal(state)
		if err != nil {
			renderMessage(w, http.StatusInternalServerError, "Render failed", err.Error())
			return
		}
		// Prevent the JSON from prematurely closing the <script> tag.
		stateJSON := strings.ReplaceAll(string(stateBytes), "</", "<\\/")

		w.Header().Set("Content-Type", "text/html")
		err = indexTmpl.Execute(w, map[string]interface{}{
			"Tenant":    tenant,
			"StateJSON": template.JS(stateJSON),
		})
		if err != nil {
			log.Printf("template execute error: %v", err)
		}
	}
}

func cacheInvalidateHandler(cache *manifestCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			TenantID string `json:"tenantId"`
			Env      string `json:"env"`
			Slot     string `json:"slot"`
			Version  int    `json:"version"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		cache.invalidate(payload.TenantID, payload.Env)
		log.Printf("cache invalidated for tenant=%s env=%s (triggered by slot=%s v%d)", payload.TenantID, payload.Env, payload.Slot, payload.Version)
		w.WriteHeader(http.StatusNoContent)
	}
}

func main() {
	ctx := context.Background()

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET is required")
	}
	wrcsBaseURL := os.Getenv("WRCS_BASE_URL")
	if wrcsBaseURL == "" {
		log.Fatal("WRCS_BASE_URL is required")
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

	cache := newManifestCache()
	wrcs := newWRCSClient(wrcsBaseURL, cache)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", indexHandler(wrcs, az, jwtSecret))
	mux.HandleFunc("POST /internal/cache-invalidate", cacheInvalidateHandler(cache))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("web-runtime-service listening on :%s (wrcs=%s)", port, wrcsBaseURL)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
