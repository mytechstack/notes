package main

import (
	"encoding/json"
	"os"
)

// Environment bindings: the only environment-specific input. The manifest
// itself is identical in dev, sit, perf, uat and prod; each environment
// supplies its own hosts here. In production every environment's Web
// Runtime Service would carry just its own entry; this POC runs one
// service for all of them and picks the entry from ?env=.

type envBinding struct {
	Label     string `json:"label"`
	CdaasBase string `json:"cdaasBase"` // browser-facing host for experience modules
	DataBase  string `json:"dataBase"`  // server-side host for experience data
}

func loadBindings(path string) (map[string]envBinding, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var b map[string]envBinding
	return b, json.Unmarshal(data, &b)
}

// bind returns a copy of an environment-agnostic manifest with each
// experience's path/dataPath turned into this environment's remote/dataUrl
// — the shape runtime.js, buildNavItems and ssrdata.go already expect.
func bind(raw map[string]interface{}, b envBinding) map[string]interface{} {
	var m map[string]interface{}
	data, _ := json.Marshal(raw)
	_ = json.Unmarshal(data, &m)
	shell, _ := m["shell"].(map[string]interface{})
	nav, _ := shell["nav"].([]interface{})
	for _, item := range nav {
		it, _ := item.(map[string]interface{})
		exp, _ := it["experience"].(map[string]interface{})
		if exp == nil {
			continue
		}
		if p, ok := exp["path"].(string); ok {
			exp["remote"] = b.CdaasBase + p
		}
		if p, ok := exp["dataPath"].(string); ok {
			exp["dataUrl"] = b.DataBase + p
		}
	}
	// Experience frame modules (header, nav, footer) load from CDAaS too.
	frame, _ := shell["frame"].(map[string]interface{})
	for _, slot := range []string{"header", "nav", "footer"} {
		if f, ok := frame[slot].(map[string]interface{}); ok {
			if p, ok := f["path"].(string); ok {
				f["remote"] = b.CdaasBase + p
			}
		}
	}
	return m
}
