// mockbackend is a lightweight HTTP server that simulates the four upstream
// services (profile, preferences, permissions, resources). It is intended for
// local development and testing only.
//
// All four services run on a single port (default 9000).
// The context-hydrator expects each service on its own URL, so set:
//
//	PROFILE_SERVICE_URL=http://localhost:9000
//	PREFERENCES_SERVICE_URL=http://localhost:9000
//	PERMISSIONS_SERVICE_URL=http://localhost:9000
//	RESOURCES_SERVICE_URL=http://localhost:9000
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	port := flag.String("port", "9000", "port to listen on")
	latency := flag.Duration("latency", 50*time.Millisecond, "simulated upstream latency")
	flag.Parse()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Latency simulation middleware
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(*latency)
			next.ServeHTTP(w, r)
		})
	})

	r.Get("/users/{userId}/profile", handleProfile)
	r.Get("/users/{userId}/preferences", handlePreferences)
	r.Get("/users/{userId}/permissions", handlePermissions)
	r.Get("/users/{userId}/resources", handleResources)

	addr := ":" + *port
	log.Printf("mock backend listening on %s (latency=%s)", addr, *latency)
	log.Printf("  GET /users/{userId}/profile")
	log.Printf("  GET /users/{userId}/preferences")
	log.Printf("  GET /users/{userId}/permissions")
	log.Printf("  GET /users/{userId}/resources")

	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, "encode error", http.StatusInternalServerError)
	}
}

func handleProfile(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	writeJSON(w, map[string]any{
		"user_id":    userID,
		"email":      userID + "@example.com",
		"first_name": "Test",
		"last_name":  "User",
		"avatar_url": fmt.Sprintf("https://avatars.example.com/%s.png", userID),
		"created_at": "2024-01-15T08:00:00Z",
		"plan":       "pro",
	})
}

func handlePreferences(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	writeJSON(w, map[string]any{
		"user_id":  userID,
		"theme":    "dark",
		"language": "en-US",
		"timezone": "America/Los_Angeles",
		"notifications": map[string]bool{
			"email":   true,
			"push":    false,
			"in_app":  true,
			"weekly":  true,
		},
		"dashboard_layout": "compact",
	})
}

func handlePermissions(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	writeJSON(w, map[string]any{
		"user_id": userID,
		"roles":   []string{"viewer", "editor"},
		"scopes": []string{
			"read:projects",
			"write:projects",
			"read:reports",
			"read:billing",
		},
		"feature_flags": map[string]bool{
			"beta_features":  true,
			"advanced_export": false,
			"api_access":     true,
		},
	})
}

func handleResources(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	writeJSON(w, map[string]any{
		"user_id": userID,
		"projects": []map[string]any{
			{"id": "proj_001", "name": "Alpha", "role": "owner"},
			{"id": "proj_002", "name": "Beta", "role": "member"},
		},
		"workspaces": []map[string]any{
			{"id": "ws_001", "name": "Default Workspace"},
		},
		"storage_quota_mb": 5120,
		"storage_used_mb":  312,
	})
}
