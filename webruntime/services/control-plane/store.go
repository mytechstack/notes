package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Authoring config, revisioned, one per tenant — authored once and
// promoted through every environment as the same generated manifest. WCR
// never sees it directly: it pulls a revision snapshot and generates.
//
// Layout on disk (DATA_DIR, default /data):
//   configs/{tenant}/revisions/{n}.json   immutable config snapshot

type Page struct {
	ID            string `json:"id"`
	Label         string `json:"label"`
	Icon          string `json:"icon"`
	Default       bool   `json:"default"`
	Module        string `json:"module"`
	ModuleVersion string `json:"moduleVersion"`
}

type Config struct {
	TenantID       string   `json:"tenantId"`
	DisplayName    string   `json:"displayName"`
	RuntimeVersion string   `json:"runtimeVersion"`
	Roles          []string `json:"roles"`
	Pages          []Page   `json:"pages"`
	Frame          Frame    `json:"frame"` // header, nav, footer + theme (frame.go)
}

// Module catalog: modules are shared, independent of tenants. Each has
// several released versions; a tenant's page pins one of them. Locations
// are environment-free paths; each environment's Web Runtime Service
// prefixes them with its own CDAaS / data host.
type ModuleVersion struct {
	Path       string `json:"path"`
	DeployedAt string `json:"deployedAt"`
	DataPath   string `json:"dataPath,omitempty"`
}

// Kind is "experience" (a page) or "frame" (header, nav or footer — see
// Slot). Frame modules also declare the settings a tenant can fill in.
type Module struct {
	Kind     string                   `json:"kind"`
	Slot     string                   `json:"slot,omitempty"`
	Scope    string                   `json:"scope"`
	Latest   string                   `json:"latest"`
	Settings []SettingDef             `json:"settings,omitempty"`
	Versions map[string]ModuleVersion `json:"versions"`
}

// resolvedModule is one exact module version, as WCR receives it in a
// snapshot (keyed "module@version").
type resolvedModule struct {
	Path       string `json:"path"`
	Scope      string `json:"scope"`
	Version    string `json:"version"`
	DeployedAt string `json:"deployedAt"`
	DataPath   string `json:"dataPath,omitempty"`
}

// pinVersions fills empty page and frame versions with the module's
// latest (and missing frame settings with their defaults), so stored config
// always names exact versions.
func pinVersions(c *Config, modules map[string]Module) {
	for i := range c.Pages {
		if m, ok := modules[c.Pages[i].Module]; ok && c.Pages[i].ModuleVersion == "" {
			c.Pages[i].ModuleVersion = m.Latest
		}
	}
	normalizeFrame(&c.Frame, modules)
}

// slug guards every identifier that ends up in a file path (here and in
// WCR) or a URL path segment.
var slug = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func validateConfig(c Config, modules map[string]Module) []string {
	var errs []string
	if !slug.MatchString(c.TenantID) {
		errs = append(errs, "tenantId must be a lowercase slug")
	}
	if strings.TrimSpace(c.RuntimeVersion) == "" {
		errs = append(errs, "runtimeVersion is required")
	}
	roles := 0
	for _, r := range c.Roles {
		if strings.TrimSpace(r) != "" {
			roles++
		}
	}
	if roles == 0 {
		errs = append(errs, "at least one role is required")
	}
	if len(c.Pages) == 0 {
		errs = append(errs, "at least one page is required")
	}
	defaults := 0
	seen := map[string]bool{}
	for i, p := range c.Pages {
		where := fmt.Sprintf("page %d", i+1)
		if !slug.MatchString(p.ID) {
			errs = append(errs, where+": id must be a lowercase slug")
		} else if seen[p.ID] {
			errs = append(errs, where+": duplicate id "+p.ID)
		}
		seen[p.ID] = true
		if strings.TrimSpace(p.Label) == "" {
			errs = append(errs, where+": label is required")
		}
		if m, ok := modules[p.Module]; !ok {
			errs = append(errs, where+": unknown module "+strconv.Quote(p.Module))
		} else if m.Kind != "experience" {
			errs = append(errs, fmt.Sprintf("%s: %s is a %s module, not a page experience", where, p.Module, m.Kind))
		} else if _, ok := m.Versions[p.ModuleVersion]; !ok {
			errs = append(errs, fmt.Sprintf("%s: %s has no version %q", where, p.Module, p.ModuleVersion))
		}
		if p.Default {
			defaults++
		}
	}
	if len(c.Pages) > 0 && defaults != 1 {
		errs = append(errs, "exactly one page must be the default")
	}
	return append(errs, validateFrame(c.Frame, modules)...)
}

type configStore struct {
	mu  sync.Mutex
	dir string
}

func newConfigStore(dataDir string) *configStore {
	return &configStore{dir: filepath.Join(dataDir, "configs")}
}

func (s *configStore) revisionsDir(tenant string) string {
	return filepath.Join(s.dir, tenant, "revisions")
}

func (s *configStore) revisions(tenant string) ([]int, error) {
	entries, err := os.ReadDir(s.revisionsDir(tenant))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []int
	for _, e := range entries {
		n, err := strconv.Atoi(strings.TrimSuffix(e.Name(), ".json"))
		if err == nil {
			out = append(out, n)
		}
	}
	sort.Ints(out)
	return out, nil
}

func (s *configStore) revision(tenant string, n int) (Config, error) {
	var c Config
	data, err := os.ReadFile(filepath.Join(s.revisionsDir(tenant), fmt.Sprintf("%d.json", n)))
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(data, &c)
	return c, err
}

// latest returns the newest revision, or revision 0 if none exists yet.
func (s *configStore) latest(tenant string) (Config, int, error) {
	revs, err := s.revisions(tenant)
	if err != nil || len(revs) == 0 {
		return Config{}, 0, err
	}
	n := revs[len(revs)-1]
	c, err := s.revision(tenant, n)
	return c, n, err
}

func (s *configStore) save(c Config) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	revs, err := s.revisions(c.TenantID)
	if err != nil {
		return 0, err
	}
	next := 1
	if len(revs) > 0 {
		next = revs[len(revs)-1] + 1
	}
	dir := s.revisionsDir(c.TenantID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return 0, err
	}
	return next, os.WriteFile(filepath.Join(dir, fmt.Sprintf("%d.json", next)), data, 0o644)
}

func (s *configStore) tenants() ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && slug.MatchString(e.Name()) {
			out = append(out, e.Name())
		}
	}
	return out, nil
}
