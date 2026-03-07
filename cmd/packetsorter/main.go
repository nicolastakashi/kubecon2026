package main

import (
	"log"
	"net/http"
	"os"

	"github.com/nicolastakashi/kubecon2026/internal/instrumentation"
	"github.com/nicolastakashi/kubecon2026/internal/server"
)

func main() {
	mode := instrumentation.ParseMode(os.Getenv("INSTRUMENTATION_MODE"))

	srv, err := server.New("parcels.db", mode)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}
	defer func() {
		if err := srv.Close(); err != nil {
			log.Printf("Failed to close server: %v", err)
		}
	}()

	port := ":8080"
	log.Printf("Packet Sorter server starting on %s", port)
	log.Printf("Instrumentation mode: %s", mode)
	log.Printf("API endpoints:")
	log.Printf("  POST /api/parcels              - Ingest a new parcel")
	log.Printf("  POST /api/parcels/{id}/process - Process a parcel (outcome: ok/rework/drop)")
	log.Printf("  POST /api/parcels/{id}/ship    - Ship a parcel (if outcome is ok)")
	log.Printf("  GET  /api/parcels/{id}         - Get parcel status")
	log.Printf("  GET  /api/parcels              - List recent parcels")
	log.Printf("  GET  /metrics                  - Prometheus metrics")
	log.Fatal(http.ListenAndServe(port, srv.Router()))
}
