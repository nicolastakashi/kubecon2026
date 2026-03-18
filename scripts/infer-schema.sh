#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=${ROOT_DIR:?ROOT_DIR is required}
APP_BIN=${APP_BIN:?APP_BIN is required}
INSTRUMENTATION_MODE=${INSTRUMENTATION_MODE:?INSTRUMENTATION_MODE is required}
CONTAINER_CLI=${CONTAINER_CLI:?CONTAINER_CLI is required}
CONTAINER_HOST_GATEWAY_ARG=${CONTAINER_HOST_GATEWAY_ARG:-}
OTEL_COLLECTOR_IMAGE=${OTEL_COLLECTOR_IMAGE:?OTEL_COLLECTOR_IMAGE is required}
OTEL_COLLECTOR_CONFIG=${OTEL_COLLECTOR_CONFIG:?OTEL_COLLECTOR_CONFIG is required}
OTEL_COLLECTOR_CONTAINER=${OTEL_COLLECTOR_CONTAINER:?OTEL_COLLECTOR_CONTAINER is required}
WEAVER_IMAGE=${WEAVER_IMAGE:?WEAVER_IMAGE is required}
WEAVER_CONTAINER=${WEAVER_CONTAINER:?WEAVER_CONTAINER is required}
WEAVER_LIVE_CHECK_CONTAINER=${WEAVER_LIVE_CHECK_CONTAINER:?WEAVER_LIVE_CHECK_CONTAINER is required}
WEAVER_INFER_PORT=${WEAVER_INFER_PORT:?WEAVER_INFER_PORT is required}
WEAVER_ADMIN_PORT=${WEAVER_ADMIN_PORT:?WEAVER_ADMIN_PORT is required}
WEAVER_INACTIVITY_TIMEOUT=${WEAVER_INACTIVITY_TIMEOUT:?WEAVER_INACTIVITY_TIMEOUT is required}
WEAVER_OUTPUT_DIR=${WEAVER_OUTPUT_DIR:?WEAVER_OUTPUT_DIR is required}
E2E_TEST_DIR=${E2E_TEST_DIR:?E2E_TEST_DIR is required}

APP_PID=""

cleanup() {
  if [[ -n "$APP_PID" ]]; then
    kill "$APP_PID" 2>/dev/null || true
  fi
  "$CONTAINER_CLI" rm -f "$WEAVER_CONTAINER" >/dev/null 2>&1 || true
  "$CONTAINER_CLI" rm -f "$WEAVER_LIVE_CHECK_CONTAINER" >/dev/null 2>&1 || true
  "$CONTAINER_CLI" rm -f "$OTEL_COLLECTOR_CONTAINER" >/dev/null 2>&1 || true
}

trap cleanup EXIT

echo ""
echo "=========================================="
echo "  Starting Schema Inference Pipeline"
echo "=========================================="
echo ""

mkdir -p "$ROOT_DIR/$WEAVER_OUTPUT_DIR"

echo "[1/5] Starting Weaver infer..."
"$CONTAINER_CLI" rm -f "$WEAVER_CONTAINER" >/dev/null 2>&1 || true
"$CONTAINER_CLI" rm -f "$WEAVER_LIVE_CHECK_CONTAINER" >/dev/null 2>&1 || true
"$CONTAINER_CLI" run -d --name "$WEAVER_CONTAINER" \
  -p "$WEAVER_INFER_PORT":4317 \
  -p "$WEAVER_ADMIN_PORT":8080 \
  -v "$ROOT_DIR/$WEAVER_OUTPUT_DIR:/output" \
  "$WEAVER_IMAGE" registry infer \
  --output /output \
  --inactivity-timeout "$WEAVER_INACTIVITY_TIMEOUT" >/dev/null

sleep 2

echo "[2/5] Starting OTel Collector..."
"$CONTAINER_CLI" rm -f "$OTEL_COLLECTOR_CONTAINER" >/dev/null 2>&1 || true
OTEL_COLLECTOR_RUN_ARGS=()
if [[ -n "$CONTAINER_HOST_GATEWAY_ARG" ]]; then
  OTEL_COLLECTOR_RUN_ARGS+=($CONTAINER_HOST_GATEWAY_ARG)
fi
"$CONTAINER_CLI" run -d --name "$OTEL_COLLECTOR_CONTAINER" \
  "${OTEL_COLLECTOR_RUN_ARGS[@]}" \
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

echo "[5/5] Waiting for metrics to be scraped..."
sleep 10

echo "Stopping Weaver to flush schema..."
curl -s -X POST "http://localhost:$WEAVER_ADMIN_PORT/stop" 2>/dev/null || true
"$CONTAINER_CLI" wait "$WEAVER_CONTAINER" >/dev/null 2>&1 || true

if [[ ! -f "$ROOT_DIR/$WEAVER_OUTPUT_DIR/registry.yaml" ]]; then
  echo "Error: $WEAVER_OUTPUT_DIR/registry.yaml was not created."
  exit 1
fi

echo ""
echo "=========================================="
echo "  Schema inferred successfully!"
echo "  Output: $WEAVER_OUTPUT_DIR/"
echo "=========================================="
echo ""
