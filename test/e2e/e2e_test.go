package e2e

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/nicolastakashi/kubecon2026/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupSuite creates and initializes an E2E test suite
func setupSuite(t *testing.T) *E2ESuite {
	t.Helper()
	suite := NewE2ESuite()
	suite.WaitForServer(t)
	return suite
}

// TestHappyPath_ParcelLifecycle tests the complete parcel flow
func TestHappyPath_ParcelLifecycle(t *testing.T) {
	suite := setupSuite(t)

	// Step 1: Ingest a parcel
	parcelID := suite.IngestParcel(t)

	// Step 2: Process with ok outcome
	resp := suite.ProcessParcel(t, parcelID, "ok", "")
	RequireStatusCode(t, resp, http.StatusOK)

	// Step 3: Ship the parcel
	resp = suite.ShipParcel(t, parcelID)
	RequireStatusCode(t, resp, http.StatusOK)

	// Step 4: Verify final state
	resp = suite.GetParcel(t, parcelID)
	defer CloseResponse(resp)

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var parcel models.Parcel
	AssertJSONResponse(t, resp, &parcel)

	assert.Equal(t, "shipped", parcel.Status)
	assert.Equal(t, "ok", parcel.Outcome)
}

// TestHappyPath_ReworkReasons tests all rework reason variations
func TestHappyPath_ReworkReasons(t *testing.T) {
	suite := setupSuite(t)

	tests := []struct {
		name   string
		reason string
	}{
		{"damaged", "damaged"},
		{"wrong_address", "wrong_address"},
		{"weight_issue", "weight_issue"},
		{"inspection", "inspection"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create parcel
			parcelID := suite.IngestParcel(t)

			// Process with rework and specific reason
			resp := suite.ProcessParcel(t, parcelID, "rework", tc.reason)
			defer CloseResponse(resp)

			RequireStatusCode(t, resp, http.StatusOK)

			// Verify state
			var result ParcelResponse
			AssertJSONResponse(t, resp, &result)

			assert.Equal(t, "rework", result.Outcome)
		})
	}
}

// TestHappyPath_ProcessOutcomes tests all process outcome variations
func TestHappyPath_ProcessOutcomes(t *testing.T) {
	suite := setupSuite(t)

	tests := []struct {
		name    string
		outcome string
	}{
		{"ok", "ok"},
		{"rework", "rework"},
		{"drop", "drop"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create parcel
			parcelID := suite.IngestParcel(t)

			// Process with the outcome
			resp := suite.ProcessParcel(t, parcelID, tc.outcome, "")
			defer CloseResponse(resp)

			RequireStatusCode(t, resp, http.StatusOK)
		})
	}
}

// TestHappyPath_BacklogTracking tests backlog tracking at different stages
func TestHappyPath_BacklogTracking(t *testing.T) {
	suite := setupSuite(t)

	// Create 3 parcels, don't process them
	parcelIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		parcelIDs[i] = suite.IngestParcel(t)
	}

	// List parcels and verify they exist
	resp := suite.ListParcels(t)
	defer CloseResponse(resp)

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var parcels []models.Parcel
	AssertJSONResponse(t, resp, &parcels)

	assert.GreaterOrEqual(t, len(parcels), 3, "Expected at least 3 parcels, got %d", len(parcels))

	// Now process 2 parcels with 'ok', don't ship yet
	for i := 0; i < 2; i++ {
		resp := suite.ProcessParcel(t, parcelIDs[i], "ok", "")
		CloseResponse(resp)

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Failed to process parcel %d: status %d", i, resp.StatusCode)
	}

	// List again to verify processed parcels exist
	resp = suite.ListParcels(t)
	defer CloseResponse(resp)

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var updatedParcels []models.Parcel
	AssertJSONResponse(t, resp, &updatedParcels)

	// Verify we still have parcels
	assert.GreaterOrEqual(t, len(updatedParcels), 3, "Expected at least 3 parcels after processing, got %d", len(updatedParcels))
}

// Failure Mode Tests

// TestFailure_ProcessInvalidJSON tests invalid JSON handling
func TestFailure_ProcessInvalidJSON(t *testing.T) {
	suite := setupSuite(t)

	// Create a parcel first
	parcelID := suite.IngestParcel(t)

	// Send invalid JSON
	url := fmt.Sprintf("%s/api/parcels/%s/process", suite.BaseURL, parcelID)
	resp, err := suite.Client.Post(url, "application/json", bytes.NewBufferString("{invalid json"))
	require.NoError(t, err, "Failed to make request")
	defer CloseResponse(resp)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Expected status 400 for invalid JSON, got %d", resp.StatusCode)
}

// TestFailure_ProcessInvalidOutcome tests invalid outcome handling
func TestFailure_ProcessInvalidOutcome(t *testing.T) {
	suite := setupSuite(t)

	// Create a parcel first
	parcelID := suite.IngestParcel(t)

	// Process with invalid outcome
	resp := suite.ProcessParcel(t, parcelID, "invalid_outcome", "")
	defer CloseResponse(resp)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Expected status 400 for invalid outcome, got %d", resp.StatusCode)
}

