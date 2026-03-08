package instrumentation

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	generatedotel "github.com/nicolastakashi/kubecon2026/generated/otel"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/otlptranslator"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

type generatedOTelProvider struct {
	metrics       *generatedotel.Metrics
	handler       http.Handler
	meterProvider *sdkmetric.MeterProvider
	ctx           context.Context
}

func newGeneratedOTelProvider() (Provider, error) {
	registry := prometheus.NewRegistry()

	exporter, err := otelprom.New(
		otelprom.WithRegisterer(registry),
		otelprom.WithoutTargetInfo(),
		otelprom.WithTranslationStrategy(otlptranslator.NoTranslation),
	)
	if err != nil {
		return nil, fmt.Errorf("create otel prometheus exporter: %w", err)
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
	)

	appMetrics, err := generatedotel.New(provider, "packetsorter")
	if err != nil {
		return nil, fmt.Errorf("create generated otel metrics: %w", err)
	}

	handler, err := enrichPrometheusRegistry(registry)
	if err != nil {
		return nil, fmt.Errorf("create otel prometheus registry: %w", err)
	}

	return &generatedOTelProvider{
		metrics:       appMetrics,
		handler:       handler,
		meterProvider: provider,
		ctx:           context.Background(),
	}, nil
}

func (p *generatedOTelProvider) RecordIngest(duration time.Duration) {
	p.metrics.StageDurationSeconds.Record(
		p.ctx,
		duration.Seconds(),
		metric.WithAttributes(generatedotel.AttrStage("ingest")),
	)
	p.metrics.ParcelsTotal.Add(
		p.ctx,
		1,
		metric.WithAttributes(generatedotel.AttrStage("ingest"), generatedotel.AttrOutcome("success")),
	)
}

func (p *generatedOTelProvider) RecordProcess(outcome string, duration time.Duration) {
	p.metrics.StageDurationSeconds.Record(
		p.ctx,
		duration.Seconds(),
		metric.WithAttributes(generatedotel.AttrStage("process")),
	)
	p.metrics.ParcelsTotal.Add(
		p.ctx,
		1,
		metric.WithAttributes(generatedotel.AttrStage("process"), generatedotel.AttrOutcome(outcome)),
	)
}

func (p *generatedOTelProvider) RecordShip(duration time.Duration) {
	p.metrics.StageDurationSeconds.Record(
		p.ctx,
		duration.Seconds(),
		metric.WithAttributes(generatedotel.AttrStage("ship")),
	)
	p.metrics.ParcelsTotal.Add(
		p.ctx,
		1,
		metric.WithAttributes(generatedotel.AttrStage("ship"), generatedotel.AttrOutcome("success")),
	)
}

func (p *generatedOTelProvider) RecordReworkReason(reason string) {
	p.metrics.ReworkReasonTotal.Add(
		p.ctx,
		1,
		metric.WithAttributes(generatedotel.AttrReason(reason)),
	)
}

func (p *generatedOTelProvider) SetBacklog(ingested, readyToShip int) {
	p.metrics.ParcelsBacklog.Record(
		p.ctx,
		float64(ingested),
		metric.WithAttributes(generatedotel.AttrStage("ingested")),
	)
	p.metrics.ParcelsBacklog.Record(
		p.ctx,
		float64(readyToShip),
		metric.WithAttributes(generatedotel.AttrStage("ready_to_ship")),
	)
}

func (p *generatedOTelProvider) RecordHTTPRequest(method, endpoint string, statusCode int, duration time.Duration) {
	p.metrics.HttpRequestDurationSeconds.Record(
		p.ctx,
		duration.Seconds(),
		metric.WithAttributes(generatedotel.AttrMethod(method), generatedotel.AttrEndpoint(endpoint)),
	)
	p.metrics.HttpRequestsTotal.Add(
		p.ctx,
		1,
		metric.WithAttributes(
			generatedotel.AttrMethod(method),
			generatedotel.AttrEndpoint(endpoint),
			generatedotel.AttrStatus(strconv.Itoa(statusCode)),
		),
	)
}

func (p *generatedOTelProvider) Handler() http.Handler {
	return p.handler
}

func (p *generatedOTelProvider) Shutdown(ctx context.Context) error {
	return p.meterProvider.Shutdown(ctx)
}
