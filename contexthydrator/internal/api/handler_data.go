package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/yourorg/context-hydrator/internal/cache"
	"github.com/yourorg/context-hydrator/internal/services"
)

func (s *Server) handleData() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := chi.URLParam(r, "userId")
		resource := chi.URLParam(r, "resource")

		cacheKey, ok := cache.KeyForResource(userID, resource)
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

		if !errors.Is(err, cache.ErrCacheMiss) {
			s.log.ErrorContext(r.Context(), "cache read failed",
				"user_id", userID, "resource", resource, "error", err)
			http.Error(w, `{"error":"service unavailable"}`, http.StatusServiceUnavailable)
			return
		}

		// Cache miss — fetch live from backend
		s.log.InfoContext(r.Context(), "cache miss, fetching live",
			"user_id", userID, "resource", resource)

		liveCtx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
		defer cancel()

		results := s.backend.FetchAll(liveCtx, userID, []services.ServiceName{services.ServiceName(resource)})
		result := results[0]

		if result.Err != nil {
			s.log.WarnContext(r.Context(), "live fetch failed",
				"user_id", userID, "resource", resource, "error", result.Err)
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}

		// Write back to cache in the background
		go func(data []byte) {
			ttl, key := ttlAndKeyFor(services.ServiceName(resource), userID)
			if err := s.store.Set(context.Background(), key, data, ttl); err != nil {
				s.log.Warn("cache write-back failed",
					"user_id", userID, "resource", resource, "error", err)
			}
		}(result.Data)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "MISS")
		w.Write(result.Data)
	}
}
