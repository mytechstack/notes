package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/yourorg/context-hydrator/internal/cache"
)

// handleData serves GET /data/{contextKey}/{resource}.
//
// Returns the cached resource for the given context key.
// Returns 404 if the resource has not been hydrated yet — the caller
// should trigger POST /hydrate and retry.
func (s *Server) handleData() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		contextKey := chi.URLParam(r, "contextKey")
		resource := chi.URLParam(r, "resource")

		cacheKey, ok := cache.KeyForResource(s.appID(), contextKey, resource)
		if !ok {
			http.Error(w, `{"error":"unknown resource","allowed":["profile","preferences","permissions","resources"]}`,
				http.StatusBadRequest)
			return
		}

		data, err := s.store.Get(r.Context(), cacheKey)
		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache", "HIT")
			w.Write(data)
			return
		}

		if errors.Is(err, cache.ErrCacheMiss) {
			http.Error(w, `{"error":"not found","hint":"trigger POST /hydrate first"}`, http.StatusNotFound)
			return
		}

		s.log.ErrorContext(r.Context(), "cache read failed",
			"context_key", contextKey, "resource", resource, "error", err)
		http.Error(w, `{"error":"service unavailable"}`, http.StatusServiceUnavailable)
	}
}
