SHELL := /usr/bin/env bash -o pipefail
.DEFAULT_GOAL := help

APP_NAME ?= packetsorter
BUILD_DIR ?= bin
APP_BIN := $(BUILD_DIR)/$(APP_NAME)
MAIN_FILE ?= cmd/packetsorter/main.go
DB_FILE ?= parcels.db
INSTRUMENTATION_MODE ?= manual-prom

CONTAINER_CLI ?= docker
IMAGE_NAME ?= $(APP_NAME)
IMAGE_TAG ?= latest

E2E_TEST_DIR ?= test/e2e

OTEL_COLLECTOR_IMAGE ?= otel/opentelemetry-collector-contrib:0.146.0
OTEL_COLLECTOR_CONFIG ?= config/otel-collector-config-local.yaml
OTEL_COLLECTOR_CONTAINER ?= otel-collector-local

WEAVER_IMAGE ?= otel/weaver:main
WEAVER_CONTAINER ?= weaver-infer
WEAVER_INFER_PORT ?= 4319
WEAVER_ADMIN_PORT ?= 4320
WEAVER_INACTIVITY_TIMEOUT ?= 300
WEAVER_OUTPUT_DIR ?= registry

WEAVER_LIVE_CHECK_CONTAINER ?= weaver-live-check
WEAVER_LIVE_CHECK_PORT ?= 4319
WEAVER_LIVE_CHECK_ADMIN_PORT ?= 4321

.PHONY: all build image clean \
	run run-collector run-weaver-infer run-weaver-live-check metrics \
	test test-e2e schema-diff-report \
	lint tidy \
	infer-schema live-check generate generate-prom generate-otel schema-diff \
	help

##@ Build
all: build ## Build the application binary.

build: ## Compile the application to bin/.
	@mkdir -p $(BUILD_DIR)
	go build -o $(APP_BIN) $(MAIN_FILE)

image: ## Build the local container image.
	$(CONTAINER_CLI) build -t $(IMAGE_NAME):$(IMAGE_TAG) .

clean: ## Remove generated build/test artifacts.
	rm -rf $(BUILD_DIR) $(DB_FILE) tmp/ generated/ $(WEAVER_OUTPUT_DIR)/
	rm -f coverage.out coverage.html

##@ Run
run: build ## Run the application locally.
	INSTRUMENTATION_MODE=$(INSTRUMENTATION_MODE) ./$(APP_BIN)

run-collector: ## Run the OTel collector (foreground).
	@echo "Starting OTel Collector..."
	@echo "Metrics:    http://localhost:8888/metrics"
	@echo "Health:     http://localhost:13133"
	@echo ""
	$(CONTAINER_CLI) run --rm --name $(OTEL_COLLECTOR_CONTAINER) \
		-p 4317:4317 -p 4318:4318 -p 8888:8888 -p 13133:13133 \
		-v $(PWD)/$(OTEL_COLLECTOR_CONFIG):/conf/otel-collector-config.yaml:ro \
		$(OTEL_COLLECTOR_IMAGE) --config=/conf/otel-collector-config.yaml

run-weaver-infer: ## Run Weaver infer (foreground).
	@mkdir -p $(WEAVER_OUTPUT_DIR)
	@echo "Starting Weaver infer..."
	@echo "OTLP Endpoint:  localhost:$(WEAVER_INFER_PORT)"
	@echo "Admin API:      http://localhost:$(WEAVER_ADMIN_PORT)/stop"
	@echo "Schema Output:  $(WEAVER_OUTPUT_DIR)/"
	@echo ""
	$(CONTAINER_CLI) run --rm --name $(WEAVER_CONTAINER) \
		-p $(WEAVER_INFER_PORT):4317 \
		-p $(WEAVER_ADMIN_PORT):8080 \
		-v $(PWD)/$(WEAVER_OUTPUT_DIR):/output \
		$(WEAVER_IMAGE) registry infer \
			--output /output \
			--inactivity-timeout $(WEAVER_INACTIVITY_TIMEOUT)

