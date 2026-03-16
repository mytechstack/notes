package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourorg/context-hydrator/internal/observability"
	"github.com/yourorg/context-hydrator/internal/services"
)

func TestParseResourcesParam_Default(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/context/u1", nil)
	got := parseResourcesParam(req)
	if len(got) != len(services.AllServices) {
		t.Errorf("expected %d resources, got %d", len(services.AllServices), len(got))
	}
}

func TestParseResourcesParam_CommaSeparated(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/context/u1?resources=profile,preferences", nil)
	got := parseResourcesParam(req)
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
	if got[0] != services.ServiceProfile || got[1] != services.ServicePreferences {
		t.Errorf("unexpected values: %v", got)
	}
}

func TestParseResourcesParam_MultiKey(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/context/u1?resources=profile&resources=permissions", nil)
	got := parseResourcesParam(req)
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d: %v", len(got), got)
	}
}

func TestParseResourcesParam_Deduplicated(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/context/u1?resources=profile,profile,profile", nil)
	got := parseResourcesParam(req)
	if len(got) != 1 {
		t.Errorf("expected 1 after dedup, got %d", len(got))
	}
}

func TestParseResourcesParam_AllUnknown(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/context/u1?resources=bogus,invalid", nil)
	got := parseResourcesParam(req)
	if len(got) != 0 {
		t.Errorf("expected 0, got %d", len(got))
	}
}

func TestHandleContext_NoValidResources(t *testing.T) {
	log := observability.NewLogger("info", "text")
	srv := NewServer(nil, nil, nil, nil, log)

	req := httptest.NewRequest(http.MethodGet, "/context/u1?resources=bogus", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}
