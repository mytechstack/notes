package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Environments and the promotion path between them. The ENVIRONMENTS list
// order IS the promotion order: a version may only move to the next
// environment, and only once it has been stable in the current one.

var environments []string

func initEnvironments() {
	for _, e := range strings.Split(envOr("ENVIRONMENTS", "dev,sit,perf,uat,prod"), ",") {
		if e = strings.TrimSpace(e); slugRe.MatchString(e) {
			environments = append(environments, e)
		}
	}
}

func envIndex(env string) int {
	for i, e := range environments {
		if e == env {
			return i
		}
	}
	return -1
}

func validEnv(env string) bool { return envIndex(env) >= 0 }

// nextEnv returns the environment after env, or "" for the last one.
func nextEnv(env string) string {
	if i := envIndex(env); i >= 0 && i+1 < len(environments) {
		return environments[i+1]
	}
	return ""
}

// Release record: per version, where it has been stable and what happened.

type HistoryEntry struct {
	At     string `json:"at"`
	Env    string `json:"env"`
	Action string `json:"action"` // built | stable | candidate | promoted-from-<env>
}

type Release struct {
	Version        int            `json:"version"`
	SourceRevision int            `json:"sourceRevision"`
	StableIn       []string       `json:"stableIn"`
	History        []HistoryEntry `json:"history"`
}

func releaseKey(tenant string, n int) string {
	return fmt.Sprintf("releases/%s/v%d.json", tenant, n)
}

func readRelease(tenant string, n int) (Release, error) {
	r := Release{Version: n, StableIn: []string{}, History: []HistoryEntry{}}
	data, err := getObject(releaseKey(tenant, n))
	if err != nil || data == nil {
		return r, err
	}
	err = json.Unmarshal(data, &r)
	return r, err
}

func writeRelease(tenant string, r Release) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return putJSON(releaseKey(tenant, r.Version), data)
}

// recordRelease appends a history entry and, for "stable", adds env to
// StableIn. Callers hold writeMu.
func recordRelease(tenant string, version int, env, action string) error {
	r, err := readRelease(tenant, version)
	if err != nil {
		return err
	}
	if action == "stable" && !contains(r.StableIn, env) {
		r.StableIn = append(r.StableIn, env)
	}
	r.History = append(r.History, HistoryEntry{At: time.Now().UTC().Format(time.RFC3339), Env: env, Action: action})
	return writeRelease(tenant, r)
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