run-weaver-live-check: ## Run Weaver live-check (foreground).
	@if [ ! -d "$(WEAVER_OUTPUT_DIR)" ]; then \
		echo "Error: $(WEAVER_OUTPUT_DIR)/ not found."; \
		echo "Run 'make infer-schema' first."; \
		exit 1; \
	fi
	@echo "Starting Weaver live-check..."
	@echo "OTLP Endpoint:  localhost:$(WEAVER_LIVE_CHECK_PORT)"
	@echo "Admin API:      http://localhost:$(WEAVER_LIVE_CHECK_ADMIN_PORT)/stop"
	@echo "Schema:         $(WEAVER_OUTPUT_DIR)/"
	@echo ""
	$(CONTAINER_CLI) run --rm --name $(WEAVER_LIVE_CHECK_CONTAINER) \
		-p $(WEAVER_LIVE_CHECK_PORT):4317 \
		-p $(WEAVER_LIVE_CHECK_ADMIN_PORT):8080 \
		-v $(PWD)/$(WEAVER_OUTPUT_DIR):/registry:ro \
		$(WEAVER_IMAGE) registry live-check \
			--registry /registry \
			--quiet \
			--inactivity-timeout $(WEAVER_INACTIVITY_TIMEOUT)

metrics: ## Print metrics from a running local app.
	@curl -fsS http://localhost:8080/metrics

##@ Test
test: ## Run unit tests (excludes e2e).
	go test -short $$(go list ./... | grep -vE '/(test|tmp)(/|$$)')

test-e2e: ## Run e2e tests (assumes app is running on :8080).
	@cd $(E2E_TEST_DIR) && go test -count=1 -v . -timeout=120s

##@ Development
lint: ## Run formatters and linters.
	go fmt ./...
	go vet ./...
	@command -v golangci-lint >/dev/null && golangci-lint run || echo "golangci-lint not installed"

tidy: ## Ensure go.mod/go.sum are tidy.
	go mod tidy

##@ Schema Inference
infer-schema: build ## Infer schema from app telemetry (runs full pipeline).
	@ROOT_DIR="$(CURDIR)" \
	APP_BIN="$(APP_BIN)" \
	INSTRUMENTATION_MODE="$(INSTRUMENTATION_MODE)" \
	CONTAINER_CLI="$(CONTAINER_CLI)" \
	OTEL_COLLECTOR_IMAGE="$(OTEL_COLLECTOR_IMAGE)" \
	OTEL_COLLECTOR_CONFIG="$(OTEL_COLLECTOR_CONFIG)" \
	OTEL_COLLECTOR_CONTAINER="$(OTEL_COLLECTOR_CONTAINER)" \
	WEAVER_IMAGE="$(WEAVER_IMAGE)" \
	WEAVER_CONTAINER="$(WEAVER_CONTAINER)" \
	WEAVER_LIVE_CHECK_CONTAINER="$(WEAVER_LIVE_CHECK_CONTAINER)" \
	WEAVER_INFER_PORT="$(WEAVER_INFER_PORT)" \
	WEAVER_ADMIN_PORT="$(WEAVER_ADMIN_PORT)" \
	WEAVER_INACTIVITY_TIMEOUT="$(WEAVER_INACTIVITY_TIMEOUT)" \
	WEAVER_OUTPUT_DIR="$(WEAVER_OUTPUT_DIR)" \
	E2E_TEST_DIR="$(E2E_TEST_DIR)" \
	./scripts/infer-schema.sh

