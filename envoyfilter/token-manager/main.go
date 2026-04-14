// Token Manager — Envoy ext_proc gRPC service
//
// Envoy opens a bidirectional gRPC stream for every HTTP request
// (envoy.service.ext_proc.v3.ExternalProcessor/Process).
// This service:
//   1. Receives the ProcessingRequest{RequestHeaders} message
//   2. Fetches (or serves from cache) a JWT from the OAuth server
//   3. Returns a ProcessingResponse that sets the Authorization header
//      and x-envoy-injected-token before Envoy forwards to the backend
//
// Token caching:
//   - sync.Mutex-protected in-memory cache
//   - Only calls the OAuth server when the cached token is within
//     REFRESH_BUFFER_SECS of expiry
//
// Ports:
//   9090 — gRPC ext_proc  (Envoy connects here)
//   8083 — HTTP /health   (Docker health check)

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"google.golang.org/grpc"
)

// ── Token cache ───────────────────────────────────────────────────────────────

type tokenCache struct {
	mu            sync.Mutex
	token         string
	expiresAt     time.Time
	fetchCount    int
	cacheHits     int
	refreshBuffer time.Duration

	oauthURL     string
	clientID     string
	clientSecret string
}

type oauthTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// get returns a valid cached token, fetching a new one from the OAuth server
// only when the cache is empty or within refreshBuffer of expiry.
func (c *tokenCache) get() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.token != "" && time.Now().Before(c.expiresAt.Add(-c.refreshBuffer)) {
		c.cacheHits++
		return c.token, nil
	}

	token, expiresIn, err := c.fetchFromOAuth()
	if err != nil {
		return "", err
	}
	c.token = token
	c.expiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
	c.fetchCount++
	log.Printf("[token-manager] Token refreshed (fetch #%d, expires in %ds, cache hits so far: %d)",
		c.fetchCount, expiresIn, c.cacheHits)
	return c.token, nil
}

func (c *tokenCache) stats() map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	remaining := time.Until(c.expiresAt).Seconds()
	if remaining < 0 {
		remaining = 0
	}
	return map[string]any{
		"token_cached":    c.token != "",
		"fetch_count":     c.fetchCount,
		"cache_hits":      c.cacheHits,
		"expires_in_secs": int(remaining),
	}
}

func (c *tokenCache) fetchFromOAuth() (string, int, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)

	resp, err := http.PostForm(c.oauthURL+"/token", form)
	if err != nil {
		return "", 0, fmt.Errorf("oauth request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("reading oauth response: %w", err)
	}

	var t oauthTokenResponse
	if err := json.Unmarshal(body, &t); err != nil || t.AccessToken == "" {
		return "", 0, fmt.Errorf("parsing oauth response: %s", string(body))
	}
	if t.ExpiresIn == 0 {
		t.ExpiresIn = 3600
	}
	return t.AccessToken, t.ExpiresIn, nil
}

// ── ext_proc gRPC server ──────────────────────────────────────────────────────

type extProcServer struct {
	extprocv3.UnimplementedExternalProcessorServer
	cache *tokenCache
}

// Process implements the bidirectional streaming RPC.
// Envoy opens one stream per HTTP request, sends phase messages, and we respond
// with mutations.  We only handle RequestHeaders; all other phases are skipped
// via Envoy's processing_mode config.
func (s *extProcServer) Process(stream extprocv3.ExternalProcessor_ProcessServer) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil // Envoy closed the stream normally
		}
		if err != nil {
			return err
		}

		switch req.Request.(type) {
		case *extprocv3.ProcessingRequest_RequestHeaders:
			token, err := s.cache.get()
			if err != nil {
				log.Printf("[token-manager] ERROR fetching token: %v", err)
				// Tell Envoy to return 503 to the client immediately
				return stream.Send(&extprocv3.ProcessingResponse{
					Response: &extprocv3.ProcessingResponse_ImmediateResponse{
						ImmediateResponse: &extprocv3.ImmediateResponse{
							Status: &typev3.HttpStatus{
								Code: typev3.StatusCode_ServiceUnavailable,
							},
						},
					},
				})
			}

			// Mutate the request: set Authorization and a sentinel header.
			// Use RawValue (bytes) — Envoy v1.29 deprecated the string Value
			// field in HeaderValue in favour of RawValue.
			if err := stream.Send(&extprocv3.ProcessingResponse{
				Response: &extprocv3.ProcessingResponse_RequestHeaders{
					RequestHeaders: &extprocv3.HeadersResponse{
						Response: &extprocv3.CommonResponse{
							HeaderMutation: &extprocv3.HeaderMutation{
								SetHeaders: []*corev3.HeaderValueOption{
									{Header: &corev3.HeaderValue{
										Key:      "authorization",
										RawValue: []byte("Bearer " + token),
									}},
									{Header: &corev3.HeaderValue{
										Key:      "x-token-injected-by-proxy",
										RawValue: []byte("true"),
									}},
								},
							},
						},
					},
				},
			}); err != nil {
				return err
			}

		default:
			// Unexpected phase — send empty response to unblock Envoy
			if err := stream.Send(&extprocv3.ProcessingResponse{}); err != nil {
				return err
			}
		}
	}
}

// ── Health check HTTP server ──────────────────────────────────────────────────

func runHealthServer(cache *tokenCache, port string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		stats := cache.stats()
		stats["status"] = "ok"
		stats["service"] = "token-manager"
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(stats); err != nil {
			log.Printf("[token-manager] health encode error: %v", err)
		}
	})
	log.Printf("[token-manager] Health endpoint  on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Health server error: %v", err)
	}
}

// ── Main ──────────────────────────────────────────────────────────────────────

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func main() {
	oauthURL      := envStr("OAUTH_SERVER_URL", "http://oauth-server:8082")
	clientID      := envStr("CLIENT_ID", "envoy-proxy")
	clientSecret  := envStr("CLIENT_SECRET", "envoy-secret")
	refreshBuf    := envInt("REFRESH_BUFFER_SECS", 60)
	grpcPort      := envStr("GRPC_PORT", "9090")
	healthPort    := envStr("HEALTH_PORT", "8083")

	cache := &tokenCache{
		oauthURL:      oauthURL,
		clientID:      clientID,
		clientSecret:  clientSecret,
		refreshBuffer: time.Duration(refreshBuf) * time.Second,
	}

	log.Printf("[token-manager] OAuth server   : %s", oauthURL)
	log.Printf("[token-manager] gRPC port      : %s", grpcPort)
	log.Printf("[token-manager] Health port    : %s", healthPort)
	log.Printf("[token-manager] Refresh buffer : %ds before expiry", refreshBuf)

	go runHealthServer(cache, healthPort)

	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("Failed to listen on :%s: %v", grpcPort, err)
	}
	srv := grpc.NewServer()
	extprocv3.RegisterExternalProcessorServer(srv, &extProcServer{cache: cache})
	log.Printf("[token-manager] gRPC ext_proc  on :%s", grpcPort)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("gRPC server error: %v", err)
	}
}
