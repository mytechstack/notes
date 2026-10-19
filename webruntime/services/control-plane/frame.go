package main

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Experience frame: the header, navigation and footer around every page.
// Each slot is a versioned frame module from the catalog plus the settings
// a tenant filled in; the theme colors apply to the whole frame. The frame
// is part of the config, so it is built into the manifest and promoted
// dev → prod like everything else.

type Theme struct {
	Primary  string `json:"primary"`
	HeaderBg string `json:"headerBg"`
}

type FrameSlot struct {
	Module        string                 `json:"module"` // "" = slot empty (footer only)
	ModuleVersion string                 `json:"moduleVersion"`
	Settings      map[string]interface{} `json:"settings"`
}

type Frame struct {
	Theme  Theme     `json:"theme"`
	Header FrameSlot `json:"header"`
	Nav    FrameSlot `json:"nav"`
	Footer FrameSlot `json:"footer"`
}

// SettingDef describes one setting a frame module accepts. Types: text,
// url, bool, select (one of Options), links (list of {label, url}).
type SettingDef struct {
	Key     string      `json:"key"`
	Label   string      `json:"label"`
	Type    string      `json:"type"`
	Options []string    `json:"options,omitempty"`
	Default interface{} `json:"default"`
}

var frameSlots = []string{"header", "nav", "footer"}

func (f *Frame) slot(name string) *FrameSlot {
	switch name {
	case "header":
		return &f.Header
	case "nav":
		return &f.Nav
	default:
		return &f.Footer
	}
}

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

const (
	defaultPrimary  = "#1a1a2e"
	defaultHeaderBg = "#1a1a2e"
	maxText         = 200
	maxLinks        = 10
)

// defaultFrame is the starting frame for a new or seeded tenant: the
// catalog's standard header, nav and footer at their latest versions.
func defaultFrame(displayName string, modules map[string]Module) Frame {
	f := Frame{
		Theme:  Theme{Primary: defaultPrimary, HeaderBg: defaultHeaderBg},
		Header: FrameSlot{Module: "frame-header", Settings: map[string]interface{}{"title": displayName}},
		Nav:    FrameSlot{Module: "frame-nav"},
		Footer: FrameSlot{Module: "frame-footer", Settings: map[string]interface{}{"text": "© 2026 " + displayName}},
	}
	normalizeFrame(&f, modules)
	return f
}

// normalizeFrame pins empty versions to latest and fills missing settings
// with their defaults. It never invents a module for an empty slot.
func normalizeFrame(f *Frame, modules map[string]Module) {
	if f.Theme.Primary == "" {
		f.Theme.Primary = defaultPrimary
	}
	if f.Theme.HeaderBg == "" {
		f.Theme.HeaderBg = defaultHeaderBg
	}
	for _, name := range frameSlots {
		s := f.slot(name)
		m, ok := modules[s.Module]
		if s.Module == "" || !ok {
			continue
		}
		if s.ModuleVersion == "" {
			s.ModuleVersion = m.Latest
		}
		if s.Settings == nil {
			s.Settings = map[string]interface{}{}
		}
		for _, d := range m.Settings {
			if _, set := s.Settings[d.Key]; !set {
				s.Settings[d.Key] = d.Default
			}
		}
	}
}

func validateFrame(f Frame, modules map[string]Module) []string {
	var errs []string
	if !hexColor.MatchString(f.Theme.Primary) {
		errs = append(errs, "frame theme: primary color must look like #1a2b3c")
	}
	if !hexColor.MatchString(f.Theme.HeaderBg) {
		errs = append(errs, "frame theme: header color must look like #1a2b3c")
	}
	for _, name := range frameSlots {
		s := f.slot(name)
		where := "frame " + name
		if s.Module == "" {
			if name != "footer" {
				errs = append(errs, where+": a module is required")
			}
			continue
		}
		m, ok := modules[s.Module]
		switch {
		case !ok:
			errs = append(errs, fmt.Sprintf("%s: unknown module %q", where, s.Module))
			continue
		case m.Kind != "frame" || m.Slot != name:
			errs = append(errs, fmt.Sprintf("%s: %s is not a %s frame module", where, s.Module, name))
			continue
		}
		if _, ok := m.Versions[s.ModuleVersion]; !ok {
			errs = append(errs, fmt.Sprintf("%s: %s has no version %q", where, s.Module, s.ModuleVersion))
		}
		defs := map[string]SettingDef{}
		for _, d := range m.Settings {
			defs[d.Key] = d
		}
		for key, v := range s.Settings {
			d, ok := defs[key]
			if !ok {
				errs = append(errs, fmt.Sprintf("%s: unknown setting %q", where, key))
				continue
			}
			if msg := checkSetting(d, v); msg != "" {
				errs = append(errs, fmt.Sprintf("%s: %s %s", where, d.Label, msg))
			}
		}
	}
	return errs
}

func checkSetting(d SettingDef, v interface{}) string {
	switch d.Type {
	case "text":
		s, ok := v.(string)
		if !ok {
			return "must be text"
		}
		if len(s) > maxText {
			return fmt.Sprintf("must be at most %d characters", maxText)
		}
	case "url":
		s, ok := v.(string)
		if !ok {
			return "must be a URL"
		}
		if s != "" && !safeURL(s) {
			return "must start with https:// or http://"
		}
	case "bool":
		if _, ok := v.(bool); !ok {
			return "must be true or false"
		}
	case "select":
		s, _ := v.(string)
		for _, o := range d.Options {
			if s == o {
				return ""
			}
		}
		return fmt.Sprintf("must be one of %s", strings.Join(d.Options, ", "))
	case "links":
		list, ok := v.([]interface{})
		if !ok {
			return "must be a list of links"
		}
		if len(list) > maxLinks {
			return fmt.Sprintf("can have at most %d links", maxLinks)
		}
		for i, item := range list {
			l, _ := item.(map[string]interface{})
			label, _ := l["label"].(string)
			u, _ := l["url"].(string)
			if strings.TrimSpace(label) == "" || len(label) > 80 {
				return fmt.Sprintf("link %d needs a label (up to 80 characters)", i+1)
			}
			if !safeURL(u) {
				return fmt.Sprintf("link %d must start with https:// or http://", i+1)
			}
		}
	default:
		return "has an unsupported type " + d.Type
	}
	return ""
}

// safeURL only allows absolute http(s) URLs — never javascript:, data: etc.
func safeURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != ""
}

// frameModules returns the frame modules a frame uses, resolved for the
// snapshot WCR pulls (same "module@version" keys as pages).
func frameModules(f Frame, modules map[string]Module) map[string]resolvedModule {
	out := map[string]resolvedModule{}
	for _, name := range frameSlots {
		s := f.slot(name)
		m, ok := modules[s.Module]
		if !ok {
			continue
		}
		if v, ok := m.Versions[s.ModuleVersion]; ok {
			out[s.Module+"@"+s.ModuleVersion] = resolvedModule{
				Path: v.Path, Scope: m.Scope, Version: s.ModuleVersion, DeployedAt: v.DeployedAt,
			}
		}
	}
	return out
}
