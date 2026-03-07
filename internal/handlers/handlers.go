package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/nicolastakashi/kubecon2026/internal/instrumentation"
	"github.com/nicolastakashi/kubecon2026/internal/models"
)

type Handler struct {
	db            *sql.DB
	metrics       instrumentation.Provider
	UpdateBacklog func()
}

func New(db *sql.DB, metrics instrumentation.Provider, updateBacklog func()) *Handler {
	return &Handler{
		db:            db,
		metrics:       metrics,
		UpdateBacklog: updateBacklog,
	}
}

func writeJSON(w http.ResponseWriter, payload interface{}) {
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
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

	h.metrics.RecordIngest(time.Since(start))
	h.UpdateBacklog()

	response := map[string]interface{}{
		"id":      parcelID,
		"status":  "ingested",
		"message": "Parcel ingested successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, response)
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
		http.Error(w, "Invalid outcome. Must be one of: ok, rework, drop", http.StatusBadRequest)
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

	h.metrics.RecordProcess(req.Outcome, time.Since(start))

	if req.Outcome == "rework" && req.Reason != "" {
		h.metrics.RecordReworkReason(req.Reason)
	}

	h.UpdateBacklog()

	response := map[string]interface{}{
		"id":      parcelID,
		"status":  "processed",
		"outcome": req.Outcome,
		"message": fmt.Sprintf("Parcel processed with outcome: %s", req.Outcome),
	}

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, response)
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

	h.metrics.RecordShip(time.Since(start))
	h.UpdateBacklog()

	response := map[string]interface{}{
		"id":      parcelID,
		"status":  "shipped",
		"message": "Parcel shipped successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, response)
}

func (h *Handler) GetParcel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	parcelID := vars["id"]

	var parcel models.Parcel
	var createdAt, updatedAt string
	var outcome sql.NullString

	err := h.db.QueryRowContext(r.Context(),
		"SELECT id, status, outcome, created_at, updated_at FROM parcels WHERE id = ?",
		parcelID).Scan(&parcel.ID, &parcel.Status, &outcome, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		http.Error(w, "Parcel not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get parcel: %v", err), http.StatusInternalServerError)
		return
	}

	if outcome.Valid {
		parcel.Outcome = outcome.String
	}
	parcel.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	parcel.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, parcel)
}

func (h *Handler) ListParcels(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		"SELECT id, status, outcome, created_at, updated_at FROM parcels ORDER BY created_at DESC LIMIT 100")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list parcels: %v", err), http.StatusInternalServerError)
		return
	}
	defer func() {
		_ = rows.Close()
	}()

	var parcels []models.Parcel
	for rows.Next() {
		var parcel models.Parcel
		var createdAt, updatedAt string
		var outcome sql.NullString
		if err := rows.Scan(&parcel.ID, &parcel.Status, &outcome, &createdAt, &updatedAt); err != nil {
			continue
		}
		if outcome.Valid {
			parcel.Outcome = outcome.String
		}
		parcel.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		parcel.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		parcels = append(parcels, parcel)
	}

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, parcels)
}
