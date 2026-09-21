package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// Mock OIDC Shell — RFC goal G4: "Auth is separate from delivery."
//
// This service does exactly one thing: authenticate a user and hand back a
// JWT. It does NOT serve index.html or know anything about manifests, nav,
// or experiences — that's the Web Runtime Service's job.

func b64url(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

// signJWT builds a minimal HS256 JWT. Hand-rolled (no external dependency)
// since the POC only ever needs to sign here and verify in one other place.
func signJWT(claims map[string]interface{}, secret string) (string, error) {
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	signingInput := b64url(headerJSON) + "." + b64url(claimsJSON)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	sig := b64url(mac.Sum(nil))
	return signingInput + "." + sig, nil
}

const homepage = `<!doctype html>
<html><head><title>Mock OIDC Shell</title>
<style>
  body { font-family: system-ui, sans-serif; max-width: 640px; margin: 64px auto; color: #1a1a2e; }
  h1 { font-size: 1.4rem; }
  p.note { color: #555; }
  a.btn { display: block; padding: 12px 16px; margin: 10px 0; border-radius: 8px;
          background: #1a1a2e; color: #fff; text-decoration: none; }
  a.btn.deny { background: #a13d3d; }
  code { background: #f0f0f4; padding: 2px 6px; border-radius: 4px; }
</style></head>
<body>
  <h1>Mock OIDC Shell</h1>
  <p class="note">Issues a demo JWT and redirects to the Web Runtime Service. Auth only — this
  service never renders <code>index.html</code> (RFC goal G4).</p>
  <a class="btn" href="/login?tenant=retail-checkout&amp;role=checkout-user">Log in as Retail Checkout user</a>
  <a class="btn" href="/login?tenant=wealth-dashboard&amp;role=wealth-user">Log in as Wealth Dashboard user</a>
  <a class="btn deny" href="/login?tenant=retail-checkout&amp;role=wrong-role">Log in with the wrong role (expect 403)</a>
</body></html>`

func loginHandler(secret, wrsPublicURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenant := r.URL.Query().Get("tenant")
		role := r.URL.Query().Get("role")
		if tenant == "" || role == "" {
			http.Error(w, "tenant and role query params are required", http.StatusBadRequest)
			return
		}
		now := time.Now()
		claims := map[string]interface{}{
			"sub":   fmt.Sprintf("demo-user@%s", tenant),
			"tenant": tenant,
			"roles": []string{role},
			"iat":   now.Unix(),
			"exp":   now.Add(1 * time.Hour).Unix(),
		}
		token, err := signJWT(claims, secret)
		if err != nil {
			http.Error(w, "failed to sign token", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("%s/?token=%s&slot=stable", wrsPublicURL, token), http.StatusFound)
	}
}

func main() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatal("JWT_SECRET is required")
	}
	wrsPublicURL := os.Getenv("WRS_PUBLIC_URL")
	if wrsPublicURL == "" {
		wrsPublicURL = "http://localhost:8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(homepage))
	})
	mux.HandleFunc("GET /login", loginHandler(secret, wrsPublicURL))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	log.Printf("oidc-shell listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
