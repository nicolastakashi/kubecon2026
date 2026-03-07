# KubeCon 2026 Demo Script

## Schema Inference and Automation: A New Era for Telemetry Management

This document provides the step-by-step demo flow for the talk.

---

## Pre-Demo Setup

```bash
# Ensure clean state
make clean
docker rm -f weaver-infer weaver-live-check otel-collector-local 2>/dev/null || true

# Verify registry exists (or generate it)
ls registry/registry.yaml || make infer-schema

# Re-generate the migration SDKs from the committed schema
make generate
```

---

## Demo Flow

### Act 1: The Baseline (Legacy Telemetry)

**Talking Point:** "Here is the baseline: handwritten Prometheus instrumentation. The key question is how to migrate that implementation without breaking the telemetry contract."

```bash
# Show the handwritten baseline
sed -n '1,220p' internal/metrics/metrics.go

# Show the runtime mode switch
sed -n '1,200p' cmd/packetsorter/main.go
```

**Key points to highlight:**
- `manual-prom` is the legacy implementation
- `generated-prom` and `generated-otel` are migration targets
- the stable contract is `registry/registry.yaml`

---

### Act 2: Schema Inference

**Talking Point:** "Weaver can infer a schema from live telemetry. That gives us the compatibility contract that every implementation has to respect."

```bash
make infer-schema
head -100 registry/registry.yaml
```

**Expected output:**
```
==========================================
  Starting Schema Inference Pipeline
==========================================
[1/5] Starting Weaver infer...
[2/5] Starting OTel Collector...
[3/5] Starting application...
[4/5] Running e2e tests...
[5/5] Waiting for metrics to be scraped...
==========================================
  Schema inferred successfully!
  Output: registry/
==========================================
```

---

### Act 3: Migration 1 (Manual Prom -> Generated Prom)

**Talking Point:** "The first migration swaps handwritten Prometheus metrics for generated Prometheus collectors, but preserves the emitted schema."

```bash
INSTRUMENTATION_MODE=manual-prom make live-check
INSTRUMENTATION_MODE=generated-prom make live-check
```

**What to highlight:**
- both modes pass against the same committed schema
- the app code uses one semantic instrumentation facade
- `generated/prom/metrics.go` is derived from the schema, not handwritten

---

### Act 4: Migration 2 (Generated Prom -> Generated OTel)

**Talking Point:** "The second migration changes telemetry SDKs entirely. We switch from Prometheus collectors to generated OpenTelemetry instruments, but still expose the same contract."

```bash
INSTRUMENTATION_MODE=generated-otel make live-check
```

**What to highlight:**
- `generated/otel/metrics.go` comes from the same schema
- the collector and live-check workflow stay the same
- we changed implementation technology without changing observability behavior

---

### Act 5: Show All Modes Passing

**Talking Point:** "Here is the whole compatibility proof in one command."

```bash
make live-check-all
```

**Expected output:**
```text
==========================================
  Live-check mode: manual-prom
==========================================
...
✓ No violations found - telemetry matches schema!

==========================================
  Live-check mode: generated-prom
==========================================
...
✓ No violations found - telemetry matches schema!

==========================================
  Live-check mode: generated-otel
==========================================
...
✓ No violations found - telemetry matches schema!
```

---

### Act 6: Breaking the Contract

**Talking Point:** "Now let’s break one migration mode on purpose. The contract test should catch the drift immediately."

```bash
# Example: rename a metric or label in one implementation
INSTRUMENTATION_MODE=generated-prom make live-check
```

**Expected output:**
```text
==========================================
  Live-Check Results
==========================================

✗ Found 1 violation(s):

  - <metric-or-label>: <violation message>

==========================================
  Live-check validation complete!
==========================================
```

**What to highlight:**
- the app still runs
- the migration still compiles
- the contract test is what prevents silent observability regressions

---

### Act 7: Fix the Migration

**Talking Point:** "Now restore schema compatibility and re-run the same command. The migration is safe once the contract passes again."

```bash
INSTRUMENTATION_MODE=generated-prom make live-check
```

---

### Act 8: Code Generation

**Talking Point:** "Both migration targets are code-generated from the same committed contract."

```bash
make generate
sed -n '1,220p' generated/prom/metrics.go
sed -n '1,220p' generated/otel/metrics.go
```

**Key points to highlight:**
- same schema, two generated SDKs
- the business logic does not need to know which backend is active
- this makes migration repeatable and reviewable

---

### Act 9: CI / PR Integration

**Talking Point:** "This is not just a local trick. Every PR gets a telemetry migration report."

```bash
sed -n '1,260p' .github/workflows/ci.yml
```

**PR flow to show:**
- First commit intentionally breaks one migration mode
- GitHub shows the normal code diff in the PR
- The sticky PR comment shows per-mode `live-check` results and a schema diff excerpt
- The follow-up fix commit updates the same PR
- The sticky comment refreshes and the checks turn green

---

### Bonus: Local Schema Diff

**Talking Point:** "You can inspect schema drift locally before opening the PR."

```bash
make schema-diff
make schema-diff-report
```

---

## Quick Commands Reference

| Action | Command |
|--------|---------|
| Infer schema | `make infer-schema` |
| Validate one mode | `INSTRUMENTATION_MODE=<mode> make live-check` |
| Validate all modes | `make live-check-all` |
| Generate both SDKs | `make generate` |
| Compare schemas | `make schema-diff` / `make schema-diff-report` |
| Run all tests | `make test` |
| Clean up | `make clean` |

---

## Troubleshooting

**Port already in use:**
```bash
docker rm -f weaver-infer weaver-live-check otel-collector-local
```

**Registry not found:**
```bash
make infer-schema
```

**Regenerate the migration SDKs:**
```bash
make generate
```

**Reset to baseline mode:**
```bash
unset INSTRUMENTATION_MODE
```
