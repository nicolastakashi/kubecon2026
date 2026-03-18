package instrumentation

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"
)

func TestParseModeDefaultsToManualProm(t *testing.T) {
	if got := ParseMode(""); got != ModeManualProm {
		t.Fatalf("ParseMode(\"\") = %q, want %q", got, ModeManualProm)
	}
}

func TestNewRejectsUnknownMode(t *testing.T) {
	t.Parallel()

	if _, err := New(Mode("unknown")); err == nil {
		t.Fatal("New() error = nil, want error for unknown mode")
	}
}

func TestProvidersExposeCompatibleMetrics(t *testing.T) {
	t.Parallel()

	patterns := []*regexp.Regexp{
		regexp.MustCompile(`parcels_total\{[^}]*outcome="success"[^}]*stage="ingest"[^}]*\}\s+1`),
		regexp.MustCompile(`parcels_total\{[^}]*outcome="rework"[^}]*stage="process"[^}]*\}\s+1`),
		regexp.MustCompile(`parcels_total\{[^}]*outcome="success"[^}]*stage="ship"[^}]*\}\s+1`),
		regexp.MustCompile(`parcels_backlog\{[^}]*stage="ingested"[^}]*\}\s+2`),
		regexp.MustCompile(`parcels_backlog\{[^}]*stage="ready_to_ship"[^}]*\}\s+1`),
		regexp.MustCompile(`rework_reason_total\{[^}]*reason="damaged"[^}]*\}\s+1`),
		regexp.MustCompile(`http_requests_total\{[^}]*endpoint="/api/parcels"[^}]*method="POST"[^}]*status="201"[^}]*\}\s+1`),
		regexp.MustCompile(`stage_duration_seconds_(bucket|count|sum)`),
		regexp.MustCompile(`http_request_duration_seconds_(bucket|count|sum)`),
	}

	tests := []Mode{ModeManualProm, ModeGeneratedProm, ModeGeneratedOTel}
	for _, mode := range tests {
		t.Run(string(mode), func(t *testing.T) {
			provider, err := New(mode)
			if err != nil {
				t.Fatalf("New(%q) error = %v", mode, err)
			}
			defer func() {
				if err := provider.Shutdown(context.Background()); err != nil {
					t.Fatalf("Shutdown() error = %v", err)
				}
			}()

			provider.RecordIngest(15 * time.Millisecond)
			provider.RecordProcess("rework", 20*time.Millisecond)
			provider.RecordReworkReason("damaged")
			provider.RecordShip(25 * time.Millisecond)
			provider.SetBacklog(2, 1)
			provider.RecordHTTPRequest("POST", "/api/parcels", http.StatusCreated, 30*time.Millisecond)

			req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			rec := httptest.NewRecorder()
			provider.Handler().ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("metrics handler status = %d, want %d", rec.Code, http.StatusOK)
			}

			body, err := io.ReadAll(rec.Result().Body)
			if err != nil {
				t.Fatalf("ReadAll() error = %v", err)
			}

			metricsText := string(body)
			if regexp.MustCompile(`(?m)^go_[^{\s]+`).MatchString(metricsText) {
				t.Fatalf("metrics output unexpectedly contains default go runtime metrics\n%s", metricsText)
			}

			for _, pattern := range patterns {
				if !pattern.MatchString(metricsText) {
					t.Fatalf("metrics output missing pattern %q\n%s", pattern.String(), metricsText)
				}
			}
		})
	}
}
