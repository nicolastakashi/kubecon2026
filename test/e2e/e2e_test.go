package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/nicolastakashi/kubecon2026/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	baseURL = "http://localhost:8080"
	timeout = 30 * time.Second
)

// ParcelResponse for API responses that include additional fields like 'message'
type ParcelResponse struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Outcome string `json:"outcome,omitempty"`
	Message string `json:"message"`
}

func setupHTTPClient() *http.Client {
	return &http.Client{
		Timeout: timeout,
	}
}

func waitForServer(t *testing.T, client *http.Client) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)

	for time.Now().Before(deadline) {
		resp, err := client.Get(fmt.Sprintf("%s/health", baseURL))
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatal("Server did not become healthy within 30 seconds")
}

func ingestParcel(t *testing.T, client *http.Client) string {
	t.Helper()

	resp, err := client.Post(fmt.Sprintf("%s/api/parcels", baseURL), "application/json", nil)
	require.NoError(t, err, "Failed to ingest parcel")
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "Expected status 201, got %d: %s", resp.StatusCode, string(body))

	var result ParcelResponse
	err = json.Unmarshal(body, &result)
	require.NoError(t, err, "Failed to decode response: %v", err)
	require.NotEmpty(t, result.ID, "No parcel ID returned")

	return result.ID
}

func processParcel(t *testing.T, client *http.Client, parcelID, outcome, reason string) *http.Response {
	t.Helper()

	reqBody := models.ProcessRequest{
		Outcome: outcome,
		Reason:  reason,
	}

	body, err := json.Marshal(reqBody)
	require.NoError(t, err, "Failed to marshal request: %v", err)

	url := fmt.Sprintf("%s/api/parcels/%s/process", baseURL, parcelID)
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(body))
	require.NoError(t, err, "Failed to process parcel")

	return resp
}

func shipParcel(t *testing.T, client *http.Client, parcelID string) *http.Response {
	t.Helper()

	url := fmt.Sprintf("%s/api/parcels/%s/ship", baseURL, parcelID)
	resp, err := client.Post(url, "application/json", nil)
	require.NoError(t, err, "Failed to ship parcel")

	return resp
}

func getParcel(t *testing.T, client *http.Client, parcelID string) *http.Response {
	t.Helper()

	url := fmt.Sprintf("%s/api/parcels/%s", baseURL, parcelID)
	resp, err := client.Get(url)
	require.NoError(t, err, "Failed to get parcel")

	return resp
}

func listParcels(t *testing.T, client *http.Client) *http.Response {
	t.Helper()

	resp, err := client.Get(fmt.Sprintf("%s/api/parcels", baseURL))
	require.NoError(t, err, "Failed to list parcels")

	return resp
}

func closeResponse(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
}

// Happy Path Tests

func TestHappyPath_ParcelLifecycle(t *testing.T) {
	client := setupHTTPClient()
	waitForServer(t, client)

	// Step 1: Ingest a parcel
	parcelID := ingestParcel(t, client)
	t.Logf("Created parcel: %s", parcelID)

	// Step 2: Process with ok outcome
	resp := processParcel(t, client, parcelID, "ok", "")
	closeResponse(resp)

	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "Expected status 200, got %d: %s", resp.StatusCode, string(body))

	// Step 3: Ship the parcel
	resp = shipParcel(t, client, parcelID)
	closeResponse(resp)

	body, _ = io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "Expected status 200, got %d: %s", resp.StatusCode, string(body))

	// Step 4: Verify final state
	resp = getParcel(t, client, parcelID)
	defer closeResponse(resp)

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var parcel models.Parcel
	err := json.NewDecoder(resp.Body).Decode(&parcel)
	require.NoError(t, err)

	assert.Equal(t, "shipped", parcel.Status)
	assert.Equal(t, "ok", parcel.Outcome)
}

func TestHappyPath_ReworkReasons(t *testing.T) {
	client := setupHTTPClient()
	waitForServer(t, client)

	reworkReasons := []string{"damaged", "wrong_address", "weight_issue", "inspection"}

	for _, reason := range reworkReasons {
		t.Run(reason, func(t *testing.T) {
			// Create parcel
			parcelID := ingestParcel(t, client)

			// Process with rework and specific reason
			resp := processParcel(t, client, parcelID, "rework", reason)
			defer closeResponse(resp)

			body, _ := io.ReadAll(resp.Body)
			require.Equal(t, http.StatusOK, resp.StatusCode, "Expected status 200 for reason '%s', got %d: %s", reason, resp.StatusCode, string(body))

			// Verify state
			var result ParcelResponse
			err := json.Unmarshal(body, &result)
			require.NoError(t, err)
			assert.Equal(t, "rework", result.Outcome)
		})
	}
}

func TestHappyPath_ProcessOutcomes(t *testing.T) {
	client := setupHTTPClient()
	waitForServer(t, client)

	outcomes := []string{"ok", "rework", "drop"}

	for _, outcome := range outcomes {
		t.Run(outcome, func(t *testing.T) {
			// Create parcel
			parcelID := ingestParcel(t, client)

			// Process with the outcome
			resp := processParcel(t, client, parcelID, outcome, "")
			defer closeResponse(resp)

			body, _ := io.ReadAll(resp.Body)
			require.Equal(t, http.StatusOK, resp.StatusCode, "Expected status 200 for outcome '%s', got %d: %s", outcome, resp.StatusCode, string(body))
		})
	}
}

