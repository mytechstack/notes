package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"sync"
	"time"
)

// WCR's event listener. The control plane POSTs a thin config.changed
// event; WCR acks with 202, then pulls that config revision, generates ONE
// environment-agnostic manifest, and stores it as a new immutable version.
// The first environment (dev) gets it automatically; every later
// environment only gets it through an explicit promotion.

type configEvent struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	TenantID   string `json:"tenantId"`
	Revision   int    `json:"revision"`
	OccurredAt string `json:"occurredAt"`
	Bootstrap  bool   `json:"bootstrap"` // pre-existing tenant: first build goes live everywhere
}

type eventLogEntry struct {
	Event      configEvent `json:"event"`
	ReceivedAt string      `json:"receivedAt"`
	Status     string      `json:"status"` // processing | generated | duplicate | error
	Version    int         `json:"version,omitempty"`
	Error      string      `json:"error,omitempty"`
}

const maxEventLog = 200

var (
	eventLogMu sync.Mutex
	eventLog   []*eventLogEntry

	// writeMu serializes everything that creates versions or moves
	// pointers, so concurrent events/promotes/rollbacks can't race on
	// nextVersion or pointers.json.
	writeMu sync.Mutex

	slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

	cpClient = &http.Client{Timeout: 5 * time.Second}
)

func recordEvent(e configEvent) *eventLogEntry {
	entry := &eventLogEntry{Event: e, ReceivedAt: time.Now().UTC().Format(time.RFC3339), Status: "processing"}
	eventLogMu.Lock()
	eventLog = append(eventLog, entry)
	if len(eventLog) > maxEventLog {
		eventLog = eventLog[len(eventLog)-maxEventLog:]
	}
	eventLogMu.Unlock()
	return entry
}

func finishEvent(entry *eventLogEntry, status string, version int, err error) {
	eventLogMu.Lock()
	defer eventLogMu.Unlock()
	entry.Status = status
	entry.Version = version
	if err != nil {
		entry.Error = err.Error()
	}
}

func eventsHandler(w http.ResponseWriter, r *http.Request) {
	var e configEvent
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&e); err != nil {
		writeErr(w, 400, "invalid json", nil)
		return
	}
	if e.Type != "config.changed" || e.ID == "" || e.Revision < 1 ||
		!slugRe.MatchString(e.TenantID) {
		writeErr(w, 400, "expected config.changed with id, tenantId, revision", nil)
		return
	}
	entry := recordEvent(e)
	go processEvent(entry)
	writeJSON(w, http.StatusAccepted, map[string]interface{}{"accepted": e.ID})
}

func listEventsHandler(w http.ResponseWriter, r *http.Request) {
	eventLogMu.Lock()
	out := make([]eventLogEntry, 0, len(eventLog))
	for i := len(eventLog) - 1; i >= 0; i-- {
		out = append(out, *eventLog[i])
	}
	eventLogMu.Unlock()
	writeJSON(w, 200, map[string]interface{}{"events": out})
}

func processEvent(entry *eventLogEntry) {
	e := entry.Event
	writeMu.Lock()
	defer writeMu.Unlock()

	if v, ok := versionForRevision(e.TenantID, e.Revision); ok {
		finishEvent(entry, "duplicate", v, nil)
		return
	}
	snap, err := fetchSnapshot(e)
	if err != nil {
		finishEvent(entry, "error", 0, err)
		log.Printf("event %s: %v", e.ID, err)
		return
	}
	m, err := generateManifest(snap, e.ID)
	if err != nil {
		finishEvent(entry, "error", 0, err)
		return
	}
	data, _ := json.Marshal(m)
	if _, missing, err := validateManifest(data); err != nil || len(missing) > 0 {
		finishEvent(entry, "error", 0, fmt.Errorf("generated manifest invalid: %v %v", err, missing))
		return
	}
	version, err := nextVersion(e.TenantID)
	if err == nil {
		err = writeVersion(e.TenantID, version, data)
	}
	if err == nil {
		err = writeRelease(e.TenantID, Release{Version: version, SourceRevision: e.Revision,
			StableIn: []string{}, History: []HistoryEntry{{At: time.Now().UTC().Format(time.RFC3339), Action: "built"}}})
	}
	if err != nil {
		finishEvent(entry, "error", 0, err)
		return
	}
	// dev (the first environment) always runs the newest build. Later
	// environments only get a version by promotion — except for a
	// pre-existing tenant being bootstrapped (event.Bootstrap), whose
	// empty environments are filled so it is live everywhere from the start.
	for i, env := range environments {
		if i > 0 && !e.Bootstrap {
			break
		}
		p, err := readPointers(e.TenantID, env)
		if err != nil {
			finishEvent(entry, "error", version, err)
			return
		}
		if i > 0 && p.Stable != 0 {
			continue
		}
		p.Stable, p.Candidate, p.Canary = version, version, Canary{}
		if err := writePointers(e.TenantID, env, p); err != nil {
			finishEvent(entry, "error", version, err)
			return
		}
		_ = recordRelease(e.TenantID, version, env, "stable")
		emitWebhooks(e.TenantID, env, "stable", version)
	}
	finishEvent(entry, "generated", version, nil)
	log.Printf("event %s: generated %s v%d from revision %d (live in %s)", e.ID, e.TenantID, version, e.Revision, environments[0])
}

func fetchSnapshot(e configEvent) (cpSnapshot, error) {
	var s cpSnapshot
	base := os.Getenv("CONTROL_PLANE_URL")
	if base == "" {
		base = "http://control-plane:8093"
	}
	url := fmt.Sprintf("%s/api/configs/%s/revisions/%d", base, e.TenantID, e.Revision)
	resp, err := cpClient.Get(url)
	if err != nil {
		return s, fmt.Errorf("fetch snapshot: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return s, fmt.Errorf("fetch snapshot: control plane returned %d", resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&s); err != nil {
		return s, fmt.Errorf("decode snapshot: %w", err)
	}
	if s.TenantID != e.TenantID || s.Revision != e.Revision {
		return s, fmt.Errorf("snapshot does not match event")
	}
	return s, nil
}

// versionForRevision finds an existing version generated from the given
// control-plane revision — makes event handling idempotent.
func versionForRevision(tenant string, revision int) (int, bool) {
	versions, _ := listVersions(tenant)
	for _, v := range versions {
		data, err := readVersion(tenant, v)
		if err != nil {
			continue
		}
		var m map[string]interface{}
		if json.Unmarshal(data, &m) == nil && sourceRevision(m) == revision {
			return v, true
		}
	}
	return 0, false
}
