package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Control plane (authoring plane). Owns structured tenant config — one per
// tenant, not per environment — and the module catalog; never produces a
// manifest itself. On every config save it emits a config.changed event to
// WCR, which pulls GET /api/configs/{tenant}/revisions/{n} and generates
// one environment-agnostic manifest that is then promoted dev → … → prod.

type server struct {
	store     *configStore
	modules   map[string]Module
	templates map[string]Template
	outbox    *outbox
}

// Template: a starting set of pages for a new tenant. Versions are not
// stored — creating a tenant pins each page to the module's latest.
type Template struct {
	Description string `json:"description"`
	Pages       []Page `json:"pages"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string, details []string) {
	body := map[string]interface{}{"error": msg}
	if len(details) > 0 {
		body["details"] = details
	}
	writeJSON(w, status, body)
}

func tenantParam(r *http.Request) (string, bool) {
	t := r.PathValue("tenant")
	return t, slug.MatchString(t)
}

func (s *server) listTenants(w http.ResponseWriter, r *http.Request) {
	tenants, err := s.store.tenants()
	if err != nil {
		writeErr(w, 500, err.Error(), nil)
		return
	}
	if tenants == nil {
		tenants = []string{}
	}
	writeJSON(w, 200, map[string]interface{}{"tenants": tenants})
}

func (s *server) listModules(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{"modules": s.modules})
}

func (s *server) getConfig(w http.ResponseWriter, r *http.Request) {
	tenant, ok := tenantParam(r)
	if !ok {
		writeErr(w, 400, "invalid tenant", nil)
		return
	}
	c, rev, err := s.store.latest(tenant)
	if err != nil {
		writeErr(w, 500, err.Error(), nil)
		return
	}
	if rev == 0 {
		writeErr(w, 404, "no config for "+tenant, nil)
		return
	}
	writeJSON(w, 200, map[string]interface{}{"revision": rev, "config": c})
}

func (s *server) putConfig(w http.ResponseWriter, r *http.Request) {
	tenant, ok := tenantParam(r)
	if !ok {
		writeErr(w, 400, "invalid tenant", nil)
		return
	}
	var c Config
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&c); err != nil {
		writeErr(w, 400, "invalid json: "+err.Error(), nil)
		return
	}
	c.TenantID = tenant
	cleaned := c.Roles[:0]
	for _, role := range c.Roles {
		if role = strings.TrimSpace(role); role != "" {
			cleaned = append(cleaned, role)
		}
	}
	c.Roles = cleaned
	if c.Frame.Header.Module == "" && c.Frame.Nav.Module == "" && c.Frame.Footer.Module == "" {
		c.Frame = defaultFrame(firstNonEmpty(c.DisplayName, tenant), s.modules)
	}
	pinVersions(&c, s.modules)
	if errs := validateConfig(c, s.modules); len(errs) > 0 {
		writeErr(w, 400, "config failed validation", errs)
		return
	}
	rev, err := s.store.save(c)
	if err != nil {
		writeErr(w, 500, err.Error(), nil)
		return
	}
	e := s.outbox.enqueue(tenant, rev, false)
	writeJSON(w, 201, map[string]interface{}{"revision": rev, "eventId": e.ID})
}

func (s *server) listTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{"templates": s.templates})
}

// createTenant registers a new tenant from a template. Its first build
// goes live in dev only; every other environment gets it by promotion.
func (s *server) createTenant(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TenantID    string `json:"tenantId"`
		DisplayName string `json:"displayName"`
		Template    string `json:"template"`
		Role        string `json:"role"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeErr(w, 400, "invalid json: "+err.Error(), nil)
		return
	}
	req.TenantID = strings.TrimSpace(req.TenantID)
	if !slug.MatchString(req.TenantID) {
		writeErr(w, 400, "tenant id must be a lowercase slug, e.g. acme-bank", nil)
		return
	}
	tpl, ok := s.templates[req.Template]
	if !ok {
		writeErr(w, 400, fmt.Sprintf("unknown template %q", req.Template), nil)
		return
	}
	if _, rev, _ := s.store.latest(req.TenantID); rev > 0 {
		writeErr(w, 409, fmt.Sprintf("tenant %q already exists", req.TenantID), nil)
		return
	}
	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = req.TenantID + "-user"
	}
	name := strings.TrimSpace(req.DisplayName)
	if name == "" {
		name = req.TenantID
	}
	c := Config{
		TenantID:       req.TenantID,
		DisplayName:    name,
		RuntimeVersion: "1.7.0",
		Roles:          []string{role},
		Pages:          append([]Page(nil), tpl.Pages...),
		Frame:          defaultFrame(name, s.modules),
	}
	pinVersions(&c, s.modules)
	if errs := validateConfig(c, s.modules); len(errs) > 0 {
		writeErr(w, 400, "template produced an invalid config", errs)
		return
	}
	rev, err := s.store.save(c)
	if err != nil {
		writeErr(w, 500, err.Error(), nil)
		return
	}
	e := s.outbox.enqueue(c.TenantID, rev, false)
	writeJSON(w, 201, map[string]interface{}{"tenantId": c.TenantID, "revision": rev, "eventId": e.ID, "role": role})
}

