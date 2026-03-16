package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/yourorg/context-hydrator/internal/cache"
	redisc "github.com/yourorg/context-hydrator/internal/redis"
	"github.com/yourorg/context-hydrator/internal/services"
)

type resourceMeta struct {
	Source string `json:"source"`           // "cache" | "live" | "unavailable"
	Error  string `json:"error,omitempty"`  // set when source == "unavailable"
}

type contextResponse struct {
	UserID string                      `json:"user_id"`
	Data   map[string]json.RawMessage  `json:"data"`
	Meta   map[string]resourceMeta     `json:"meta"`
}

// handleContext serves GET /context/{userId}?resources=profile,preferences,...
//
// For each requested resource:
//  1. Try Redis cache  → source: "cache"
//  2. On miss: fetch live from backend → source: "live", write-back to cache in bg
//  3. On error: include in meta with source: "unavailable", omit from data
//
// Always returns 200 with whatever data is available. Callers should inspect
// meta.source to know the freshness of each field.
func (s *Server) handleContext() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := chi.URLParam(r, "userId")

		requested := parseResourcesParam(r)
		if len(requested) == 0 {
			http.Error(w, `{"error":"no valid resources requested","allowed":["profile","preferences","permissions","resources"]}`,
				http.StatusBadRequest)
			return
		}

		resp := contextResponse{
			UserID: userID,
			Data:   make(map[string]json.RawMessage, len(requested)),
			Meta:   make(map[string]resourceMeta, len(requested)),
		}

		// Separate cache hits from misses in a first pass (no I/O yet for hits)
		var cacheMisses []services.ServiceName

		for _, svc := range requested {
			key, _ := cache.KeyForResource(userID, string(svc))
			data, err := s.store.Get(r.Context(), key)
			if err == nil {
				resp.Data[string(svc)] = data
				resp.Meta[string(svc)] = resourceMeta{Source: "cache"}
				continue
			}
			if errors.Is(err, cache.ErrCacheMiss) {
				cacheMisses = append(cacheMisses, svc)
				continue
			}
			// Redis error — record and skip
			s.log.WarnContext(r.Context(), "cache read error",
				"user_id", userID, "resource", svc, "error", err)
			resp.Meta[string(svc)] = resourceMeta{Source: "unavailable", Error: "cache error"}
		}

		// Live fetch for cache misses — parallel, with a tight deadline
		if len(cacheMisses) > 0 {
			liveCtx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
			defer cancel()

			results := s.backend.FetchAll(liveCtx, userID, cacheMisses)

			for _, result := range results {
				name := string(result.Service)
				if result.Err != nil {
					s.log.WarnContext(r.Context(), "live fetch failed",
						"user_id", userID, "resource", name, "error", result.Err)
					resp.Meta[name] = resourceMeta{Source: "unavailable", Error: result.Err.Error()}
					continue
				}

				resp.Data[name] = result.Data
				resp.Meta[name] = resourceMeta{Source: "live"}

				// Write back to cache in the background so the next request is warm
				go func(svc services.ServiceName, data json.RawMessage) {
					ttl, key := ttlAndKeyFor(svc, userID)
					if err := s.store.Set(context.Background(), key, data, ttl); err != nil {
						s.log.Warn("cache write-back failed",
							"user_id", userID, "resource", svc, "error", err)
					}
				}(result.Service, result.Data)
			}
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

	// Flatten comma-separated values: ?resources=profile,preferences
	var tokens []string
	for _, v := range raw {
		for _, part := range strings.Split(v, ",") {
			if t := strings.TrimSpace(part); t != "" {
				tokens = append(tokens, t)
			}
		}
	}

	if len(tokens) == 0 {
		return services.AllServices // default: fetch everything
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

func ttlAndKeyFor(svc services.ServiceName, userID string) (time.Duration, string) {
	switch svc {
	case services.ServiceProfile:
		return redisc.TTLProfile, cache.ProfileKey(userID)
	case services.ServicePreferences:
		return redisc.TTLPreferences, cache.PreferencesKey(userID)
	case services.ServicePermissions:
		return redisc.TTLPermissions, cache.PermissionsKey(userID)
	default:
		return redisc.TTLResources, cache.ResourcesKey(userID)
	}
}
