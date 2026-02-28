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
# Build and run
make run

# View metrics (while running)
make metrics

# Run e2e tests
make test-e2e

# Infer schema from telemetry
make infer-schema

# Validate telemetry against schema
make live-check

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

## Testing

The project uses Docker-based testing (no Kubernetes required):

```bash
# Run unit tests
make test

# Run e2e tests (starts app, runs tests, cleans up)
make test-e2e

# Run contract tests (validates telemetry against schema)
make live-check
```

CI runs both e2e and contract tests automatically on push/PR.

## Schema Inference Demo

This project demonstrates schema-driven observability using OpenTelemetry Weaver for the KubeCon talk: **"Schema Inference and Automation: A New Era for Telemetry Management"**.

### Quick Demo

```bash
# Step 1: Infer schema from app telemetry
make infer-schema

# Step 2: Validate telemetry against schema (happy path)
make live-check

# Step 3: Break a metric and validate (show contract violation)
# Edit internal/metrics/metrics.go, change a label name
make live-check   # Should show validation errors
```

### Standalone Commands (for multi-terminal demo)

```bash
# Schema Inference Demo:
# Terminal 1: Run Weaver infer
make run-weaver-infer

# Terminal 2: Run OTel Collector  
make run-collector

# Terminal 3: Run the app
make run

# Terminal 4: Run e2e tests (generates telemetry)
make test-e2e

# Contract Testing Demo:
# Terminal 1: Run Weaver live-check
make run-weaver-live-check

# Terminal 2-4: Same as above (collector, app, tests)
```

### Data Flow

```
App (:8080/metrics) → OTel Collector → Weaver (:4319) → registry/
```

### Demo Scenarios

1. **Infer schema from legacy app**: Run `make infer-schema` to generate `registry/registry.yaml` from live telemetry
2. **Contract testing**: Run `make live-check` to validate app telemetry matches the schema
3. **Breaking change detection**: Modify a metric name in `internal/metrics/metrics.go`, run `make live-check` to see:
   ```
   ✗ Found 1 violation(s):
     - new_metric_name: Metric does not exist in the registry.
   ```

## Docker (Optional)

```bash
# Build image
docker build -t packetsorter .

# Run
docker run -p 8080:8080 packetsorter
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