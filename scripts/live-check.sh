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
WEAVER_LIVE_CHECK_PORT=${WEAVER_LIVE_CHECK_PORT:?WEAVER_LIVE_CHECK_PORT is required}
WEAVER_LIVE_CHECK_ADMIN_PORT=${WEAVER_LIVE_CHECK_ADMIN_PORT:?WEAVER_LIVE_CHECK_ADMIN_PORT is required}
WEAVER_INACTIVITY_TIMEOUT=${WEAVER_INACTIVITY_TIMEOUT:?WEAVER_INACTIVITY_TIMEOUT is required}
WEAVER_OUTPUT_DIR=${WEAVER_OUTPUT_DIR:?WEAVER_OUTPUT_DIR is required}
E2E_TEST_DIR=${E2E_TEST_DIR:?E2E_TEST_DIR is required}
LIVE_CHECK_SUMMARY_FILE=${LIVE_CHECK_SUMMARY_FILE:-}

APP_PID=""
STDOUT_LOGS=""
STDERR_LOGS=""
JSON_LOGS=""
WEAVER_STDOUT_LOG=""
WEAVER_STDERR_LOG=""
APP_LOG_FILE=""
E2E_LOG_FILE=""
SUMMARY_BODY=""
DEBUG_BODY=""

append_summary() {
  SUMMARY_BODY+="$1"$'\n'
}

append_debug() {
  DEBUG_BODY+="$1"$'\n'
}

capture_debug_logs() {
  if [[ -n "$STDOUT_LOGS" ]]; then
    append_debug "stdout:"
    append_debug "$STDOUT_LOGS"
  fi
  if [[ -n "$STDERR_LOGS" ]]; then
    if [[ -n "$DEBUG_BODY" ]]; then
      append_debug ""
    fi
    append_debug "stderr:"
    append_debug "$STDERR_LOGS"
  fi
}

capture_file_debug() {
  local label=$1
  local path=$2

  if [[ -f "$path" && -s "$path" ]]; then
    if [[ -n "$DEBUG_BODY" ]]; then
      append_debug ""
    fi
    append_debug "$label:"
    while IFS= read -r log_line; do
      append_debug "$log_line"
    done < "$path"
  fi
}

emit_summary() {
  if [[ -n "$LIVE_CHECK_SUMMARY_FILE" ]]; then
    tee "$LIVE_CHECK_SUMMARY_FILE"
  else
    cat
  fi
}

