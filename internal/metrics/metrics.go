package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	ParcelsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "parcels_total",
			Help: "Total number of parcels processed by stage and outcome",
		},
		[]string{"stage", "outcome"},
	)

	StageDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "stage_duration_seconds",
			Help:    "Time spent processing parcels at each stage",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"stage"},
	)

	BacklogGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "parcels_backlog",
			Help: "Current number of parcels waiting at each stage",
		},
		[]string{"stage"},
	)

	ReworkReasonTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rework_reason_total",
			Help: "Total number of parcels sent to rework by reason",
		},
		[]string{"reason"},
	)

	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests by endpoint and status",
		},
		[]string{"method", "endpoint", "status"},
	)

	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration by endpoint",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)
)

func init() {
	prometheus.MustRegister(ParcelsTotal)
	prometheus.MustRegister(StageDuration)
	prometheus.MustRegister(BacklogGauge)
	prometheus.MustRegister(ReworkReasonTotal)
	prometheus.MustRegister(HTTPRequestsTotal)
	prometheus.MustRegister(HTTPRequestDuration)
}
