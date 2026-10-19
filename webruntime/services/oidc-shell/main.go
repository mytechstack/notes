package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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

var homepage = template.Must(template.New("home").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><title>Mock OIDC Shell</title>
<style>
  body { font-family: system-ui, sans-serif; max-width: 760px; margin: 48px auto; padding: 0 16px; color: #1a1a2e; }
  h1 { font-size: 1.4rem; } h2 { font-size: 1.05rem; margin-top: 28px; }
  p.note { color: #555; font-size: 0.9rem; }
  code { background: #f0f0f4; padding: 2px 6px; border-radius: 4px; }
  table { width: 100%; border-collapse: collapse; font-size: 0.9rem; }
  th { text-align: left; color: #666; font-weight: 600; font-size: 0.8rem; }
  th, td { padding: 8px 6px; border-bottom: 1px solid #eee; vertical-align: middle; }
  td.envs a { display: inline-block; padding: 4px 9px; margin: 2px; border-radius: 6px;
              background: #1a1a2e; color: #fff; text-decoration: none; font-size: 0.82rem; }
  td.envs a.deny { background: #a13d3d; }
  .muted { color: #888; font-size: 0.8rem; }
  .warn { color: #a13d3d; }
  form { display: grid; gap: 10px; padding: 16px; border: 1px solid #e2e2ea; border-radius: 8px; }
  label { display: flex; justify-content: space-between; gap: 12px; font-size: 0.9rem; }
  select, input, button { font: inherit; padding: 6px 8px; }
  button { background: #1a1a2e; color: #fff; border: 0; border-radius: 6px; padding: 10px; cursor: pointer; }
</style></head>
<body>
  <h1>Mock OIDC Shell</h1>
  <p class="note">Issues a demo JWT and redirects to the Web Runtime Service. Auth only — this
  service never renders <code>index.html</code> (RFC goal G4).</p>

  <h2>Log in to a tenant</h2>
  {{if .Error}}<p class="note warn">Could not load tenants from the control plane ({{.Error}}); showing the built-in ones.</p>{{end}}
  <table>
    <tr><th>Tenant</th><th>Role</th><th>Open in environment (random user)</th></tr>
    {{range .Tenants}}{{$t := .}}
    <tr>
      <td><strong>{{.Name}}</strong><div class="muted">{{.ID}}</div></td>
      <td><code>{{.Role}}</code></td>
      <td class="envs">{{range $.Envs}}<a href="/login?tenant={{$t.ID}}&role={{$t.Role}}&env={{.}}">{{.}}</a>{{end}}</td>
    </tr>
    {{end}}
    {{with .Default}}
    <tr>
      <td><strong>Wrong role</strong><div class="muted">expect 403</div></td>
      <td><code>wrong-role</code></td>
      <td class="envs"><a class="deny" href="/login?tenant={{.ID}}&role=wrong-role&env=prod">{{.ID}} · prod</a></td>
    </tr>
    {{end}}
  </table>
  <p class="note">A tenant only answers in environments it has been promoted to — new tenants start in
  <code>dev</code>; elsewhere you'll see "Not deployed yet".</p>

  <h2>Custom login</h2>
  <form action="/login" method="get">
    <label>Tenant <input name="tenant" list="tenants" value="{{.Default.ID}}" required></label>
    <datalist id="tenants">{{range .Tenants}}<option value="{{.ID}}">{{.Name}}</option>{{end}}</datalist>
    <label>Role <input name="role" list="roles" value="{{.Default.Role}}" required></label>
    <datalist id="roles">{{range .Tenants}}<option value="{{.Role}}">{{.ID}}</option>{{end}}<option value="wrong-role">expect 403</option></datalist>
    <label>User id <input name="user" placeholder="blank = random"></label>
    <label>Environment
      <select name="env">{{range .Envs}}<option value="{{.}}">{{.}}</option>{{end}}</select></label>
    <label>Version
      <select name="slot">
        <option value="stable">normal (stable / canary by user)</option>
        <option value="candidate">preview candidate</option>
      </select></label>
    <button type="submit">Log in</button>
  </form>
  <p class="note">Canary is decided per user: the same user id always lands on the same version.</p>
</body></html>`))

type tenantRow struct {
	ID, Name, Role string
}

// fallbackTenants is used when the control plane is unreachable.
var fallbackTenants = []tenantRow{
	{ID: "startup-banking", Name: "Startup Banking", Role: "finance-user"},
	{ID: "retail-checkout", Name: "Retail Checkout", Role: "checkout-user"},
}

// listTenants asks the control plane which tenants exist and which role
// each one requires. A real IdP would know its tenants/clients itself;
// in this POC the control plane is the only registry of tenants.
func listTenants(controlPlane string) ([]tenantRow, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	var list struct {
		Tenants []string `json:"tenants"`
	}
	if err := getJSON(client, controlPlane+"/api/tenants", &list); err != nil {
		return nil, err
	}
	rows := make([]tenantRow, 0, len(list.Tenants))
	for _, id := range list.Tenants {
		var cfg struct {
			Config struct {
				DisplayName string   `json:"displayName"`
				Roles       []string `json:"roles"`
			} `json:"config"`
		}
		if err := getJSON(client, controlPlane+"/api/configs/"+url.PathEscape(id), &cfg); err != nil {
			continue
		}
		row := tenantRow{ID: id, Name: cfg.Config.DisplayName}
		if row.Name == "" {
			row.Name = id
		}
		if len(cfg.Config.Roles) > 0 {
			row.Role = cfg.Config.Roles[0]
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no tenants")
	}
	return rows, nil
}

func getJSON(client *http.Client, u string, v interface{}) error {
	resp, err := client.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned %d", u, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

var (
	userRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
	envRe  = regexp.MustCompile(`^[a-z0-9-]{1,32}$`)
)

func randomUser() string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return "user-" + hex.EncodeToString(b)
}

func loginHandler(secret, wrsPublicURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenant := r.URL.Query().Get("tenant")
		role := r.URL.Query().Get("role")
		if tenant == "" || role == "" {
			http.Error(w, "tenant and role query params are required", http.StatusBadRequest)
			return
		}
		user := r.URL.Query().Get("user")
		if user == "" {
			user = randomUser()
		} else if !userRe.MatchString(user) {
			http.Error(w, "user may only contain letters, digits, . _ -", http.StatusBadRequest)
			return
		}
		now := time.Now()
		claims := map[string]interface{}{
			"sub":    user,
			"tenant": tenant,
			"roles":  []string{role},
			"iat":    now.Unix(),
			"exp":    now.Add(1 * time.Hour).Unix(),
		}
		token, err := signJWT(claims, secret)
		if err != nil {
			http.Error(w, "failed to sign token", http.StatusInternalServerError)
			return
		}
		slot := r.URL.Query().Get("slot")
		if slot != "candidate" {
			slot = "stable"
		}
		env := r.URL.Query().Get("env")
		if !envRe.MatchString(env) {
			env = "prod"
		}
		http.Redirect(w, r, fmt.Sprintf("%s/?token=%s&slot=%s&env=%s", wrsPublicURL, token, slot, env), http.StatusFound)
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
	controlPlane := os.Getenv("CONTROL_PLANE_URL")
	if controlPlane == "" {
		controlPlane = "http://control-plane:8093"
	}
	envs := strings.Split(os.Getenv("ENVIRONMENTS"), ",")
	if os.Getenv("ENVIRONMENTS") == "" {
		envs = []string{"dev", "sit", "perf", "uat", "prod"}
	}

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		data := map[string]interface{}{"Envs": envs}
		tenants, err := listTenants(controlPlane)
		if err != nil {
			tenants = fallbackTenants
			data["Error"] = err.Error()
		}
		data["Tenants"] = tenants
		// Default to a tenant that is live in prod (the seeded one), so the
		// "wrong role" link shows a 403 rather than "not deployed".
		data["Default"] = tenants[0]
		for _, t := range tenants {
			if t.ID == "startup-banking" {
				data["Default"] = t
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := homepage.Execute(w, data); err != nil {
			log.Printf("render homepage: %v", err)
		}
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
