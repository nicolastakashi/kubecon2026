package instrumentation

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	generatedprom "github.com/nicolastakashi/kubecon2026/generated/prom"
	"github.com/prometheus/client_golang/prometheus"
)

type generatedPromProvider struct {
	metrics *generatedprom.Metrics
	handler http.Handler
}

func newGeneratedPromProvider() (Provider, error) {
	metrics := generatedprom.New()
	collectors := metrics.Collectors()

	promCollectors := make([]prometheus.Collector, 0, len(collectors))
	promCollectors = append(promCollectors, collectors...)

	_, handler, err := newPrometheusRegistry(promCollectors...)
	if err != nil {
		return nil, fmt.Errorf("create generated prometheus registry: %w", err)
	}

	return &generatedPromProvider{
		metrics: metrics,
		handler: handler,
	}, nil
}

func (p *generatedPromProvider) RecordIngest(duration time.Duration) {
	p.metrics.StageDurationSeconds.With(prometheus.Labels{"stage": "ingest"}).Observe(duration.Seconds())
	p.metrics.ParcelsTotal.With(prometheus.Labels{"stage": "ingest", "outcome": "success"}).Inc()
}

func (p *generatedPromProvider) RecordProcess(outcome string, duration time.Duration) {
	p.metrics.StageDurationSeconds.With(prometheus.Labels{"stage": "process"}).Observe(duration.Seconds())
	p.metrics.ParcelsTotal.With(prometheus.Labels{"stage": "process", "outcome": outcome}).Inc()
}

func (p *generatedPromProvider) RecordShip(duration time.Duration) {
	p.metrics.StageDurationSeconds.With(prometheus.Labels{"stage": "ship"}).Observe(duration.Seconds())
	p.metrics.ParcelsTotal.With(prometheus.Labels{"stage": "ship", "outcome": "success"}).Inc()
}

func (p *generatedPromProvider) RecordReworkReason(reason string) {
	p.metrics.ReworkReasonTotal.With(prometheus.Labels{"reason": reason}).Inc()
}

func (p *generatedPromProvider) SetBacklog(ingested, readyToShip int) {
	p.metrics.ParcelsBacklog.With(prometheus.Labels{"stage": "ingested"}).Set(float64(ingested))
	p.metrics.ParcelsBacklog.With(prometheus.Labels{"stage": "ready_to_ship"}).Set(float64(readyToShip))
}

func (p *generatedPromProvider) RecordHTTPRequest(method, endpoint string, statusCode int, duration time.Duration) {
	p.metrics.HttpRequestDurationSeconds.With(prometheus.Labels{
		"endpoint": endpoint,
		"method":   method,
	}).Observe(duration.Seconds())
	p.metrics.HttpRequestsTotal.With(prometheus.Labels{
		"endpoint": endpoint,
		"method":   method,
		"status":   strconv.Itoa(statusCode),
	}).Inc()
}

func (p *generatedPromProvider) Handler() http.Handler {
	return p.handler
}

func (p *generatedPromProvider) Shutdown(context.Context) error {
	return nil
}
