package main

import (
	"log"
	"net/http"

	"github.com/nicolastakashi/kubecon2026/internal/server"
)

func main() {
	srv, err := server.New("parcels.db")
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}
	defer srv.Close()

	port := ":8080"
	log.Printf("Packet Sorter server starting on %s", port)
	log.Printf("API endpoints:")
	log.Printf("  POST /api/parcels              - Ingest a new parcel")
	log.Printf("  POST /api/parcels/{id}/process - Process a parcel (outcome: ok/rework/drop)")
	log.Printf("  POST /api/parcels/{id}/ship    - Ship a parcel (if outcome is ok)")
	log.Printf("  GET  /api/parcels/{id}         - Get parcel status")
	log.Printf("  GET  /api/parcels              - List recent parcels")
	log.Printf("  GET  /metrics                  - Prometheus metrics")
	log.Fatal(http.ListenAndServe(port, srv.Router()))
}
