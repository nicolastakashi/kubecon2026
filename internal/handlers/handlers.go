package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/nicolastakashi/kubecon2026/internal/metrics"
	"github.com/nicolastakashi/kubecon2026/internal/models"
)

type Handler struct {
	db            *sql.DB
	UpdateBacklog func()
}

func New(db *sql.DB, updateBacklog func()) *Handler {
	return &Handler{
		db:            db,
		UpdateBacklog: updateBacklog,
	}
}

func (h *Handler) IngestParcel(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	parcelID := fmt.Sprintf("PKG-%d", time.Now().UnixNano())

	_, err := h.db.ExecContext(r.Context(),
		"INSERT INTO parcels (id, status, outcome, created_at, updated_at) VALUES (?, 'ingested', NULL, ?, ?)",
		parcelID, time.Now(), time.Now())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to ingest parcel: %v", err), http.StatusInternalServerError)
		return
	}

	duration := time.Since(start).Seconds()
	metrics.StageDuration.WithLabelValues("ingest").Observe(duration)
	metrics.ParcelsTotal.WithLabelValues("ingest", "success").Inc()
	h.UpdateBacklog()

	response := map[string]interface{}{
		"id":      parcelID,
		"status":  "ingested",
		"message": "Parcel ingested successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) ProcessParcel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	parcelID := vars["id"]

	var req models.ProcessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	validOutcomes := map[string]bool{"ok": true, "rework": true, "drop": true}
	if !validOutcomes[req.Outcome] {
		http.Error(w, fmt.Sprintf("Invalid outcome. Must be one of: ok, rework, drop"), http.StatusBadRequest)
		return
	}

	start := time.Now()

	result, err := h.db.ExecContext(r.Context(),
		"UPDATE parcels SET status = 'processed', outcome = ?, updated_at = ? WHERE id = ? AND status = 'ingested'",
		req.Outcome, time.Now(), parcelID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to process parcel: %v", err), http.StatusInternalServerError)
		return
	}

	rows, err := result.RowsAffected()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to check rows affected: %v", err), http.StatusInternalServerError)
		return
	}

	if rows == 0 {
		http.Error(w, "Parcel not found or already processed", http.StatusNotFound)
		return
	}

	duration := time.Since(start).Seconds()
	metrics.StageDuration.WithLabelValues("process").Observe(duration)
	metrics.ParcelsTotal.WithLabelValues("process", req.Outcome).Inc()

	if req.Outcome == "rework" && req.Reason != "" {
		metrics.ReworkReasonTotal.WithLabelValues(req.Reason).Inc()
	}

	h.UpdateBacklog()

	response := map[string]interface{}{
		"id":      parcelID,
		"status":  "processed",
		"outcome": req.Outcome,
		"message": fmt.Sprintf("Parcel processed with outcome: %s", req.Outcome),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) ShipParcel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	parcelID := vars["id"]

	start := time.Now()

	result, err := h.db.ExecContext(r.Context(),
		"UPDATE parcels SET status = 'shipped', updated_at = ? WHERE id = ? AND status = 'processed' AND outcome = 'ok'",
		time.Now(), parcelID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to ship parcel: %v", err), http.StatusInternalServerError)
		return
	}

	rows, err := result.RowsAffected()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to check rows affected: %v", err), http.StatusInternalServerError)
		return
	}

	if rows == 0 {
		http.Error(w, "Parcel not found, not processed, or outcome not ok", http.StatusNotFound)
		return
	}

	duration := time.Since(start).Seconds()
	metrics.StageDuration.WithLabelValues("ship").Observe(duration)
	metrics.ParcelsTotal.WithLabelValues("ship", "success").Inc()
	h.UpdateBacklog()

	response := map[string]interface{}{
		"id":      parcelID,
		"status":  "shipped",
		"message": "Parcel shipped successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) GetParcel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	parcelID := vars["id"]

	var parcel models.Parcel
	var createdAt, updatedAt string

	err := h.db.QueryRowContext(r.Context(),
		"SELECT id, status, outcome, created_at, updated_at FROM parcels WHERE id = ?",
		parcelID).Scan(&parcel.ID, &parcel.Status, &parcel.Outcome, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		http.Error(w, "Parcel not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get parcel: %v", err), http.StatusInternalServerError)
		return
	}

	parcel.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	parcel.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(parcel)
}

func (h *Handler) ListParcels(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		"SELECT id, status, outcome, created_at, updated_at FROM parcels ORDER BY created_at DESC LIMIT 100")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list parcels: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var parcels []models.Parcel
	for rows.Next() {
		var parcel models.Parcel
		var createdAt, updatedAt string
		if err := rows.Scan(&parcel.ID, &parcel.Status, &parcel.Outcome, &createdAt, &updatedAt); err != nil {
			continue
		}
		parcel.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		parcel.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		parcels = append(parcels, parcel)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(parcels)
}
