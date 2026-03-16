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
	store    *cache.Store
	hydrator *hydrator.Hydrator
	decoder  *cookie.Decoder
	backend  *services.Backend
	log      *slog.Logger
}

func NewServer(
	store *cache.Store,
	hyd *hydrator.Hydrator,
	decoder *cookie.Decoder,
	backend *services.Backend,
	log *slog.Logger,
) *Server {
	return &Server{
		store:    store,
		hydrator: hyd,
		decoder:  decoder,
		backend:  backend,
		log:      log,
	}
}

func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()

	r.Use(requestIDMiddleware)
	r.Use(loggingMiddleware(s.log))
	r.Use(chimiddleware.Recoverer)

	r.Post("/hydrate", s.handleHydrate())
	r.Get("/data/{userId}/{resource}", s.handleData())
	r.Head("/data/{userId}/{resource}", s.handleData())
	r.Get("/context/{userId}", s.handleContext())
	r.Head("/context/{userId}", s.handleContext())
	r.Get("/health", s.handleHealth())
	r.Head("/health", s.handleHealth())

	return r
}
