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