# Note: The following metrics are filtered out from violations:
# - go_gc_duration_seconds: Prometheus Summary not fully supported by OTel prometheusreceiver
# - process_*: Go runtime metrics not relevant to app schema
# - scrape_*: OTel Collector internal metrics
live-check: build ## Validate app telemetry against schema (runs full pipeline).
	@ROOT_DIR="$(CURDIR)" \
	APP_BIN="$(APP_BIN)" \
	INSTRUMENTATION_MODE="$(INSTRUMENTATION_MODE)" \
	CONTAINER_CLI="$(CONTAINER_CLI)" \
	OTEL_COLLECTOR_IMAGE="$(OTEL_COLLECTOR_IMAGE)" \
	OTEL_COLLECTOR_CONFIG="$(OTEL_COLLECTOR_CONFIG)" \
	OTEL_COLLECTOR_CONTAINER="$(OTEL_COLLECTOR_CONTAINER)" \
	WEAVER_IMAGE="$(WEAVER_IMAGE)" \
	WEAVER_CONTAINER="$(WEAVER_CONTAINER)" \
	WEAVER_LIVE_CHECK_CONTAINER="$(WEAVER_LIVE_CHECK_CONTAINER)" \
	WEAVER_LIVE_CHECK_PORT="$(WEAVER_LIVE_CHECK_PORT)" \
	WEAVER_LIVE_CHECK_ADMIN_PORT="$(WEAVER_LIVE_CHECK_ADMIN_PORT)" \
	WEAVER_INACTIVITY_TIMEOUT="$(WEAVER_INACTIVITY_TIMEOUT)" \
	WEAVER_OUTPUT_DIR="$(WEAVER_OUTPUT_DIR)" \
	E2E_TEST_DIR="$(E2E_TEST_DIR)" \
	./scripts/live-check.sh

GENERATE_BASE_DIR ?= generated
GENERATE_PROM_OUTPUT_DIR ?= $(GENERATE_BASE_DIR)/prom
GENERATE_OTEL_OUTPUT_DIR ?= $(GENERATE_BASE_DIR)/otel

generate: generate-prom generate-otel ## Generate Prometheus and OTel SDKs from registry.
	@echo ""
	@echo "=========================================="
	@echo "  Generated SDKs:"
	@echo "    - $(GENERATE_PROM_OUTPUT_DIR)/metrics.go"
	@echo "    - $(GENERATE_OTEL_OUTPUT_DIR)/metrics.go"
	@echo "=========================================="
	@echo ""

generate-prom: ## Generate Prometheus SDK from registry.
	@if [ ! -d "$(WEAVER_OUTPUT_DIR)" ]; then \
		echo "Error: $(WEAVER_OUTPUT_DIR)/ not found."; \
		echo "Run 'make infer-schema' first."; \
		exit 1; \
	fi
	@echo ""
	@echo "=========================================="
	@echo "  Generating Prometheus SDK metrics.go"
	@echo "=========================================="
	@echo ""
	@mkdir -p $(GENERATE_PROM_OUTPUT_DIR)
	@$(CONTAINER_CLI) run --rm \
		-v $(PWD)/$(WEAVER_OUTPUT_DIR):/registry:ro \
		-v $(PWD)/templates/go:/templates:ro \
		-v $(PWD)/$(GENERATE_PROM_OUTPUT_DIR):/output \
		$(WEAVER_IMAGE) registry generate \
			--registry /registry \
			--templates /templates \
			--skip-policies \
			. /output >/tmp/generated-prom.log 2>&1 || true
	@test -f "$(GENERATE_PROM_OUTPUT_DIR)/metrics.go"
	@echo ""
	@echo "=========================================="
	@echo "  Generated: $(GENERATE_PROM_OUTPUT_DIR)/metrics.go"
	@echo "=========================================="
	@echo ""

generate-otel: ## Generate OTel SDK metrics.go from registry.
	@if [ ! -d "$(WEAVER_OUTPUT_DIR)" ]; then \
		echo "Error: $(WEAVER_OUTPUT_DIR)/ not found."; \
		echo "Run 'make infer-schema' first."; \
		exit 1; \
	fi
	@echo ""
	@echo "=========================================="
	@echo "  Generating OTel SDK metrics.go"
	@echo "=========================================="
	@echo ""
	@mkdir -p $(GENERATE_OTEL_OUTPUT_DIR)
	@$(CONTAINER_CLI) run --rm \
		-v $(PWD)/$(WEAVER_OUTPUT_DIR):/registry:ro \
		-v $(PWD)/templates/otel:/templates:ro \
		-v $(PWD)/$(GENERATE_OTEL_OUTPUT_DIR):/output \
		$(WEAVER_IMAGE) registry generate \
			--registry /registry \
			--templates /templates \
			--skip-policies \
			. /output >/tmp/generated-otel.log 2>&1 || true
	@test -f "$(GENERATE_OTEL_OUTPUT_DIR)/metrics.go"
	@echo ""
	@echo "=========================================="
	@echo "  Generated: $(GENERATE_OTEL_OUTPUT_DIR)/metrics.go"
	@echo "=========================================="
	@echo ""

