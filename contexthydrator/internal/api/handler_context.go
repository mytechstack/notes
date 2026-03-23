package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/yourorg/context-hydrator/internal/cache"
	"github.com/yourorg/context-hydrator/internal/services"
)

type resourceMeta struct {
	Source string `json:"source"`          // "cache" | "unavailable"
	Error  string `json:"error,omitempty"` // set when source == "unavailable"
}

type contextResponse struct {
	ContextKey string                     `json:"context_key"`
	Data       map[string]json.RawMessage `json:"data"`
	Meta       map[string]resourceMeta    `json:"meta"`
}

// handleContext serves GET /context/{contextKey}?resources=profile,preferences,...
//
// For each requested resource:
//  1. Try Redis cache → source: "cache"
//  2. On miss or error: include in meta with source: "unavailable"
//
// Always returns 200 with whatever data is available.
// Callers should inspect meta.source to know the freshness of each field.
// On full cache miss, trigger POST /hydrate and retry.
func (s *Server) handleContext() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		contextKey := chi.URLParam(r, "contextKey")

		requested := parseResourcesParam(r)
		if len(requested) == 0 {
			http.Error(w, `{"error":"no valid resources requested","allowed":["profile","preferences","permissions","resources"]}`,
				http.StatusBadRequest)
			return
		}

		resp := contextResponse{
			ContextKey: contextKey,
			Data:       make(map[string]json.RawMessage, len(requested)),
			Meta:       make(map[string]resourceMeta, len(requested)),
		}

		for _, svc := range requested {
			key, _ := cache.KeyForResource(s.appID(), contextKey, string(svc))
			data, err := s.store.Get(r.Context(), key)
			if err == nil {
				resp.Data[string(svc)] = data
				resp.Meta[string(svc)] = resourceMeta{Source: "cache"}
				continue
			}
			if errors.Is(err, cache.ErrCacheMiss) {
				resp.Meta[string(svc)] = resourceMeta{Source: "unavailable", Error: "cache miss — trigger POST /hydrate"}
				continue
			}
			s.log.WarnContext(r.Context(), "cache read error",
				"context_key", contextKey, "resource", svc, "error", err)
			resp.Meta[string(svc)] = resourceMeta{Source: "unavailable", Error: "cache error"}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}
}

// parseResourcesParam reads ?resources=profile,preferences or ?resources=profile&resources=permissions.
// Defaults to all four resources when the param is absent.
func parseResourcesParam(r *http.Request) []services.ServiceName {
	raw := r.URL.Query()["resources"]

	var tokens []string
	for _, v := range raw {
		for _, part := range strings.Split(v, ",") {
			if t := strings.TrimSpace(part); t != "" {
				tokens = append(tokens, t)
			}
		}
	}

	if len(tokens) == 0 {
		return services.AllServices
	}

	seen := make(map[services.ServiceName]bool)
	var result []services.ServiceName
	for _, t := range tokens {
		svc := services.ServiceName(t)
		switch svc {
		case services.ServiceProfile, services.ServicePreferences,
			services.ServicePermissions, services.ServiceResources:
			if !seen[svc] {
				seen[svc] = true
				result = append(result, svc)
			}
		}
	}
	return result
}
