package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/yourorg/context-hydrator/internal/cache"
	"github.com/yourorg/context-hydrator/internal/cookie"
	"github.com/yourorg/context-hydrator/internal/hydrator"
	"github.com/yourorg/context-hydrator/internal/services"
)

type Server struct {
	store     *cache.Store
	hydrator  *hydrator.Hydrator
	decoder   *cookie.Decoder
	appConfig *services.AppConfig
	log       *slog.Logger
}

func NewServer(
	store *cache.Store,
	hyd *hydrator.Hydrator,
	decoder *cookie.Decoder,
	appConfig *services.AppConfig,
	log *slog.Logger,
) *Server {
	return &Server{
		store:     store,
		hydrator:  hyd,
		decoder:   decoder,
		appConfig: appConfig,
		log:       log,
	}
}

// appID returns the configured app ID, falling back to "default" for tests.
func (s *Server) appID() string {
	if s.appConfig == nil {
		return "default"
	}
	return s.appConfig.AppID
}

// HydrationHandler returns routes for the hydration service (unauthenticated, pre-auth).
// Exposed to the internet — POST /hydrate only.
func (s *Server) HydrationHandler() http.Handler {
	r := chi.NewRouter()
	r.Use(requestIDMiddleware)
	r.Use(loggingMiddleware(s.log))
	r.Use(chimiddleware.Recoverer)

	r.Post("/hydrate", s.handleHydrate())
	r.Get("/health", s.handleHealth())
	r.Head("/health", s.handleHealth())

	return r
}

// ReaderHandler returns routes for the context reader service (authenticated, post-auth).
// Internal — requires session token. Reads from Redis only; returns 404 on cache miss.
func (s *Server) ReaderHandler() http.Handler {
	r := chi.NewRouter()
	r.Use(requestIDMiddleware)
	r.Use(loggingMiddleware(s.log))
	r.Use(chimiddleware.Recoverer)

	r.Get("/data/{contextKey}/{resource}", s.handleData())
	r.Head("/data/{contextKey}/{resource}", s.handleData())
	r.Get("/context/{contextKey}", s.handleContext())
	r.Head("/context/{contextKey}", s.handleContext())
	r.Get("/health", s.handleHealth())
	r.Head("/health", s.handleHealth())

	return r
}

// Handler returns all routes on a single mux — used by cmd/server for local development
// and combined testing. Both hydration and reader routes are served on the same port.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(requestIDMiddleware)
	r.Use(loggingMiddleware(s.log))
	r.Use(chimiddleware.Recoverer)

	r.Post("/hydrate", s.handleHydrate())
	r.Get("/data/{contextKey}/{resource}", s.handleData())
	r.Head("/data/{contextKey}/{resource}", s.handleData())
	r.Get("/context/{contextKey}", s.handleContext())
	r.Head("/context/{contextKey}", s.handleContext())
	r.Get("/health", s.handleHealth())
	r.Head("/health", s.handleHealth())

	return r
}
