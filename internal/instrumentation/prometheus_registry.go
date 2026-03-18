package instrumentation

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func newPrometheusRegistry(extraCollectors ...prometheus.Collector) (*prometheus.Registry, http.Handler, error) {
	registry := prometheus.NewRegistry()

	for _, collector := range extraCollectors {
		if err := registry.Register(collector); err != nil {
			return nil, nil, fmt.Errorf("register collector: %w", err)
		}
	}

	handler, err := enrichPrometheusRegistry(registry)
	if err != nil {
		return nil, nil, err
	}

	return registry, handler, nil
}

func enrichPrometheusRegistry(registry *prometheus.Registry) (http.Handler, error) {
	if err := registry.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})); err != nil {
		return nil, fmt.Errorf("register process collector: %w", err)
	}

	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	return promhttp.InstrumentMetricHandler(registry, handler), nil
}
