package api

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		redisStatus := "ok"
		httpStatus := http.StatusOK

		if err := s.store.Ping(r.Context()); err != nil {
			redisStatus = "error: " + err.Error()
			httpStatus = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpStatus)
		json.NewEncoder(w).Encode(map[string]any{
			"status": func() string {
				if httpStatus == http.StatusOK {
					return "ok"
				}
				return "degraded"
			}(),
			"checks": map[string]string{
				"redis": redisStatus,
			},
		})
	}
}