// getSnapshot is what WCR pulls when it handles a config.changed event:
// the config at that exact revision plus the catalog entries it
// references.
func (s *server) getSnapshot(w http.ResponseWriter, r *http.Request) {
	tenant, ok := tenantParam(r)
	n, err := strconv.Atoi(r.PathValue("n"))
	if !ok || err != nil {
		writeErr(w, 400, "invalid tenant/revision", nil)
		return
	}
	c, err := s.store.revision(tenant, n)
	if err != nil {
		writeErr(w, 404, "revision not found", nil)
		return
	}
	// Exactly the module versions this revision pins, keyed "module@version".
	mods := map[string]resolvedModule{}
	for _, p := range c.Pages {
		m, ok := s.modules[p.Module]
		if !ok {
			continue
		}
		if v, ok := m.Versions[p.ModuleVersion]; ok {
			mods[p.Module+"@"+p.ModuleVersion] = resolvedModule{
				Path: v.Path, Scope: m.Scope, Version: p.ModuleVersion, DeployedAt: v.DeployedAt, DataPath: v.DataPath,
			}
		}
	}
	for k, v := range frameModules(c.Frame, s.modules) {
		mods[k] = v
	}
	writeJSON(w, 200, map[string]interface{}{
		"tenantId": tenant, "revision": n, "config": c, "modules": mods,
	})
}

func (s *server) listEvents(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{"events": s.outbox.snapshot()})
}

func loadModules(path string) (map[string]Module, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]Module
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	for id, mod := range m {
		if mod.Kind == "" {
			mod.Kind = "experience"
			m[id] = mod
		}
	}
	return m, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func loadTemplates(dir string) (map[string]Template, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	out := map[string]Template{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var t Template
		if err := json.Unmarshal(data, &t); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		out[strings.TrimSuffix(filepath.Base(f), ".json")] = t
	}
	return out, nil
}

// seedConfigs writes revision 1 for any seed tenant that has no config
// yet (idempotent across restarts thanks to the data volume) and returns
// the tenants it just created.
func seedConfigs(s *server, seedDir string) map[string]bool {
	seeded := map[string]bool{}
	files, _ := filepath.Glob(filepath.Join(seedDir, "configs", "*.json"))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var c Config
		if err := json.Unmarshal(data, &c); err != nil {
			log.Printf("seed: %s: %v", f, err)
			continue
		}
		if c.Frame.Header.Module == "" {
			c.Frame = defaultFrame(firstNonEmpty(c.DisplayName, c.TenantID), s.modules)
		}
		pinVersions(&c, s.modules)
		if errs := validateConfig(c, s.modules); len(errs) > 0 {
			log.Printf("seed: %s failed validation: %v", f, errs)
			continue
		}
		if _, rev, _ := s.store.latest(c.TenantID); rev > 0 {
			continue
		}
		if _, err := s.store.save(c); err != nil {
			log.Printf("seed: %s: %v", f, err)
			continue
		}
		seeded[c.TenantID] = true
		log.Printf("seed: %s revision 1", c.TenantID)
	}
	return seeded
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	dataDir := envOr("DATA_DIR", "/data")
	seedDir := envOr("SEED_DIR", "seed")
	wcrEvents := envOr("WRCS_EVENTS_URL", "http://wrcs:8091/events")
	port := envOr("PORT", "8093")

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatal(err)
	}
	modules, err := loadModules(filepath.Join(seedDir, "modules.json"))
	if err != nil {
		log.Fatalf("failed to load module catalog: %v", err)
	}
	templates, err := loadTemplates(filepath.Join(seedDir, "templates"))
	if err != nil {
		log.Fatalf("failed to load templates: %v", err)
	}
	s := &server{
		store:     newConfigStore(dataDir),
		modules:   modules,
		templates: templates,
		outbox:    newOutbox(filepath.Join(dataDir, "outbox.json"), wcrEvents),
	}
	seeded := seedConfigs(s, seedDir)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/tenants", s.listTenants)
	mux.HandleFunc("POST /api/tenants", s.createTenant)
	mux.HandleFunc("GET /api/templates", s.listTemplates)
	mux.HandleFunc("GET /api/modules", s.listModules)
	mux.HandleFunc("GET /api/configs/{tenant}", s.getConfig)
	mux.HandleFunc("PUT /api/configs/{tenant}", s.putConfig)
	mux.HandleFunc("GET /api/configs/{tenant}/revisions/{n}", s.getSnapshot)
	mux.HandleFunc("GET /api/events", s.listEvents)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	// no-cache: browsers revalidate on every load, so a stale app.js can
	// never be paired with a newer index.html after a rebuild.
	static := http.FileServer(http.Dir("public"))
	mux.Handle("GET /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		static.ServeHTTP(w, r)
	}))

	// Listen before emitting: WCR calls back into the snapshot endpoint
	// while handling the boot-time events.
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("control-plane listening on :%s (events -> %s)", port, wcrEvents)

	// Re-announce every tenant's latest revision on boot. WCR dedupes by
	// revision, so this is a no-op normally and re-converges WCR if its
	// data was wiped. Tenants seeded just now are pre-existing tenants:
	// flag them so their first build goes live in every environment.
	tenants, _ := s.store.tenants()
	for _, t := range tenants {
		if _, rev, err := s.store.latest(t); err == nil && rev > 0 {
			s.outbox.enqueue(t, rev, seeded[t])
		}
	}
	go s.outbox.run()

	log.Fatal(http.Serve(ln, mux))
}
