package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// Transactional-outbox stand-in: every config save appends an event here
// first (persisted), and a single worker delivers pending events to WRCS,
// retrying until WRCS accepts. A save never fails because WRCS is down —
// the event just stays pending until it comes back.

type Event struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	TenantID   string `json:"tenantId"`
	Revision   int    `json:"revision"`
	OccurredAt string `json:"occurredAt"`
	// Bootstrap marks a tenant that already exists outside this system
	// (seeded): its first build goes live in every environment. New
	// tenants omit it and start in dev only.
	Bootstrap bool `json:"bootstrap,omitempty"`
}

type OutboxEntry struct {
	Event          Event  `json:"event"`
	Status         string `json:"status"` // pending | delivered
	Attempts       int    `json:"attempts"`
	LastError      string `json:"lastError,omitempty"`
	ResponseStatus int    `json:"responseStatus,omitempty"`
	DeliveredAt    string `json:"deliveredAt,omitempty"`
}

const maxDeliveredKept = 200

type outbox struct {
	mu      sync.Mutex
	path    string
	entries []*OutboxEntry
	target  string
	client  *http.Client
	kick    chan struct{}
}

func newOutbox(path, target string) *outbox {
	o := &outbox{
		path:   path,
		target: target,
		client: &http.Client{Timeout: 3 * time.Second},
		kick:   make(chan struct{}, 1),
	}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &o.entries)
	}
	return o
}

func newEventID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (o *outbox) enqueue(tenant string, revision int, bootstrap bool) Event {
	e := Event{
		ID:         newEventID(),
		Type:       "config.changed",
		TenantID:   tenant,
		Revision:   revision,
		OccurredAt: time.Now().UTC().Format(time.RFC3339),
		Bootstrap:  bootstrap,
	}
	o.mu.Lock()
	o.entries = append(o.entries, &OutboxEntry{Event: e, Status: "pending"})
	o.persistLocked()
	o.mu.Unlock()
	select {
	case o.kick <- struct{}{}:
	default:
	}
	return e
}

// snapshot returns entries newest-first for the UI.
func (o *outbox) snapshot() []OutboxEntry {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]OutboxEntry, 0, len(o.entries))
	for i := len(o.entries) - 1; i >= 0; i-- {
		out = append(out, *o.entries[i])
	}
	return out
}

func (o *outbox) persistLocked() {
	// Trim old delivered entries; pending ones are never dropped.
	delivered := 0
	for i := len(o.entries) - 1; i >= 0; i-- {
		if o.entries[i].Status == "delivered" {
			delivered++
			if delivered > maxDeliveredKept {
				o.entries = append(o.entries[:i], o.entries[i+1:]...)
			}
		}
	}
	data, err := json.MarshalIndent(o.entries, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(o.path, data, 0o644); err != nil {
		log.Printf("outbox persist failed: %v", err)
	}
}

func (o *outbox) run() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		o.deliverPending()
		select {
		case <-o.kick:
		case <-ticker.C:
		}
	}
}

func (o *outbox) deliverPending() {
	o.mu.Lock()
	var pending []*OutboxEntry
	for _, e := range o.entries {
		if e.Status == "pending" {
			pending = append(pending, e)
		}
	}
	o.mu.Unlock()

	for _, entry := range pending {
		status, err := o.post(entry.Event)
		o.mu.Lock()
		entry.Attempts++
		entry.ResponseStatus = status
		if err != nil {
			entry.LastError = err.Error()
		} else {
			entry.Status = "delivered"
			entry.LastError = ""
			entry.DeliveredAt = time.Now().UTC().Format(time.RFC3339)
			log.Printf("event %s delivered (%s rev %d)", entry.Event.ID, entry.Event.TenantID, entry.Event.Revision)
		}
		o.persistLocked()
		o.mu.Unlock()
	}
}

func (o *outbox) post(e Event) (int, error) {
	body, _ := json.Marshal(e)
	resp, err := o.client.Post(o.target, "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		return resp.StatusCode, fmt.Errorf("WRCS returned %d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}