// TestFailure_ProcessNonExistentParcel tests processing non-existent parcel
func TestFailure_ProcessNonExistentParcel(t *testing.T) {
	suite := setupSuite(t)

	// Try to process a non-existent parcel
	fakeID := os.Getenv("TEST_NONEXISTENT_ID")
	if fakeID == "" {
		fakeID = "PKG-NONEXISTENT-TEST-12345"
	}

	resp := suite.ProcessParcel(t, fakeID, "ok", "")
	defer CloseResponse(resp)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Expected status 404 for non-existent parcel, got %d", resp.StatusCode)
}

// TestFailure_ShipNonReadyParcel tests shipping non-ready parcels
func TestFailure_ShipNonReadyParcel(t *testing.T) {
	suite := setupSuite(t)

	tests := []struct {
		name      string
		setupFunc func() string
	}{
		{
			name: "non-existent",
			setupFunc: func() string {
				fakeID := os.Getenv("TEST_NONEXISTENT_ID")
				if fakeID == "" {
					fakeID = "PKG-NONEXISTENT-TEST-12345"
				}
				return fakeID
			},
		},
		{
			name: "not-processed",
			setupFunc: func() string {
				return suite.IngestParcel(t)
			},
		},
		{
			name: "rework-outcome",
			setupFunc: func() string {
				parcelID := suite.IngestParcel(t)
				resp := suite.ProcessParcel(t, parcelID, "rework", "damaged")
				CloseResponse(resp)
				require.Equal(t, http.StatusOK, resp.StatusCode, "Failed to process parcel")
				return parcelID
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parcelID := tc.setupFunc()
			resp := suite.ShipParcel(t, parcelID)
			defer CloseResponse(resp)

			assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Expected status 404 for %s, got %d", tc.name, resp.StatusCode)
		})
	}
}

// TestFailure_GetNonExistentParcel tests getting non-existent parcel
func TestFailure_GetNonExistentParcel(t *testing.T) {
	suite := setupSuite(t)

	fakeID := os.Getenv("TEST_NONEXISTENT_ID")
	if fakeID == "" {
		fakeID = "PKG-NONEXISTENT-TEST-12345"
	}

	resp := suite.GetParcel(t, fakeID)
	defer CloseResponse(resp)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Expected status 404 for non-existent parcel, got %d", resp.StatusCode)
}

// Endpoint Coverage Tests

// TestEndpoints_HealthAndMetrics tests health and metrics endpoints
func TestEndpoints_HealthAndMetrics(t *testing.T) {
	suite := setupSuite(t)

	t.Run("health", func(t *testing.T) {
		resp, err := suite.Client.Get(fmt.Sprintf("%s/health", suite.BaseURL))
		require.NoError(t, err, "Failed to get health")
		defer CloseResponse(resp)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("metrics", func(t *testing.T) {
		// First, trigger some metrics by processing parcels
		// Create and process a rework parcel to ensure rework_reason_total exists
		reworkParcelID := suite.IngestParcel(t)
		resp := suite.ProcessParcel(t, reworkParcelID, "rework", "damaged")
		CloseResponse(resp)

		// Get metrics after triggering them
		resp, err := suite.Client.Get(fmt.Sprintf("%s/metrics", suite.BaseURL))
		require.NoError(t, err, "Failed to get metrics")
		defer CloseResponse(resp)

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify metrics contain expected content
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "Failed to read metrics body")

		expectedMetrics := []string{
			"parcels_total",
			"stage_duration_seconds",
			"parcels_backlog",
			"rework_reason_total",
			"http_requests_total",
			"http_request_duration_seconds",
		}

		metricsStr := string(body)
		for _, metric := range expectedMetrics {
			assert.Contains(t, metricsStr, metric, "Metrics endpoint missing expected metric: %s", metric)
		}
	})
}

// TestEndpoints_ListParcels tests listing parcels
func TestEndpoints_ListParcels(t *testing.T) {
	suite := setupSuite(t)

	// Create multiple parcels
	for i := 0; i < 3; i++ {
		suite.IngestParcel(t)
	}

	// List parcels
	resp := suite.ListParcels(t)
	defer CloseResponse(resp)

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var parcels []models.Parcel
	AssertJSONResponse(t, resp, &parcels)

	assert.GreaterOrEqual(t, len(parcels), 3, "Expected at least 3 parcels, got %d", len(parcels))
}

// TestEndpoints_GetParcel tests getting a single parcel
func TestEndpoints_GetParcel(t *testing.T) {
	suite := setupSuite(t)

	// Create a parcel
	parcelID := suite.IngestParcel(t)

	// Get the parcel
	resp := suite.GetParcel(t, parcelID)
	defer CloseResponse(resp)

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var parcel models.Parcel
	AssertJSONResponse(t, resp, &parcel)

	assert.Equal(t, parcelID, parcel.ID)
	assert.Equal(t, "ingested", parcel.Status)
}
