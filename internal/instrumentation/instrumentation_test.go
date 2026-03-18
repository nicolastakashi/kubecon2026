package instrumentation

import (
	"testing"
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