SCHEMA_BASELINE ?= registry/registry.yaml

schema-diff: build ## Show schema changes between baseline and current telemetry.
	@if [ ! -f "$(SCHEMA_BASELINE)" ]; then \
		echo "Error: No baseline schema found at $(SCHEMA_BASELINE)"; \
		echo "Run 'make infer-schema' first to create a baseline."; \
		exit 1; \
	fi
	@echo ""
	@echo "=========================================="
	@echo "  Schema Diff: Baseline vs Current"
	@echo "=========================================="
	@echo ""
	@# Backup current schema
	@cp -r $(WEAVER_OUTPUT_DIR) $(WEAVER_OUTPUT_DIR).baseline 2>/dev/null || true
	@# Infer fresh schema to tmp location
	@WEAVER_OUTPUT_DIR=registry.new $(MAKE) infer-schema --no-print-directory >/tmp/schema-diff.log 2>&1 || { cat /tmp/schema-diff.log; rm -f /tmp/schema-diff.log; exit 1; }
	@test -f registry.new/registry.yaml || { cat /tmp/schema-diff.log; rm -f /tmp/schema-diff.log; exit 1; }
	@# Compare
	@echo ""
	@echo "Changes detected:"
	@echo ""
	@go run ./cmd/registrynormalize "$(WEAVER_OUTPUT_DIR)/registry.yaml" > /tmp/registry-baseline.normalized.yaml
	@go run ./cmd/registrynormalize registry.new/registry.yaml > /tmp/registry-current.normalized.yaml
	@diff -u /tmp/registry-baseline.normalized.yaml /tmp/registry-current.normalized.yaml || true
	@# Cleanup
	@rm -rf registry.new
	@rm -rf $(WEAVER_OUTPUT_DIR).baseline 2>/dev/null || true
	@rm -f /tmp/registry-baseline.normalized.yaml /tmp/registry-current.normalized.yaml /tmp/schema-diff.log
	@echo ""

schema-diff-report: build ## Print raw schema diff for CI/PR comments.
	@if [ ! -f "$(SCHEMA_BASELINE)" ]; then \
		echo "Error: No baseline schema found at $(SCHEMA_BASELINE)"; \
		echo "Run 'make infer-schema' first to create a baseline."; \
		exit 1; \
	fi
	@WEAVER_OUTPUT_DIR=registry.new $(MAKE) infer-schema --no-print-directory >/tmp/schema-diff-report.log 2>&1 || { cat /tmp/schema-diff-report.log; rm -f /tmp/schema-diff-report.log; exit 1; }
	@test -f registry.new/registry.yaml || { cat /tmp/schema-diff-report.log; rm -f /tmp/schema-diff-report.log; exit 1; }
	@go run ./cmd/registrynormalize "$(WEAVER_OUTPUT_DIR)/registry.yaml" > /tmp/registry-baseline.normalized.yaml
	@go run ./cmd/registrynormalize registry.new/registry.yaml > /tmp/registry-current.normalized.yaml
	@diff -u /tmp/registry-baseline.normalized.yaml /tmp/registry-current.normalized.yaml || true
	@rm -rf registry.new
	@rm -f /tmp/registry-baseline.normalized.yaml /tmp/registry-current.normalized.yaml /tmp/schema-diff-report.log

##@ Help
help: ## Show this help.
	@awk 'BEGIN {FS = ":.*##"; print "Usage: make <target>\n"} \
	/^[a-zA-Z0-9_.-]+:.*##/ {printf "  %-22s %s\n", $$1, $$2} \
	/^##@/ {printf "\n%s\n", substr($$0, 5)}' $(MAKEFILE_LIST)
