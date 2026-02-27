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
COLLECTOR_CLEANUP := $(CONTAINER_CLI) rm -f $(OTEL_COLLECTOR_CONTAINER) 2>/dev/null || true

WEAVER_IMAGE ?= otel/weaver:main
WEAVER_CONTAINER ?= weaver-infer
WEAVER_INFER_PORT ?= 4319
WEAVER_ADMIN_PORT ?= 4320
WEAVER_INACTIVITY_TIMEOUT ?= 300
WEAVER_OUTPUT_DIR ?= inferred-registry
WEAVER_STOP_CMD := curl -s -X POST http://localhost:$(WEAVER_ADMIN_PORT)/stop 2>/dev/null || true
WEAVER_CLEANUP := $(CONTAINER_CLI) rm -f $(WEAVER_CONTAINER) 2>/dev/null || true

WEAVER := $(TOOLS_DIR)/weaver
OTEL_GO_REPO := https://github.com/open-telemetry/opentelemetry-go.git
OTEL_GO_REF ?= main
OTEL_GO_VENDOR_DIR := templates/vendor/otel-go

.PHONY: all build image clean \
	run run-otel run-otel-weaver metrics \
	start-collector stop-collector start-weaver-infer stop-weaver-infer weaver-infer-logs \
	test test-e2e test-e2e-weaver \
	lint tidy \
	kind-up kind-down kind-test e2e \
	weaver vendor-otel-templates \
	help

##@ Build
all: build ## Build the application binary.

build: ## Compile the application to bin/.
	@mkdir -p $(BUILD_DIR)
	go build -o $(APP_BIN) $(MAIN_FILE)

image: ## Build the local container image.
	$(CONTAINER_CLI) build -t $(IMAGE_NAME):$(IMAGE_TAG) .

clean: ## Remove generated build/test artifacts.
	rm -rf $(BUILD_DIR) $(DB_FILE) tmp/ inferred-registry/
	rm -f coverage.out coverage.html

##@ Run
run: build ## Run the application locally.
	./$(APP_BIN)

run-otel: build start-collector ## Run app with the local OTel collector.
	@echo "App:            http://localhost:8080"
	@echo "Collector:      http://localhost:8888/metrics"
	@echo ""
	@trap '$(COLLECTOR_CLEANUP)' EXIT; ./$(APP_BIN)

run-otel-weaver: build start-weaver-infer start-collector ## Run app with collector and Weaver infer.
	@echo "App:            http://localhost:8080"
	@echo "Collector:      http://localhost:8888/metrics"
	@echo "Weaver Admin:   http://localhost:$(WEAVER_ADMIN_PORT)/stop"
	@echo "Schema Output:  $(WEAVER_OUTPUT_DIR)/"
	@echo "Weaver Logs:    make weaver-infer-logs"
	@echo ""
	@trap '$(WEAVER_STOP_CMD); $(WEAVER_CLEANUP); $(COLLECTOR_CLEANUP)' EXIT; ./$(APP_BIN)

metrics: ## Print metrics from a running local app.
	@curl -fsS http://localhost:8080/metrics

##@ Observability Helpers
start-collector: ## Start local OpenTelemetry Collector container.
	@$(COLLECTOR_CLEANUP)
	@$(CONTAINER_CLI) run -d --name $(OTEL_COLLECTOR_CONTAINER) \
		-p 4317:4317 -p 4318:4318 -p 8888:8888 -p 13133:13133 \
		-v $(PWD)/$(OTEL_COLLECTOR_CONFIG):/conf/otel-collector-config.yaml:ro \
		$(OTEL_COLLECTOR_IMAGE) --config=/conf/otel-collector-config.yaml >/dev/null
	@echo "OTel Collector started"

stop-collector: ## Stop local OpenTelemetry Collector container.
	@$(COLLECTOR_CLEANUP)

start-weaver-infer: ## Start Weaver infer and write schema to inferred-registry/.
	@mkdir -p $(WEAVER_OUTPUT_DIR)
	@$(WEAVER_CLEANUP)
	@$(CONTAINER_CLI) run -d --name $(WEAVER_CONTAINER) \
		-p $(WEAVER_INFER_PORT):4317 \
		-p $(WEAVER_ADMIN_PORT):8080 \
		-v $(PWD)/$(WEAVER_OUTPUT_DIR):/output \
		$(WEAVER_IMAGE) registry infer \
			--output /output \
			--inactivity-timeout $(WEAVER_INACTIVITY_TIMEOUT) >/dev/null
	@echo "Weaver infer started on port $(WEAVER_INFER_PORT)"
	@echo "Schema output: $(WEAVER_OUTPUT_DIR)/"

stop-weaver-infer: ## Stop Weaver infer and flush schema output.
	@$(WEAVER_STOP_CMD)
	@$(WEAVER_CLEANUP)

weaver-infer-logs: ## Follow logs from the Weaver infer container.
	@$(CONTAINER_CLI) logs -f $(WEAVER_CONTAINER)

##@ Test
test: ## Run unit tests.
	go test -race -short ./...

test-e2e: build start-collector ## Run local e2e tests with collector.
	@./$(APP_BIN) & APP_PID=$$!; \
	trap 'kill $$APP_PID 2>/dev/null; $(COLLECTOR_CLEANUP)' EXIT; \
	sleep 2; \
	$(E2E_TEST_CMD)

test-e2e-weaver: build start-weaver-infer start-collector ## Run e2e tests and infer schema with Weaver.
	@echo "Running e2e tests with Weaver infer..."
	@./$(APP_BIN) & APP_PID=$$!; \
	trap 'kill $$APP_PID 2>/dev/null; $(WEAVER_STOP_CMD); $(WEAVER_CLEANUP); $(COLLECTOR_CLEANUP)' EXIT; \
	sleep 2; \
	$(E2E_TEST_CMD); \
	echo ""; \
	echo "Waiting for OTel Collector to scrape and forward metrics..."; \
	sleep 10; \
	echo "Stopping Weaver to flush schema..."; \
	$(WEAVER_STOP_CMD); \
	sleep 2; \
	echo ""; \
	echo "Inferred schema written to $(WEAVER_OUTPUT_DIR)/"

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
