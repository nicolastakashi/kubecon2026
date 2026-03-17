#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=${ROOT_DIR:?ROOT_DIR is required}
APP_BIN=${APP_BIN:?APP_BIN is required}
INSTRUMENTATION_MODE=${INSTRUMENTATION_MODE:?INSTRUMENTATION_MODE is required}
CONTAINER_CLI=${CONTAINER_CLI:?CONTAINER_CLI is required}
OTEL_COLLECTOR_IMAGE=${OTEL_COLLECTOR_IMAGE:?OTEL_COLLECTOR_IMAGE is required}
OTEL_COLLECTOR_CONFIG=${OTEL_COLLECTOR_CONFIG:?OTEL_COLLECTOR_CONFIG is required}
OTEL_COLLECTOR_CONTAINER=${OTEL_COLLECTOR_CONTAINER:?OTEL_COLLECTOR_CONTAINER is required}
WEAVER_IMAGE=${WEAVER_IMAGE:?WEAVER_IMAGE is required}
WEAVER_CONTAINER=${WEAVER_CONTAINER:?WEAVER_CONTAINER is required}
WEAVER_LIVE_CHECK_CONTAINER=${WEAVER_LIVE_CHECK_CONTAINER:?WEAVER_LIVE_CHECK_CONTAINER is required}
WEAVER_LIVE_CHECK_PORT=${WEAVER_LIVE_CHECK_PORT:?WEAVER_LIVE_CHECK_PORT is required}
WEAVER_LIVE_CHECK_ADMIN_PORT=${WEAVER_LIVE_CHECK_ADMIN_PORT:?WEAVER_LIVE_CHECK_ADMIN_PORT is required}
WEAVER_INACTIVITY_TIMEOUT=${WEAVER_INACTIVITY_TIMEOUT:?WEAVER_INACTIVITY_TIMEOUT is required}
WEAVER_OUTPUT_DIR=${WEAVER_OUTPUT_DIR:?WEAVER_OUTPUT_DIR is required}
E2E_TEST_DIR=${E2E_TEST_DIR:?E2E_TEST_DIR is required}

APP_PID=""

cleanup() {
  if [[ -n "$APP_PID" ]]; then
    kill "$APP_PID" 2>/dev/null || true
  fi
  "$CONTAINER_CLI" rm -f "$WEAVER_CONTAINER" 2>/dev/null || true
  "$CONTAINER_CLI" rm -f "$WEAVER_LIVE_CHECK_CONTAINER" 2>/dev/null || true
  "$CONTAINER_CLI" rm -f "$OTEL_COLLECTOR_CONTAINER" 2>/dev/null || true
}

trap cleanup EXIT

if [[ ! -d "$ROOT_DIR/$WEAVER_OUTPUT_DIR" ]]; then
  echo "Error: $WEAVER_OUTPUT_DIR/ not found."
  echo "Run 'make infer-schema' first."
  exit 1
fi

echo ""
echo "=========================================="
echo "  Starting Live-Check Validation"
echo "=========================================="
echo ""

echo "[1/5] Starting Weaver live-check..."
"$CONTAINER_CLI" rm -f "$WEAVER_LIVE_CHECK_CONTAINER" 2>/dev/null || true
"$CONTAINER_CLI" rm -f "$WEAVER_CONTAINER" 2>/dev/null || true
"$CONTAINER_CLI" run -d --name "$WEAVER_LIVE_CHECK_CONTAINER" \
  -p "$WEAVER_LIVE_CHECK_PORT":4317 \
  -p "$WEAVER_LIVE_CHECK_ADMIN_PORT":8080 \
  -v "$ROOT_DIR/$WEAVER_OUTPUT_DIR:/registry:ro" \
  "$WEAVER_IMAGE" registry live-check \
  --registry /registry \
  --format json \
  --inactivity-timeout "$WEAVER_INACTIVITY_TIMEOUT" >/dev/null

sleep 2

echo "[2/5] Starting OTel Collector..."
"$CONTAINER_CLI" rm -f "$OTEL_COLLECTOR_CONTAINER" 2>/dev/null || true
"$CONTAINER_CLI" run -d --name "$OTEL_COLLECTOR_CONTAINER" \
  -p 4317:4317 -p 4318:4318 -p 8888:8888 -p 13133:13133 \
  -v "$ROOT_DIR/$OTEL_COLLECTOR_CONFIG:/conf/otel-collector-config.yaml:ro" \
  "$OTEL_COLLECTOR_IMAGE" --config=/conf/otel-collector-config.yaml >/dev/null

sleep 2

echo "[3/5] Starting application..."
INSTRUMENTATION_MODE="$INSTRUMENTATION_MODE" "$ROOT_DIR/$APP_BIN" &
APP_PID=$!

sleep 2

echo "[4/5] Running e2e tests..."
echo ""
(cd "$ROOT_DIR/$E2E_TEST_DIR" && go test -count=1 -v . -timeout=120s) || true
echo ""

echo "[5/5] Waiting for metrics to be validated..."
sleep 15

echo ""
echo "=========================================="
echo "  Live-Check Results"
echo "=========================================="
echo ""

VIOLATIONS=$("$CONTAINER_CLI" logs "$WEAVER_LIVE_CHECK_CONTAINER" 2>&1 | \
  sed -n '/^{/,$p' | \
  jq -s '[.[] | select(.metric.live_check_result.all_advice[]?.advice_level == "violation") | {name: .metric.name, message: .metric.live_check_result.all_advice[] | select(.advice_level == "violation") | .message}] | unique | [.[] | select(.name | test("^(go_gc_duration|process_|scrape_)") | not)]' 2>/dev/null || echo "[]")
VIOLATION_COUNT=$(echo "$VIOLATIONS" | jq 'length')

if [[ "$VIOLATION_COUNT" == "0" ]]; then
  echo "✓ No violations found - telemetry matches schema!"
  EXIT_CODE=0
else
  echo "✗ Found $VIOLATION_COUNT violation(s):"
  echo ""
  echo "$VIOLATIONS" | jq -r '.[] | "  - \(.name): \(.message)"'
  EXIT_CODE=1
fi

echo ""
curl -s -X POST "http://localhost:$WEAVER_LIVE_CHECK_ADMIN_PORT/stop" 2>/dev/null || true
sleep 2

echo ""
echo "=========================================="
echo "  Live-check validation complete!"
echo "=========================================="
echo ""

exit "$EXIT_CODE"