func TestHappyPath_BacklogTracking(t *testing.T) {
	client := setupHTTPClient()
	waitForServer(t, client)

	// Create 3 parcels, don't process them
	parcelIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		parcelIDs[i] = ingestParcel(t, client)
	}

	// List parcels and verify they exist
	resp := listParcels(t, client)
	defer closeResponse(resp)

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var parcels []models.Parcel
	err := json.NewDecoder(resp.Body).Decode(&parcels)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(parcels), 3, "Expected at least 3 parcels, got %d", len(parcels))

	// Now process 2 parcels with 'ok', don't ship yet
	for i := 0; i < 2; i++ {
		resp := processParcel(t, client, parcelIDs[i], "ok", "")
		closeResponse(resp)
		require.Equal(t, http.StatusOK, resp.StatusCode, "Failed to process parcel %d: status %d", i, resp.StatusCode)
	}

	// List again to verify processed parcels exist
	resp = listParcels(t, client)
	defer closeResponse(resp)

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var updatedParcels []models.Parcel
	err = json.NewDecoder(resp.Body).Decode(&updatedParcels)
	require.NoError(t, err)

	// Verify we still have parcels
	assert.GreaterOrEqual(t, len(updatedParcels), 3, "Expected at least 3 parcels after processing, got %d", len(updatedParcels))
}

// Failure Mode Tests

func TestFailure_ProcessInvalidJSON(t *testing.T) {
	client := setupHTTPClient()
	waitForServer(t, client)

	// Create a parcel first
	parcelID := ingestParcel(t, client)

	// Send invalid JSON
	url := fmt.Sprintf("%s/api/parcels/%s/process", baseURL, parcelID)
	resp, err := client.Post(url, "application/json", bytes.NewBufferString("{invalid json"))
	require.NoError(t, err, "Failed to make request")
	defer closeResponse(resp)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Expected status 400 for invalid JSON, got %d", resp.StatusCode)
}

func TestFailure_ProcessInvalidOutcome(t *testing.T) {
	client := setupHTTPClient()
	waitForServer(t, client)

	// Create a parcel first
	parcelID := ingestParcel(t, client)

	// Process with invalid outcome
	resp := processParcel(t, client, parcelID, "invalid_outcome", "")
	defer closeResponse(resp)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Expected status 400 for invalid outcome, got %d", resp.StatusCode)
}

func TestFailure_ProcessNonExistentParcel(t *testing.T) {
	client := setupHTTPClient()
	waitForServer(t, client)

	// Try to process a non-existent parcel
	fakeID := "PKG-NONEXISTENT-12345"
	resp := processParcel(t, client, fakeID, "ok", "")
	defer closeResponse(resp)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Expected status 404 for non-existent parcel, got %d", resp.StatusCode)
}

func TestFailure_ShipNonReadyParcel(t *testing.T) {
	client := setupHTTPClient()
	waitForServer(t, client)

	// Test 1: Ship a non-existent parcel
	t.Run("non-existent", func(t *testing.T) {
		fakeID := "PKG-NONEXISTENT-12345"
		resp := shipParcel(t, client, fakeID)
		defer closeResponse(resp)

		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Expected status 404 for non-existent parcel, got %d", resp.StatusCode)
	})

	// Test 2: Ship a parcel that hasn't been processed
	t.Run("not-processed", func(t *testing.T) {
		parcelID := ingestParcel(t, client)
		resp := shipParcel(t, client, parcelID)
		defer closeResponse(resp)

		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Expected status 404 for unprocessed parcel, got %d", resp.StatusCode)
	})

	// Test 3: Ship a parcel with rework outcome
	t.Run("rework-outcome", func(t *testing.T) {
		parcelID := ingestParcel(t, client)
		resp := processParcel(t, client, parcelID, "rework", "damaged")
		closeResponse(resp)

		require.Equal(t, http.StatusOK, resp.StatusCode, "Failed to process parcel: status %d", resp.StatusCode)

		resp = shipParcel(t, client, parcelID)
		defer closeResponse(resp)

		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Expected status 404 for rework parcel, got %d", resp.StatusCode)
	})
}

func TestFailure_GetNonExistentParcel(t *testing.T) {
	client := setupHTTPClient()
	waitForServer(t, client)

	fakeID := "PKG-NONEXISTENT-12345"
	resp := getParcel(t, client, fakeID)
	defer closeResponse(resp)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Expected status 404 for non-existent parcel, got %d", resp.StatusCode)
}

// Endpoint Coverage Tests

func TestEndpoints_HealthAndMetrics(t *testing.T) {
	client := setupHTTPClient()
	waitForServer(t, client)

	t.Run("health", func(t *testing.T) {
		resp, err := client.Get(fmt.Sprintf("%s/health", baseURL))
		require.NoError(t, err, "Failed to get health")
		defer closeResponse(resp)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("metrics", func(t *testing.T) {
		resp, err := client.Get(fmt.Sprintf("%s/metrics", baseURL))
		require.NoError(t, err, "Failed to get metrics")
		defer closeResponse(resp)

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

func TestEndpoints_ListParcels(t *testing.T) {
	client := setupHTTPClient()
	waitForServer(t, client)

	// Create multiple parcels
	for i := 0; i < 3; i++ {
		ingestParcel(t, client)
	}

	// List parcels
	resp := listParcels(t, client)
	defer closeResponse(resp)

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var parcels []models.Parcel
	err := json.NewDecoder(resp.Body).Decode(&parcels)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(parcels), 3, "Expected at least 3 parcels, got %d", len(parcels))
}

func TestEndpoints_GetParcel(t *testing.T) {
	client := setupHTTPClient()
	waitForServer(t, client)

	// Create a parcel
	parcelID := ingestParcel(t, client)

	// Get the parcel
	resp := getParcel(t, client, parcelID)
	defer closeResponse(resp)

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var parcel models.Parcel
	err := json.NewDecoder(resp.Body).Decode(&parcel)
	require.NoError(t, err)

	assert.Equal(t, parcelID, parcel.ID)
	assert.Equal(t, "ingested", parcel.Status)
}

// Helper function
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
