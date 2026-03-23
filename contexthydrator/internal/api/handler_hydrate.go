package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/yourorg/context-hydrator/internal/cache"
)

type hydrateRequest struct {
	Cookie string `json:"cookie"`
}

func (s *Server) handleHydrate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req hydrateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		if req.Cookie == "" {
			http.Error(w, `{"error":"cookie field is required"}`, http.StatusBadRequest)
			return
		}

		claims, err := s.decoder.Decode(req.Cookie)
		if err != nil {
			s.log.WarnContext(r.Context(), "cookie decode failed", "error", err)
			http.Error(w, `{"error":"invalid cookie"}`, http.StatusBadRequest)
			return
		}

		var contextKey string
		var rawClaims map[string]string

		if claims.HydrationToken != "" {
			// JWT mode: resolve hyd_token → {contextKey, claims} from Redis mapping.
			// The mapping is stored at login time by the issuing application.
			appID := claims.AppID
			if appID == "" {
				appID = s.appID()
			}
			mapping, err := s.store.ResolveMapping(r.Context(), appID, claims.HydrationToken)
			if err != nil {
				if err == cache.ErrCacheMiss {
					s.log.WarnContext(r.Context(), "hydration mapping not found", "app_id", appID)
					http.Error(w, `{"error":"invalid token"}`, http.StatusBadRequest)
				} else {
					s.log.ErrorContext(r.Context(), "mapping lookup failed", "error", err)
					http.Error(w, `{"error":"service unavailable"}`, http.StatusServiceUnavailable)
				}
				return
			}
			contextKey = mapping.ContextKey
			rawClaims = mapping.Claims
		} else {
			// base64json mode: use user_id directly as contextKey (local dev).
			contextKey = claims.UserID
			rawClaims = map[string]string{"user_id": claims.UserID}
		}

		if s.appConfig == nil {
			// No app config — accept the request but skip hydration (test mode).
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
			return
		}

		// Fire-and-forget: background context so HTTP cancellation does not
		// kill the hydration goroutine.
		bgCtx := context.Background()
		go s.hydrator.RunHydration(bgCtx, s.appConfig, contextKey, rawClaims)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
	}
}
