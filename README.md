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

## Instrumentation Modes

The app supports three instrumentation implementations behind one stable telemetry contract:

- `manual-prom` - the original handwritten Prometheus instrumentation
- `generated-prom` - Prometheus collectors generated from `registry/registry.yaml`
- `generated-otel` - OpenTelemetry instruments generated from the same schema and exposed through a Prometheus exporter

Select the mode with `INSTRUMENTATION_MODE`. If omitted, the app defaults to `manual-prom`.

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
# Build and run the default mode (manual-prom)
make run

# Run a specific instrumentation mode
INSTRUMENTATION_MODE=generated-prom make run
INSTRUMENTATION_MODE=generated-otel make run

# Validate one mode against the committed schema
INSTRUMENTATION_MODE=generated-prom make live-check

# Validate all modes against the committed schema
make live-check-all

# Generate both SDKs from the committed schema
make generate

# See all available commands
make help
```

### Manual

```bash
# Build and run
go build -o packetsorter ./cmd/packetsorter
INSTRUMENTATION_MODE=generated-otel ./packetsorter

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

# Run e2e tests (assumes app is already running)
make test-e2e

# Run contract tests for one mode
INSTRUMENTATION_MODE=generated-otel make live-check

# Run contract tests for all three modes
make live-check-all

# Compare current telemetry against committed schema
make schema-diff
make schema-diff-report
```

CI now publishes a sticky PR comment with a telemetry migration report. It summarizes `live-check` for `manual-prom`, `generated-prom`, and `generated-otel`, and includes a schema diff snippet when the emitted telemetry changes.

## Schema Inference & Migration Demo

This project demonstrates schema-driven observability using OpenTelemetry Weaver for the KubeCon talk: **"Schema Inference and Automation: A New Era for Telemetry Management"**.

For the full demo script, see **[DEMO.md](DEMO.md)**.

### Quick Demo

```bash
# Step 1: Infer schema from app telemetry
make infer-schema

# Step 2: Validate all instrumentation modes against the same schema
make live-check-all

# Step 3: Generate both migration targets from the same schema
make generate

# Step 4: Break a metric and validate (show contract violation)
# Example: rename a generated or manual label/metric in your feature branch
INSTRUMENTATION_MODE=generated-prom make live-check   # Should show validation errors

# Step 5: Review schema drift locally before opening a PR
make schema-diff-report
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

# Migration demo:
# Terminal 3: swap implementations without changing the contract
INSTRUMENTATION_MODE=generated-prom make run
INSTRUMENTATION_MODE=generated-otel make run

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
2. **Contract testing across migrations**: Run `make live-check-all` to prove all three implementations match the schema
3. **Breaking change detection**: Modify a metric name or label in one implementation, run `INSTRUMENTATION_MODE=<mode> make live-check` to see:
   ```
   ✗ Found 1 violation(s):
     - new_metric_name: Metric does not exist in the registry.
   ```
4. **SDK generation**: Run `make generate` to produce `generated/prom/metrics.go` and `generated/otel/metrics.go`
5. **PR review**: Open a PR and inspect the sticky telemetry migration report comment with per-mode results and a schema diff excerpt
6. **Schema diff**: Run `make schema-diff` or `make schema-diff-report` to compare current telemetry against the committed baseline

## Docker (Optional)

```bash
# Build image
docker build -t packetsorter .

# Run a specific mode
docker run -e INSTRUMENTATION_MODE=generated-otel -p 8080:8080 packetsorter
```

This app is still intentionally small, but it now demonstrates three instrumentation implementations behind one schema contract, which makes it a good fit for migration and PR-review demos.