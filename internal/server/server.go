package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/nicolastakashi/kubecon2026/internal/db"
	"github.com/nicolastakashi/kubecon2026/internal/handlers"
	"github.com/nicolastakashi/kubecon2026/internal/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	db      *db.DB
	handler *handlers.Handler
	router  *mux.Router
}

func New(dbPath string) (*Server, error) {
	database, err := db.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}

	s := &Server{
		db: database,
	}

	s.handler = handlers.New(database.Conn(), s.updateBacklogGauge)
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

	r.Handle("/metrics", promhttp.Handler()).Methods("GET")

	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
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

		duration := time.Since(start).Seconds()
		metrics.HTTPRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
		metrics.HTTPRequestsTotal.WithLabelValues(r.Method, path, fmt.Sprintf("%d", wrapped.statusCode)).Inc()
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

	metrics.BacklogGauge.WithLabelValues("ingested").Set(float64(ingested))
	metrics.BacklogGauge.WithLabelValues("ready_to_ship").Set(float64(readyToShip))
}

func (s *Server) Router() *mux.Router {
	return s.router
}

func (s *Server) Close() error {
	return s.db.Close()
}
