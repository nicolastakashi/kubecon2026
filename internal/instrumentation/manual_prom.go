package instrumentation

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	appmetrics "github.com/nicolastakashi/kubecon2026/internal/metrics"
)

type manualPromProvider struct {
	metrics *appmetrics.Bundle
}

func newManualPromProvider() (Provider, error) {
	bundle, err := appmetrics.NewBundle()
	if err != nil {
		return nil, fmt.Errorf("create manual prometheus bundle: %w", err)
	}

	return &manualPromProvider{metrics: bundle}, nil
}

func (p *manualPromProvider) RecordIngest(duration time.Duration) {
	p.metrics.StageDuration.WithLabelValues("ingest").Observe(duration.Seconds())
	p.metrics.ParcelsTotal.WithLabelValues("ingest", "success").Inc()
}

func (p *manualPromProvider) RecordProcess(outcome string, duration time.Duration) {
	p.metrics.StageDuration.WithLabelValues("process").Observe(duration.Seconds())
	p.metrics.ParcelsTotal.WithLabelValues("process", outcome).Inc()
}

func (p *manualPromProvider) RecordShip(duration time.Duration) {
	p.metrics.StageDuration.WithLabelValues("ship").Observe(duration.Seconds())
	p.metrics.ParcelsTotal.WithLabelValues("ship", "success").Inc()
}

func (p *manualPromProvider) RecordReworkReason(reason string) {
	p.metrics.ReworkReasonTotal.WithLabelValues(reason).Inc()
}

func (p *manualPromProvider) SetBacklog(ingested, readyToShip int) {
	p.metrics.BacklogGauge.WithLabelValues("ingested").Set(float64(ingested))
	p.metrics.BacklogGauge.WithLabelValues("ready_to_ship").Set(float64(readyToShip))
}

func (p *manualPromProvider) RecordHTTPRequest(method, endpoint string, statusCode int, duration time.Duration) {
	p.metrics.HTTPRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
	p.metrics.HTTPRequestsTotal.WithLabelValues(method, endpoint, strconv.Itoa(statusCode)).Inc()
}

func (p *manualPromProvider) Handler() http.Handler {
	return p.metrics.Handler()
}

func (p *manualPromProvider) Shutdown(context.Context) error {
	return nil
}
