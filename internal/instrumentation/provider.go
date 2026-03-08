package instrumentation

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Mode string

const (
	ModeManualProm    Mode = "manual-prom"
	ModeGeneratedProm Mode = "generated-prom"
	ModeGeneratedOTel Mode = "generated-otel"
)

type Provider interface {
	RecordIngest(duration time.Duration)
	RecordProcess(outcome string, duration time.Duration)
	RecordShip(duration time.Duration)
	RecordReworkReason(reason string)
	SetBacklog(ingested, readyToShip int)
	RecordHTTPRequest(method, endpoint string, statusCode int, duration time.Duration)
	Handler() http.Handler
	Shutdown(context.Context) error
}

func ParseMode(raw string) Mode {
	if strings.TrimSpace(raw) == "" {
		return ModeManualProm
	}

	return Mode(strings.ToLower(strings.TrimSpace(raw)))
}

func New(mode Mode) (Provider, error) {
	switch mode {
	case ModeManualProm:
		return newManualPromProvider()
	case ModeGeneratedProm:
		return newGeneratedPromProvider()
	case ModeGeneratedOTel:
		return newGeneratedOTelProvider()
	default:
		return nil, fmt.Errorf("unsupported instrumentation mode %q", mode)
	}
}
