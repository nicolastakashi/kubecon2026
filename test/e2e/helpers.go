package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/nicolastakashi/kubecon2026/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// E2ESuite provides a test suite with common setup
type E2ESuite struct {
	Client  *http.Client
	BaseURL string
	Config  *TestConfig
}

// NewE2ESuite creates a new E2E test suite
func NewE2ESuite() *E2ESuite {
	config := LoadConfig()
	return &E2ESuite{
		Client: &http.Client{
			Timeout: config.Timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		BaseURL: config.BaseURL,
		Config:  config,
	}
}

// WaitForServer waits for the server to become healthy
func (s *E2ESuite) WaitForServer(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			t.Fatal("Server did not become healthy within 30 seconds")
			return
		default:
			req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/health", s.BaseURL), nil)
			if err != nil {
				time.Sleep(500 * time.Millisecond)
				continue
			}

			resp, err := s.Client.Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				_ = resp.Body.Close()
				return
			}
			if resp != nil {
				_ = resp.Body.Close()
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// IngestParcel creates a new parcel and returns its ID
func (s *E2ESuite) IngestParcel(t *testing.T) string {
	t.Helper()

	resp, err := s.Client.Post(fmt.Sprintf("%s/api/parcels", s.BaseURL), "application/json", nil)
	require.NoError(t, err, "Failed to ingest parcel")
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read response body")

	require.Equal(t, http.StatusCreated, resp.StatusCode,
		"Expected status 201, got %d: %s", resp.StatusCode, string(body))

	var result ParcelResponse
	err = json.Unmarshal(body, &result)
	require.NoError(t, err, "Failed to decode response: %v", err)
	require.NotEmpty(t, result.ID, "No parcel ID returned")

	if s.Config.Debug {
		t.Logf("Created parcel: %s", result.ID)
	}

	return result.ID
}

// ProcessParcel processes a parcel with the given outcome and reason
func (s *E2ESuite) ProcessParcel(t *testing.T, parcelID, outcome, reason string) *http.Response {
	t.Helper()

	reqBody := models.ProcessRequest{
		Outcome: outcome,
		Reason:  reason,
	}

	body, err := json.Marshal(reqBody)
	require.NoError(t, err, "Failed to marshal request: %v", err)

	url := fmt.Sprintf("%s/api/parcels/%s/process", s.BaseURL, parcelID)
	resp, err := s.Client.Post(url, "application/json", bytes.NewBuffer(body))
	require.NoError(t, err, "Failed to process parcel")

	return resp
}

// ShipParcel ships a parcel
func (s *E2ESuite) ShipParcel(t *testing.T, parcelID string) *http.Response {
	t.Helper()

	url := fmt.Sprintf("%s/api/parcels/%s/ship", s.BaseURL, parcelID)
	resp, err := s.Client.Post(url, "application/json", nil)
	require.NoError(t, err, "Failed to ship parcel")

	return resp
}

// GetParcel retrieves a parcel by ID
func (s *E2ESuite) GetParcel(t *testing.T, parcelID string) *http.Response {
	t.Helper()

	url := fmt.Sprintf("%s/api/parcels/%s", s.BaseURL, parcelID)
	resp, err := s.Client.Get(url)
	require.NoError(t, err, "Failed to get parcel")

	return resp
}

// ListParcels lists all parcels
func (s *E2ESuite) ListParcels(t *testing.T) *http.Response {
	t.Helper()

	resp, err := s.Client.Get(fmt.Sprintf("%s/api/parcels", s.BaseURL))
	require.NoError(t, err, "Failed to list parcels")

	return resp
}

// CloseResponse safely closes an HTTP response
func CloseResponse(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
}

// AssertStatusCode asserts that the response has the expected status code
func AssertStatusCode(t *testing.T, resp *http.Response, expected int, msgAndArgs ...interface{}) {
	t.Helper()
	assert.Equal(t, expected, resp.StatusCode, msgAndArgs...)
}

// RequireStatusCode requires that the response has the expected status code (fatal)
func RequireStatusCode(t *testing.T, resp *http.Response, expected int, msgAndArgs ...interface{}) {
	t.Helper()
	require.Equal(t, expected, resp.StatusCode, msgAndArgs...)
}

// AssertJSONResponse asserts that the response body matches expected JSON
func AssertJSONResponse(t *testing.T, resp *http.Response, target interface{}) {
	t.Helper()
	if resp != nil && resp.Body != nil {
		defer func() {
			_ = resp.Body.Close()
		}()
	}

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read response body")

	err = json.Unmarshal(body, target)
	require.NoError(t, err, "Failed to decode JSON response: %v. Body: %s", err, string(body))
}

// ParcelResponse for API responses that include additional fields like 'message'
type ParcelResponse struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Outcome string `json:"outcome,omitempty"`
	Message string `json:"message"`
}
