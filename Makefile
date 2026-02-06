.PHONY: all build run test clean deps fmt vet lint docker help

APP_NAME := packetsorter
BUILD_DIR := bin
MAIN_FILE := cmd/packetsorter/main.go
DB_FILE := parcels.db

# Default target
all: build

## Build the application
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_FILE)
	@echo "Build complete: ./$(BUILD_DIR)/$(APP_NAME)"

## Run the application
run: build
	@echo "Starting $(APP_NAME)..."
	./$(BUILD_DIR)/$(APP_NAME)

## Run with load generator
run-with-load: build
	@echo "Starting $(APP_NAME) and load generator..."
	./$(BUILD_DIR)/$(APP_NAME) &
	@sleep 2
	./scripts/loadtest.sh

## Run tests
test:
	@echo "Running tests..."
	go test -v ./...

## Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## Clean build artifacts and database
clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -f $(DB_FILE)
	rm -f coverage.out coverage.html
	@echo "Clean complete"

## Download and tidy dependencies
deps:
	@echo "Downloading dependencies..."
	go mod tidy
	go mod download

## Format Go code
fmt:
	@echo "Formatting code..."
	go fmt ./...

## Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...

## Run all linters (requires golangci-lint)
lint: fmt vet
	@echo "Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

## Build Docker image
docker:
	@echo "Building Docker image..."
	docker build -t $(APP_NAME):latest .

## Check for security vulnerabilities (requires govulncheck)
security:
	@echo "Checking for vulnerabilities..."
	@if command -v govulncheck >/dev/null 2>&1; then \
		govulncheck ./...; \
	else \
		echo "govulncheck not installed. Run: go install golang.org/x/vuln/cmd/govulncheck@latest"; \
	fi

## Install development tools
install-tools:
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest

## Show metrics endpoint
metrics:
	@echo "Fetching metrics from localhost:8080..."
	@curl -s http://localhost:8080/metrics | grep -E "^parcels_|^stage_duration_|^parcels_backlog_|^rework_reason_"

## Show help
help:
	@echo "Available targets:"
	@echo ""
	@echo "  make build          - Build the application"
	@echo "  make run            - Build and run the application"
	@echo "  make run-with-load  - Run with load generator"
	@echo "  make test           - Run tests"
	@echo "  make test-coverage  - Run tests with coverage report"
	@echo "  make clean          - Remove build artifacts and database"
	@echo "  make deps           - Download and tidy Go dependencies"
	@echo "  make fmt            - Format Go code"
	@echo "  make vet            - Run go vet"
	@echo "  make lint           - Run all linters"
	@echo "  make docker         - Build Docker image"
	@echo "  make security       - Check for security vulnerabilities"
	@echo "  make install-tools  - Install development tools"
	@echo "  make metrics        - Show current metrics"
	@echo "  make help           - Show this help message"