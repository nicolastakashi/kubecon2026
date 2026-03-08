package metrics

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Bundle struct {
	registry            *prometheus.Registry
	ParcelsTotal        *prometheus.CounterVec
	StageDuration       *prometheus.HistogramVec
	BacklogGauge        *prometheus.GaugeVec
	ReworkReasonTotal   *prometheus.CounterVec
	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec
}

func NewBundle() (*Bundle, error) {
	registry := prometheus.NewRegistry()

	bundle := &Bundle{
		registry: registry,
		ParcelsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "parcels_total",
				Help: "Total number of parcels processed by stage and outcome",
			},
			[]string{"stage", "outcome"},
		),
		StageDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "stage_duration_seconds",
				Help:    "Time spent processing parcels at each stage",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"stage"},
		),
		BacklogGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "parcels_backlog",
				Help: "Current number of parcels waiting at each stage",
			},
			[]string{"stage"},
		),
		ReworkReasonTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rework_reason_total",
				Help: "Total number of parcels sent to rework by reason",
			},
			[]string{"reason"},
		),
		HTTPRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total HTTP requests by endpoint and status",
			},
			[]string{"method", "endpoint", "status"},
		),
		HTTPRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration by endpoint",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "endpoint"},
		),
	}

	if err := registry.Register(collectors.NewGoCollector()); err != nil {
		return nil, fmt.Errorf("register go collector: %w", err)
	}
	if err := registry.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})); err != nil {
		return nil, fmt.Errorf("register process collector: %w", err)
	}
	for _, collector := range bundle.Collectors() {
		if err := registry.Register(collector); err != nil {
			return nil, fmt.Errorf("register collector: %w", err)
		}
	}

	return bundle, nil
}

func (b *Bundle) Collectors() []prometheus.Collector {
	return []prometheus.Collector{
		b.ParcelsTotal,
		b.StageDuration,
		b.BacklogGauge,
		b.ReworkReasonTotal,
		b.HTTPRequestsTotal,
		b.HTTPRequestDuration,
	}
}

func (b *Bundle) Handler() http.Handler {
	handler := promhttp.HandlerFor(b.registry, promhttp.HandlerOpts{})
	return promhttp.InstrumentMetricHandler(b.registry, handler)
}
