# Packet Sorter - KubeCon Demo App

A simple Go application demonstrating Prometheus instrumentation with schema-driven observability principles.

## Features

- **SQLite persistence** for parcel tracking
- **Gorilla Mux** for HTTP routing  
- **Prometheus client_golang** for metrics instrumentation
- **Rich telemetry**:
  - `parcels_total{stage, outcome}` - Counter for parcels processed
  - `stage_duration_seconds{stage}` - Histogram for processing time
  - `parcels_backlog{stage}` - Gauge for backlog size
  - `rework_reason_total{reason}` - Counter for rework reasons
  - `http_requests_total{method, endpoint, status}` - HTTP request counter
  - `http_request_duration_seconds{method, endpoint}` - HTTP request latency

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/parcels` | Ingest a new parcel |
| POST | `/api/parcels/{id}/process` | Process parcel (body: `{"outcome": "ok\|rework\|drop", "reason": "..."}`) |
| POST | `/api/parcels/{id}/ship` | Ship a parcel (only if outcome=ok) |
| GET | `/api/parcels/{id}` | Get parcel status |
| GET | `/api/parcels` | List recent parcels |
| GET | `/health` | Health check |
| GET | `/metrics` | Prometheus metrics |

## Quick Start

### Using Make

```bash
# Build
make build

# Run
make run

# Run with load generator
make run-with-load

# View metrics (while running)
make metrics

# Clean up
make clean

# See all available commands
make help
```

### Manual

```bash
# Build
go build -o packetsorter .

# Run
./packetsorter

# Ingest a parcel
curl -X POST http://localhost:8080/api/parcels

# Process it (ok)
curl -X POST http://localhost:8080/api/parcels/PKG-xxx/process \
  -H "Content-Type: application/json" \
  -d '{"outcome": "ok"}'

# Ship it
curl -X POST http://localhost:8080/api/parcels/PKG-xxx/ship

# View metrics
curl http://localhost:8080/metrics
```

## OpenTelemetry Collector in E2E (Local + CI)

The e2e flow deploys an in-cluster OpenTelemetry Collector that scrapes the app's
Prometheus endpoint (`/metrics`) and runs a metrics pipeline.

- Local run: `make e2e`
- CI run: use the same `make e2e` flow

Collector setup in e2e manifests:
- Receiver: `prometheus` scraping `packetsorter.packetsorter.svc.cluster.local:8080/metrics`
- Processors: `memory_limiter`, `batch`
- Exporters: `debug` (always on), `otlphttp` (configurable by env vars)

The e2e target also port-forwards:
- app service on `http://localhost:8080`
- collector metrics on `http://localhost:8888`

This enables an e2e assertion that the collector has accepted metric points.

### Exporting to a backend later

By default, collector metrics are emitted to the `debug` exporter and OTLP HTTP
uses a local default endpoint. To ship to a real backend, set:

- `OTEL_EXPORTER_OTLP_ENDPOINT` (example: `http://your-backend:4318`)
- `OTEL_EXPORTER_OTLP_INSECURE` (`true` or `false`)

These env vars are read by the collector config in
`test/e2e/manifests/otel-collector.yaml`.

## Schema-Driven Demo Ideas

Perfect for your KubeCon talk on Telemetry First:

1. **Add priority label**: Modify `parcels_total` to include `priority={high,medium,low}` → show dashboard breakage
2. **Split outcome values**: Change `ok` to `accepted` → demonstrate backward compatibility
3. **Enforce cardinality**: Prevent unbounded `reason` labels → show data quality enforcement
4. **Migrate to OTel**: Port metrics while maintaining schema → show schema portability

## Docker (Optional)

```bash
# Build image
docker build -t packetsorter .

# Run
docker run -p 8080:8080 packetsorter
```

## Load Testing

```bash
# Generate continuous traffic
./scripts/loadtest.sh
```

This app is ~250 lines of Go with rich telemetry surface - perfect for demonstrating schema-driven observability!

## Vendored OTel Templates

This repo vendors the OpenTelemetry community Go registry templates under:

- `templates/vendor/otel-go/semconv/templates/registry/go`
- `templates/vendor/otel-go/VENDOR_METADATA`

Refresh to the latest upstream ref with:

```bash
make vendor-otel-templates
```

Pin to a specific tag or commit with:

```bash
make vendor-otel-templates OTEL_GO_REF=v1.38.0
```