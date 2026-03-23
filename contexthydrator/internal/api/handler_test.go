package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourorg/context-hydrator/internal/cache"
	"github.com/yourorg/context-hydrator/internal/cookie"
	"github.com/yourorg/context-hydrator/internal/hydrator"
	"github.com/yourorg/context-hydrator/internal/observability"
)

// mockStore satisfies the store interface used by handlers.
type mockStore struct {
	data map[string]json.RawMessage
	err  error
}

func (m *mockStore) Get(_ context.Context, key string) (json.RawMessage, error) {
	if m.err != nil {
		return nil, m.err
	}
	v, ok := m.data[key]
	if !ok {
		return nil, cache.ErrCacheMiss
	}
	return v, nil
}

func (m *mockStore) Set(_ context.Context, key string, data json.RawMessage, _ interface{}) error {
	m.data[key] = data
	return nil
}

func (m *mockStore) Ping(_ context.Context) error {
	return m.err
}

func (m *mockStore) GetAccessPattern(_ context.Context, _ string) ([]string, error) {
	return nil, cache.ErrCacheMiss
}

func TestHandleHealth_NoRedis(t *testing.T) {
	t.Skip("requires Redis — run with make test-integration")
}

func TestHandleHydrate_InvalidBody(t *testing.T) {
	log := observability.NewLogger("info", "text")
	decoder := cookie.NewDecoder("base64json", "")
	hyd := hydrator.New(nil, nil, log, 0)
	srv := NewServer(nil, hyd, decoder, nil, log)

	body := bytes.NewBufferString(`{not valid json}`)
	req := httptest.NewRequest(http.MethodPost, "/hydrate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleHydrate_MissingCookie(t *testing.T) {
	log := observability.NewLogger("info", "text")
	decoder := cookie.NewDecoder("base64json", "")
	hyd := hydrator.New(nil, nil, log, 0)
	srv := NewServer(nil, hyd, decoder, nil, log)

	body := bytes.NewBufferString(`{"cookie":""}`)
	req := httptest.NewRequest(http.MethodPost, "/hydrate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleHydrate_ValidCookie(t *testing.T) {
	log := observability.NewLogger("info", "text")
	decoder := cookie.NewDecoder("base64json", "")

	// nil appConfig — handler accepts but skips hydration (test mode)
	srv := NewServer(nil, nil, decoder, nil, log)

	claims := map[string]string{"user_id": "u123", "session_token": "tok"}
	b, _ := json.Marshal(claims)
	encoded := base64.StdEncoding.EncodeToString(b)

	body := bytes.NewBufferString(`{"cookie":"` + encoded + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/hydrate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusAccepted)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "accepted" {
		t.Errorf("status field: got %q, want %q", resp["status"], "accepted")
	}
}

func TestHandleData_UnknownResource(t *testing.T) {
	log := observability.NewLogger("info", "text")
	decoder := cookie.NewDecoder("base64json", "")
	hyd := hydrator.New(nil, nil, log, 0)
	// nil appConfig → appID() returns "default"
	srv := NewServer(nil, hyd, decoder, nil, log)

	req := httptest.NewRequest(http.MethodGet, "/data/u123/unknown", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleData_CacheMiss(t *testing.T) {
	t.Skip("requires Redis store — run with make test-integration")
}
