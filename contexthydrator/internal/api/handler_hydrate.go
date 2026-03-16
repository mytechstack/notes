package api

import (
	"context"
	"encoding/json"
	"net/http"
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

		// Fire-and-forget: use background context so HTTP response cancellation
		// does not kill the hydration goroutine.
		bgCtx := context.Background()
		go s.hydrator.RunHydration(bgCtx, claims.UserID)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "accepted",
			"user_id": claims.UserID,
		})
	}
}
