SHELL := /usr/bin/env bash -o pipefail
.DEFAULT_GOAL := help

APP_NAME ?= packetsorter
BUILD_DIR ?= bin
APP_BIN := $(BUILD_DIR)/$(APP_NAME)
MAIN_FILE ?= cmd/packetsorter/main.go
DB_FILE ?= parcels.db

CONTAINER_CLI ?= docker
IMAGE_NAME ?= $(APP_NAME)
IMAGE_TAG ?= latest

TOOLS_DIR := $(CURDIR)/tmp/bin
WEAVER_VERSION ?= 0.21.2

CLUSTER_NAME ?= packetsorter-e2e
E2E_MANIFESTS ?= test/e2e/manifests
E2E_TEST_DIR ?= test/e2e
E2E_TEST_CMD := cd $(E2E_TEST_DIR) && go test -count=1 -v . -timeout=120s

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

WEAVER := $(TOOLS_DIR)/weaver
OTEL_GO_REPO := https://github.com/open-telemetry/opentelemetry-go.git
OTEL_GO_REF ?= main
OTEL_GO_VENDOR_DIR := templates/vendor/otel-go

.PHONY: all build image clean \
	run run-collector run-weaver-infer run-weaver-live-check metrics \
	test test-e2e \
	lint tidy \
	kind-up kind-down kind-test e2e \
	weaver vendor-otel-templates \
	infer-schema live-check \
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
	./$(APP_BIN)

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
			--inactivity-timeout $(WEAVER_INACTIVITY_TIMEOUT)

metrics: ## Print metrics from a running local app.
	@curl -fsS http://localhost:8080/metrics

##@ Test
test: ## Run unit tests.
	go test -race -short ./...

test-e2e: ## Run e2e tests (assumes app is running on :8080).
	@$(E2E_TEST_CMD)

##@ Development
lint: ## Run formatters and linters.
	go fmt ./...
	go vet ./...
	@command -v golangci-lint >/dev/null && golangci-lint run || echo "golangci-lint not installed"

tidy: ## Ensure go.mod/go.sum are tidy.
	go mod tidy

##@ Kubernetes
kind-up: image ## Create a fresh kind cluster and deploy app + collector.
	@command -v kind >/dev/null || { echo "kind not installed"; exit 1; }
	@kind get clusters | grep -q $(CLUSTER_NAME) && kind delete cluster --name $(CLUSTER_NAME) || true
	kind create cluster --name $(CLUSTER_NAME)
	kind load docker-image $(IMAGE_NAME):$(IMAGE_TAG) --name $(CLUSTER_NAME)
	kubectl apply -f $(E2E_MANIFESTS)/
	kubectl wait --for=condition=available deployment/$(APP_NAME) -n packetsorter --timeout=60s
	kubectl wait --for=condition=available deployment/otel-collector -n packetsorter --timeout=60s
	@echo ""
	@kubectl get pods -n packetsorter

kind-down: ## Delete the kind cluster.
	kind delete cluster --name $(CLUSTER_NAME) 2>/dev/null || true

kind-test: kind-up ## Run e2e tests against kind and clean up automatically.
	@kubectl port-forward -n packetsorter svc/$(APP_NAME) 8080:8080 >/dev/null 2>&1 & PF_PID=$$!; \
	trap 'kill $$PF_PID 2>/dev/null; $(MAKE) -s kind-down' EXIT; \
	sleep 3; \
	cd $(E2E_TEST_DIR) && TEST_BASE_URL=http://localhost:8080 go test -v . -timeout=120s

e2e: kind-test ## Compatibility alias for previous e2e target name.

##@ Schema Inference
infer-schema: build ## Infer schema from app telemetry (runs full pipeline).
	@echo ""
	@echo "=========================================="
	@echo "  Starting Schema Inference Pipeline"
	@echo "=========================================="
	@echo ""
	@mkdir -p $(WEAVER_OUTPUT_DIR)
	@# Start Weaver infer in background
	@echo "[1/5] Starting Weaver infer..."
	@$(CONTAINER_CLI) rm -f $(WEAVER_CONTAINER) 2>/dev/null || true
	@$(CONTAINER_CLI) run -d --name $(WEAVER_CONTAINER) \
		-p $(WEAVER_INFER_PORT):4317 \
		-p $(WEAVER_ADMIN_PORT):8080 \
		-v $(PWD)/$(WEAVER_OUTPUT_DIR):/output \
		$(WEAVER_IMAGE) registry infer \
			--output /output \
			--inactivity-timeout $(WEAVER_INACTIVITY_TIMEOUT) >/dev/null
	@sleep 2
	@# Start Collector in background
	@echo "[2/5] Starting OTel Collector..."
	@$(CONTAINER_CLI) rm -f $(OTEL_COLLECTOR_CONTAINER) 2>/dev/null || true
	@$(CONTAINER_CLI) run -d --name $(OTEL_COLLECTOR_CONTAINER) \
		-p 4317:4317 -p 4318:4318 -p 8888:8888 -p 13133:13133 \
		-v $(PWD)/$(OTEL_COLLECTOR_CONFIG):/conf/otel-collector-config.yaml:ro \
		$(OTEL_COLLECTOR_IMAGE) --config=/conf/otel-collector-config.yaml >/dev/null
	@sleep 2
	@# Start app in background
	@echo "[3/5] Starting application..."
	@./$(APP_BIN) & APP_PID=$$!; \
	sleep 2; \
	echo "[4/5] Running e2e tests..."; \
	echo ""; \
	$(E2E_TEST_CMD) || true; \
	echo ""; \
	echo "[5/5] Waiting for metrics to be scraped..."; \
	sleep 10; \
	echo "Stopping Weaver to flush schema..."; \
	curl -s -X POST http://localhost:$(WEAVER_ADMIN_PORT)/stop 2>/dev/null || true; \
	sleep 2; \
	kill $$APP_PID 2>/dev/null || true
	@# Cleanup containers
	@$(CONTAINER_CLI) rm -f $(WEAVER_CONTAINER) 2>/dev/null || true
	@$(CONTAINER_CLI) rm -f $(OTEL_COLLECTOR_CONTAINER) 2>/dev/null || true
	@echo ""
	@echo "=========================================="
	@echo "  Schema inferred successfully!"
	@echo "  Output: $(WEAVER_OUTPUT_DIR)/"
	@echo "=========================================="
	@echo ""

