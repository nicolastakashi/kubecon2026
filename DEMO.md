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
```

---

## Demo Flow

### Act 1: The Problem (Legacy Telemetry)

**Talking Point:** "Here's a typical Go app with Prometheus metrics. The metrics were added ad-hoc over time, no formal schema."

```bash
# Show the current metrics implementation
cat internal/metrics/metrics.go
```

**Key points to highlight:**
- Manual metric definitions
- No schema validation
- Easy to make typos or inconsistencies

---

### Act 2: Schema Inference

**Talking Point:** "Weaver can infer a schema from live telemetry. Let's extract what we actually emit."

```bash
make infer-schema
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

**Then show the generated schema:**
```bash
head -100 registry/registry.yaml
```

---

### Act 3: Contract Testing (Happy Path)

**Talking Point:** "Now we have a schema. Let's validate that our app's telemetry matches it."

```bash
make live-check
```

**Expected output:**
```
==========================================
  Live-Check Results
==========================================

✓ No violations found - telemetry matches schema!

==========================================
  Live-check validation complete!
==========================================
```

---

### Act 4: Breaking the Contract (The Demo!)

**Talking Point:** "What happens when someone changes a metric name? Let's simulate a breaking change."

```bash
# Edit the metrics file - rename parcels_total to packages_total
sed -i.bak 's/parcels_total/packages_total/g' internal/metrics/metrics.go

# Show the change
grep -n "packages_total" internal/metrics/metrics.go
```

**Now run contract tests:**
```bash
make live-check
```

**Expected output:**
```
==========================================
  Live-Check Results
==========================================

✗ Found 1 violation(s):

  - packages_total: Metric does not exist in the registry.

==========================================
  Live-check validation complete!
==========================================
```

**Talking Point:** "The contract test caught the breaking change! In CI, this would fail the PR."

**Restore the original:**
```bash
mv internal/metrics/metrics.go.bak internal/metrics/metrics.go
```

---

### Act 5: Code Generation

**Talking Point:** "We can also generate type-safe instrumentation code from the schema."

```bash
make generate
```

**Show the generated code:**
```bash
cat generated/metrics.go
```

**Key points to highlight:**
- Single file, clean structure
- Type-safe attribute helpers (`AttrStage`, `AttrOutcome`)
- OTel SDK ready
- No manual metric definitions needed

---

### Act 6: CI Integration

**Talking Point:** "This all runs in CI. Every PR gets contract tested."

Show the GitHub Actions workflow:
```bash
cat .github/workflows/ci.yml
```

**Or show a PR with:**
- Contract test passing (green check)
- Contract test failing (PR comment with violations)

---

### Bonus: Schema Diff

**Talking Point:** "What if you want to see what changed before committing?"

```bash
make schema-diff
```

This re-infers the schema and shows a diff against the committed baseline.

---

## Quick Commands Reference

| Action | Command |
|--------|---------|
| Infer schema | `make infer-schema` |
| Validate telemetry | `make live-check` |
| Generate code | `make generate` |
| Compare schemas | `make schema-diff` |
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

**Reset metrics after demo:**
```bash
git checkout internal/metrics/metrics.go
```
