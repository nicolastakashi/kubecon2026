package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/nicolastakashi/kubecon2026/internal/db"
	"github.com/nicolastakashi/kubecon2026/internal/handlers"
	"github.com/nicolastakashi/kubecon2026/internal/instrumentation"
)

type Server struct {
	db      *db.DB
	handler *handlers.Handler
	metrics instrumentation.Provider
	router  *mux.Router
}

func New(dbPath string, mode instrumentation.Mode) (*Server, error) {
	database, err := db.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}

	metricsProvider, err := instrumentation.New(mode)
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("failed to create instrumentation provider: %w", err)
	}

	s := &Server{
		db:      database,
		metrics: metricsProvider,
	}

	s.handler = handlers.New(database.Conn(), metricsProvider, s.updateBacklogGauge)
	s.setupRouter()
	s.updateBacklogGauge()

	return s, nil
}

func (s *Server) setupRouter() {
	r := mux.NewRouter()

	api := r.PathPrefix("/api").Subrouter()
	api.Use(s.middleware)

	api.HandleFunc("/parcels", s.handler.IngestParcel).Methods("POST")
	api.HandleFunc("/parcels/{id}/process", s.handler.ProcessParcel).Methods("POST")
	api.HandleFunc("/parcels/{id}/ship", s.handler.ShipParcel).Methods("POST")
	api.HandleFunc("/parcels/{id}", s.handler.GetParcel).Methods("GET")
	api.HandleFunc("/parcels", s.handler.ListParcels).Methods("GET")

	r.Handle("/metrics", s.metrics.Handler()).Methods("GET")

	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "healthy"}); err != nil {
			http.Error(w, fmt.Sprintf("failed to encode health response: %v", err), http.StatusInternalServerError)
		}
	}).Methods("GET")

	s.router = r
}

func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		route := mux.CurrentRoute(r)
		path, _ := route.GetPathTemplate()

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		s.metrics.RecordHTTPRequest(r.Method, path, wrapped.statusCode, time.Since(start))
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (s *Server) updateBacklogGauge() {
	ingested, readyToShip, err := s.db.GetBacklogCounts()
	if err != nil {
		return
	}

	s.metrics.SetBacklog(ingested, readyToShip)
}

func (s *Server) Router() *mux.Router {
	return s.router
}

func (s *Server) Close() error {
	return errors.Join(
		s.metrics.Shutdown(context.Background()),
		s.db.Close(),
	)
}
