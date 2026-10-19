package main

import (
	"html/template"
	"net/url"
	"regexp"
)

// frameView is what the server needs from the manifest's shell.frame to
// render usable fallbacks (title, nav position, footer) before the frame
// modules load — and to keep the page working if one fails to load. Every
// value is re-checked here even though the control plane validated it.

type footerLink struct {
	Label string
	URL   string
}

type frameView struct {
	Title       string
	NavPosition string       // "left" or "top"
	ThemeStyle  template.CSS // CSS variables, built only from validated #rrggbb colors
	FooterText  string
	FooterLinks []footerLink
	HasFooter   bool
	Remotes     []string // frame module URLs to preload
}

var hexColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func buildFrameView(manifest map[string]interface{}, fallbackTitle string) frameView {
	v := frameView{Title: fallbackTitle, NavPosition: "left"}
	shell, _ := manifest["shell"].(map[string]interface{})
	frame, _ := shell["frame"].(map[string]interface{})

	theme, _ := frame["theme"].(map[string]interface{})
	primary, _ := theme["primary"].(string)
	headerBg, _ := theme["headerBg"].(string)
	style := ""
	if hexColorRe.MatchString(primary) {
		style += "--wr-primary:" + primary + ";"
	}
	if hexColorRe.MatchString(headerBg) {
		style += "--wr-header-bg:" + headerBg + ";"
	}
	v.ThemeStyle = template.CSS(style)

	settings := func(slot string) map[string]interface{} {
		f, _ := frame[slot].(map[string]interface{})
		if f == nil {
			return nil
		}
		if r, ok := f["remote"].(string); ok && r != "" {
			v.Remotes = append(v.Remotes, r)
		}
		s, _ := f["settings"].(map[string]interface{})
		if s == nil {
			s = map[string]interface{}{}
		}
		return s
	}

	if s := settings("header"); s != nil {
		if t, ok := s["title"].(string); ok && t != "" {
			v.Title = t
		}
	}
	if s := settings("nav"); s != nil {
		if p, _ := s["position"].(string); p == "top" {
			v.NavPosition = "top"
		}
	}
	if s := settings("footer"); s != nil {
		v.HasFooter = true
		v.FooterText, _ = s["text"].(string)
		links, _ := s["links"].([]interface{})
		for _, item := range links {
			l, _ := item.(map[string]interface{})
			label, _ := l["label"].(string)
			u, _ := l["url"].(string)
			if label != "" && isHTTPURL(u) {
				v.FooterLinks = append(v.FooterLinks, footerLink{Label: label, URL: u})
			}
		}
	}
	return v
}

func isHTTPURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != ""
}
