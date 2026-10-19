package main

import (
	"fmt"
	"time"
)

// Shapes of the control plane's revision snapshot
// (GET /api/configs/{tenant}/revisions/{n}). WCR owns the
// config -> manifest mapping; the control plane only owns config.

type cpPage struct {
	ID            string `json:"id"`
	Label         string `json:"label"`
	Icon          string `json:"icon"`
	Default       bool   `json:"default"`
	Module        string `json:"module"`
	ModuleVersion string `json:"moduleVersion"`
}

type cpFrameSlot struct {
	Module        string                 `json:"module"`
	ModuleVersion string                 `json:"moduleVersion"`
	Settings      map[string]interface{} `json:"settings"`
}

type cpFrame struct {
	Theme  map[string]interface{} `json:"theme"`
	Header cpFrameSlot            `json:"header"`
	Nav    cpFrameSlot            `json:"nav"`
	Footer cpFrameSlot            `json:"footer"`
}

type cpConfig struct {
	TenantID       string   `json:"tenantId"`
	DisplayName    string   `json:"displayName"`
	RuntimeVersion string   `json:"runtimeVersion"`
	Roles          []string `json:"roles"`
	Pages          []cpPage `json:"pages"`
	Frame          cpFrame  `json:"frame"`
}

// Module locations are environment-free paths; the Web Runtime Service in
// each environment prefixes them with that environment's hosts.
type cpModule struct {
	Path       string `json:"path"`
	Scope      string `json:"scope"`
	Version    string `json:"version"`
	DeployedAt string `json:"deployedAt"`
	DataPath   string `json:"dataPath"`
}

type cpSnapshot struct {
	TenantID string              `json:"tenantId"`
	Revision int                 `json:"revision"`
	Config   cpConfig            `json:"config"`
	Modules  map[string]cpModule `json:"modules"` // keyed "module@version"
}

func generateManifest(s cpSnapshot, eventID string) (map[string]interface{}, error) {
	c := s.Config
	nav := make([]interface{}, 0, len(c.Pages))
	for _, p := range c.Pages {
		mod, ok := s.Modules[p.Module+"@"+p.ModuleVersion]
		if !ok {
			return nil, fmt.Errorf("page %q references %s@%s, missing from snapshot", p.ID, p.Module, p.ModuleVersion)
		}
		exp := map[string]interface{}{
			"type":          "module-federation",
			"path":          mod.Path,
			"scope":         mod.Scope,
			"module":        "./App",
			"moduleVersion": mod.Version,
			"deployedAt":    mod.DeployedAt,
		}
		if mod.DataPath != "" {
			exp["dataPath"] = mod.DataPath
		}
		nav = append(nav, map[string]interface{}{
			"id":         p.ID,
			"label":      p.Label,
			"icon":       p.Icon,
			"path":       "/" + p.ID,
			"default":    p.Default,
			"permission": nil,
			"experience": exp,
		})
	}
	roles := make([]interface{}, 0, len(c.Roles))
	for _, r := range c.Roles {
		roles = append(roles, r)
	}
	frame, err := generateFrame(c.Frame, s.Modules)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"version":  "1.0",
		"tenantId": s.TenantID,
		"runtime":  map[string]interface{}{"version": c.RuntimeVersion, "upgradePolicy": "auto-patch"},
		"shell": map[string]interface{}{
			"frame": frame,
			"nav":   nav,
			"auth":  map[string]interface{}{"required": true, "roles": roles},
		},
		"source": map[string]interface{}{
			"controlPlaneRevision": s.Revision,
			"eventId":              eventID,
			"generatedAt":          time.Now().UTC().Format(time.RFC3339),
		},
	}, nil
}

// sourceRevision reads back the control-plane revision a stored manifest
// was generated from (0 if absent).
func sourceRevision(m map[string]interface{}) int {
	src, _ := m["source"].(map[string]interface{})
	n, _ := src["controlPlaneRevision"].(float64)
	return int(n)
}

// generateFrame turns the frame config into the manifest's shell.frame:
// per slot the frame module's location (a path, never a host) plus the
// tenant's settings, and the theme. An empty slot is omitted.
func generateFrame(f cpFrame, modules map[string]cpModule) (map[string]interface{}, error) {
	out := map[string]interface{}{"theme": f.Theme}
	for name, slot := range map[string]cpFrameSlot{"header": f.Header, "nav": f.Nav, "footer": f.Footer} {
		if slot.Module == "" {
			continue
		}
		mod, ok := modules[slot.Module+"@"+slot.ModuleVersion]
		if !ok {
			return nil, fmt.Errorf("frame %s references %s@%s, missing from snapshot", name, slot.Module, slot.ModuleVersion)
		}
		out[name] = map[string]interface{}{
			"path":          mod.Path,
			"scope":         mod.Scope,
			"module":        slot.Module,
			"moduleVersion": slot.ModuleVersion,
			"settings":      slot.Settings,
		}
	}
	return out, nil
}