cleanup() {
  if [[ -n "$APP_PID" ]]; then
    kill "$APP_PID" 2>/dev/null || true
    wait "$APP_PID" 2>/dev/null || true
  fi
  "$CONTAINER_CLI" rm -f "$WEAVER_CONTAINER" >/dev/null 2>&1 || true
  "$CONTAINER_CLI" rm -f "$WEAVER_LIVE_CHECK_CONTAINER" >/dev/null 2>&1 || true
  "$CONTAINER_CLI" rm -f "$OTEL_COLLECTOR_CONTAINER" >/dev/null 2>&1 || true
  rm -f "$WEAVER_STDOUT_LOG" "$WEAVER_STDERR_LOG" "$APP_LOG_FILE" "$E2E_LOG_FILE" >/dev/null 2>&1 || true
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
"$CONTAINER_CLI" rm -f "$WEAVER_LIVE_CHECK_CONTAINER" >/dev/null 2>&1 || true
"$CONTAINER_CLI" rm -f "$WEAVER_CONTAINER" >/dev/null 2>&1 || true
"$CONTAINER_CLI" run -d --name "$WEAVER_LIVE_CHECK_CONTAINER" \
  -p "$WEAVER_LIVE_CHECK_PORT":4317 \
  -p "$WEAVER_LIVE_CHECK_ADMIN_PORT":8080 \
  -v "$ROOT_DIR/$WEAVER_OUTPUT_DIR:/registry:ro" \
  "$WEAVER_IMAGE" registry live-check \
  --otlp-grpc-address 0.0.0.0 \
  --registry /registry \
  --format json \
  --quiet \
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
  -p 8888:8888 -p 13133:13133 \
  -v "$ROOT_DIR/$OTEL_COLLECTOR_CONFIG:/conf/otel-collector-config.yaml:ro" \
  "$OTEL_COLLECTOR_IMAGE" --config=/conf/otel-collector-config.yaml >/dev/null

sleep 2

echo "[3/5] Starting application..."
APP_LOG_FILE=$(mktemp)
INSTRUMENTATION_MODE="$INSTRUMENTATION_MODE" "$ROOT_DIR/$APP_BIN" >"$APP_LOG_FILE" 2>&1 &
APP_PID=$!

sleep 2

echo "[4/5] Running e2e tests..."
E2E_LOG_FILE=$(mktemp)
if (cd "$ROOT_DIR/$E2E_TEST_DIR" && go test -count=1 -v . -timeout=120s) >"$E2E_LOG_FILE" 2>&1; then
  echo "[4/5] E2E tests completed."
else
  echo "[4/5] E2E tests failed; continuing to collect live-check results."
  capture_file_debug "e2e test output" "$E2E_LOG_FILE"
fi

echo "[5/5] Waiting for metrics to be validated..."

WEAVER_STDOUT_LOG=$(mktemp)
WEAVER_STDERR_LOG=$(mktemp)

for ((i = 0; i < WEAVER_INACTIVITY_TIMEOUT; i++)); do
  "$CONTAINER_CLI" logs "$WEAVER_LIVE_CHECK_CONTAINER" >"$WEAVER_STDOUT_LOG" 2>"$WEAVER_STDERR_LOG" || true
  STDOUT_LOGS=$(<"$WEAVER_STDOUT_LOG")
  STDERR_LOGS=$(<"$WEAVER_STDERR_LOG")

  if [[ -n "$STDOUT_LOGS" ]] && JSON_LOGS=$(jq -s '.' "$WEAVER_STDOUT_LOG" 2>/dev/null); then
    break
  fi
  sleep 1
done

if [[ -z "$JSON_LOGS" ]]; then
  append_summary "✗ Weaver live-check did not emit parseable JSON output on stdout."
  capture_debug_logs
  capture_file_debug "application output" "$APP_LOG_FILE"
  capture_file_debug "e2e test output" "$E2E_LOG_FILE"
  EXIT_CODE=1
else
  METRIC_LOGS=$(printf '%s\n' "$JSON_LOGS" | jq '
    [
      .[]
      | if type == "array" then .[] else . end
      | select(
        type == "object"
        and (.metric | type == "object")
        and (.metric.live_check_result | type == "object")
      )
    ]
  ')
  METRIC_COUNT=$(printf '%s\n' "$METRIC_LOGS" | jq 'length')

  if [[ "$METRIC_COUNT" == "0" ]]; then
    append_summary "✗ Weaver live-check emitted JSON, but not the expected metric results."
    capture_debug_logs
    capture_file_debug "application output" "$APP_LOG_FILE"
    capture_file_debug "e2e test output" "$E2E_LOG_FILE"
    EXIT_CODE=1
  else
    ADVICE=$(printf '%s\n' "$METRIC_LOGS" | jq '
      [
        .[]
        | .metric as $metric
        | $metric.live_check_result.all_advice[]?
        | select((.advice_level // .level // "info") != "info")
        | {
            level: (.advice_level // .level // "unknown"),
            name: $metric.name,
            message: .message
          }
      ]
      | unique
      | map(select(.name | test("^(go_gc_duration|process_|scrape_|up$)") | not))
      | map(select(.message | contains("is not stable; stability = development.") | not))
    ')
    ADVICE_COUNT=$(printf '%s\n' "$ADVICE" | jq 'length')

    if [[ "$ADVICE_COUNT" == "0" ]]; then
      append_summary "✓ No violations found - telemetry matches schema!"
      EXIT_CODE=0
    else
      append_summary "✗ Found $ADVICE_COUNT contract issue(s):"
      append_summary ""
      while IFS= read -r advice_line; do
        append_summary "$advice_line"
      done < <(printf '%s\n' "$ADVICE" | jq -r '.[] | "  - [\(.level)] \(.name): \(.message)"')
      EXIT_CODE=1
    fi
  fi
fi

{
  echo ""
  echo "=========================================="
  echo "  Live-Check Results"
  echo "=========================================="
  echo ""
  printf '%s' "$SUMMARY_BODY"
  echo ""
  echo "=========================================="
  echo "  Live-check validation complete!"
  echo "=========================================="
  echo ""
} | emit_summary

if [[ -n "$DEBUG_BODY" ]]; then
  echo "Debug output:"
  printf '%s' "$DEBUG_BODY"
fi

curl -s -X POST "http://localhost:$WEAVER_LIVE_CHECK_ADMIN_PORT/stop" 2>/dev/null || true
"$CONTAINER_CLI" rm -f "$WEAVER_LIVE_CHECK_CONTAINER" >/dev/null 2>&1 || true

exit "$EXIT_CODE"