live-check: build ## Validate app telemetry against schema (runs full pipeline).
	@if [ ! -d "$(WEAVER_OUTPUT_DIR)" ]; then \
		echo "Error: $(WEAVER_OUTPUT_DIR)/ not found."; \
		echo "Run 'make infer-schema' first."; \
		exit 1; \
	fi
	@echo ""
	@echo "=========================================="
	@echo "  Starting Live-Check Validation"
	@echo "=========================================="
	@echo ""
	@# Start Weaver live-check in background
	@echo "[1/5] Starting Weaver live-check..."
	@$(CONTAINER_CLI) rm -f $(WEAVER_LIVE_CHECK_CONTAINER) 2>/dev/null || true
	@$(CONTAINER_CLI) run -d --name $(WEAVER_LIVE_CHECK_CONTAINER) \
		-p $(WEAVER_LIVE_CHECK_PORT):4317 \
		-p $(WEAVER_LIVE_CHECK_ADMIN_PORT):8080 \
		-v $(PWD)/$(WEAVER_OUTPUT_DIR):/registry:ro \
		$(WEAVER_IMAGE) registry live-check \
			--registry /registry \
			--inactivity-timeout $(WEAVER_INACTIVITY_TIMEOUT) >/dev/null
	@sleep 2
	@# Start Collector in background
	@echo "[2/5] Starting OTel Collector..."
	@$(CONTAINER_CLI) rm -f $(OTEL_COLLECTOR_CONTAINER) 2>/dev/null || true
	@$(CONTAINER_CLI) run -d --name $(OTEL_COLLECTOR_CONTAINER) \
		-p 4317:4317 -p 4318:4318 -p 8888:8888 -p 13133:13133 \
		-v $(PWD)/$(OTEL_COLLECTOR_CONFIG):/conf/otel-collector-config.yaml:ro \
		$(OTEL_COLLECTOR_IMAGE) --config=/conf/otel-collector-config.yaml >/dev/null
	@sleep 2
	@# Start app in background
	@echo "[3/5] Starting application..."
	@./$(APP_BIN) & APP_PID=$$!; \
	sleep 2; \
	echo "[4/5] Running e2e tests..."; \
	echo ""; \
	$(E2E_TEST_CMD) || true; \
	echo ""; \
	echo "[5/5] Waiting for metrics to be validated..."; \
	sleep 10; \
	echo ""; \
	echo "=========================================="; \
	echo "  Live-Check Results"; \
	echo "=========================================="; \
	$(CONTAINER_CLI) logs $(WEAVER_LIVE_CHECK_CONTAINER) 2>&1 | tail -50; \
	echo ""; \
	curl -s -X POST http://localhost:$(WEAVER_LIVE_CHECK_ADMIN_PORT)/stop 2>/dev/null || true; \
	sleep 2; \
	kill $$APP_PID 2>/dev/null || true
	@# Cleanup containers
	@$(CONTAINER_CLI) rm -f $(WEAVER_LIVE_CHECK_CONTAINER) 2>/dev/null || true
	@$(CONTAINER_CLI) rm -f $(OTEL_COLLECTOR_CONTAINER) 2>/dev/null || true
	@echo ""
	@echo "=========================================="
	@echo "  Live-check validation complete!"
	@echo "=========================================="
	@echo ""

##@ Tooling
weaver: $(WEAVER) ## Print local Weaver CLI version.
	@$(WEAVER) --version

$(WEAVER):
	@mkdir -p $(TOOLS_DIR)
	@OS=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
	ARCH=$$(uname -m); case $$ARCH in x86_64) ARCH="x86_64";; aarch64|arm64) ARCH="aarch64";; esac; \
	case $$OS in darwin) TARGET="$$ARCH-apple-darwin";; linux) TARGET="$$ARCH-unknown-linux-gnu";; esac; \
	curl -fsSL "https://github.com/open-telemetry/weaver/releases/download/v$(WEAVER_VERSION)/weaver-$$TARGET.tar.xz" | tar -xJ --strip-components=1 -C $(TOOLS_DIR)

vendor-otel-templates: ## Vendor OTel Go registry templates from upstream.
	@TMP=$$(mktemp -d) && \
	git clone --depth 1 --filter=blob:none --sparse --branch $(OTEL_GO_REF) $(OTEL_GO_REPO) $$TMP/otel && \
	git -C $$TMP/otel sparse-checkout set semconv/templates/registry/go && \
	rm -rf $(OTEL_GO_VENDOR_DIR) && mkdir -p $(OTEL_GO_VENDOR_DIR)/semconv/templates/registry && \
	cp -R $$TMP/otel/semconv/templates/registry/go $(OTEL_GO_VENDOR_DIR)/semconv/templates/registry/go && \
	rm -rf $$TMP

##@ Help
help: ## Show this help.
	@awk 'BEGIN {FS = ":.*##"; print "Usage: make <target>\n"} \
	/^[a-zA-Z0-9_.-]+:.*##/ {printf "  %-22s %s\n", $$1, $$2} \
	/^##@/ {printf "\n%s\n", substr($$0, 5)}' $(MAKEFILE_LIST)
