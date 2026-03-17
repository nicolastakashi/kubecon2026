# Packet Sorter

This repository is a small Go app that demonstrates schema-driven observability.

You do not need to understand the whole codebase to get the idea.

The simple version is:

- the app exposes metrics
- Weaver can learn a telemetry schema from those metrics
- that schema becomes the contract
- we can change how metrics are implemented without changing that contract

## What This Project Is

`Packet Sorter` is a tiny HTTP service that pretends to process parcels.

It gives us something realistic to instrument, while staying small enough to read quickly.

The interesting part is not parcel shipping. The interesting part is how telemetry is inferred, checked, and generated.

## Why This Repo Exists

Telemetry migrations are easy to break.

For example, during a migration it is common to accidentally:

- rename a metric
- remove a label
- change metric type
- change behavior between implementations

This repo shows a safer approach:

- infer a schema from live telemetry
- commit that schema
- validate implementations against it
- generate new instrumentation from the same schema

## The Three Instrumentation Modes

The app supports three implementations behind the same telemetry contract:

- `manual-prom` - handwritten Prometheus instrumentation
- `generated-prom` - Prometheus collectors generated from `registry/registry.yaml`
- `generated-otel` - OpenTelemetry instruments generated from the same schema and exposed through a Prometheus exporter

Set the mode with `INSTRUMENTATION_MODE`.

If you do not set it, the app uses `manual-prom`.

## The Main Weaver Commands

These are the three main Weaver commands used in this repo:

```bash
# Learn the telemetry contract from a running app
weaver registry infer --output ./registry

# Check if the app still matches that contract
weaver registry live-check --registry ./registry

# Generate instrumentation code from that contract
weaver registry generate --registry ./registry --templates ./templates/go . ./generated/prom
```

These examples are intentionally simple so the core idea is easy to understand.

## A Simple Mental Model

Think about the flow like this:

```text
app telemetry -> Weaver learns or checks -> registry becomes the contract
```

Then later:

```text
registry -> generated code -> new implementation
```

## Project Layout

- `cmd/packetsorter` - application entrypoint
- `internal/` - app logic, handlers, storage, and instrumentation wiring
- `registry/registry.yaml` - the committed telemetry contract
- `generated/prom/` - generated Prometheus code
- `generated/otel/` - generated OpenTelemetry code
- `templates/` - generation templates
- `test/e2e/` - end-to-end tests

## Quick Start

Run the app:

```bash
make run
```

Try another instrumentation mode:

```bash
INSTRUMENTATION_MODE=generated-prom make run
```

Send one request:

```bash
curl -X POST http://localhost:8080/api/parcels
```

See the metrics:

```bash
curl http://localhost:8080/metrics
```

## Common Commands

```bash
make run              # run the app
make test             # run unit tests
make test-e2e         # run end-to-end tests
make infer-schema     # infer registry from live telemetry
make live-check       # validate current mode against the registry
make generate         # generate Prometheus and OTel code
make schema-diff      # compare current telemetry with committed schema
make help             # list available targets
```

## API Endpoints

| Method | Endpoint                    | Description         |
| ------ | --------------------------- | ------------------- |
| POST   | `/api/parcels`              | Create a parcel     |
| POST   | `/api/parcels/{id}/process` | Process a parcel    |
| POST   | `/api/parcels/{id}/ship`    | Ship a parcel       |
| GET    | `/api/parcels/{id}`         | Get parcel status   |
| GET    | `/api/parcels`              | List recent parcels |
| GET    | `/health`                   | Health check        |
| GET    | `/metrics`                  | Prometheus metrics  |
