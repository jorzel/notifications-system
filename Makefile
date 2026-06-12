.PHONY: build test lint fmt clean run-api run-worker docker-build docker-up docker-down tidy generate

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOFMT=gofmt
GOMOD=$(GOCMD) mod
GOLINT=golangci-lint

# Binary names
API_BINARY=api
WORKER_BINARY=worker

# Build directories
BUILD_DIR=bin

# Build the API server
build-api:
	$(GOBUILD) -o $(BUILD_DIR)/$(API_BINARY) ./cmd/api

# Build the worker
build-worker:
	$(GOBUILD) -o $(BUILD_DIR)/$(WORKER_BINARY) ./cmd/worker

# Build all binaries
build: build-api build-worker

# Run tests
test:
	$(GOTEST) -v -race -cover ./...

# Run tests with coverage report
test-coverage:
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Run linter
lint:
	$(GOLINT) run ./...

# Format code
fmt:
	$(GOFMT) -s -w .

# Tidy dependencies
tidy:
	$(GOMOD) tidy

# Generate code (mocks via uber-go/mock; oapi-codegen server from the OpenAPI spec)
generate:
	$(GOCMD) generate ./...

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Run API server locally
run-api:
	$(GOCMD) run ./cmd/api

# Run worker locally
run-worker:
	$(GOCMD) run ./cmd/worker

# Docker commands
docker-build:
	docker-compose build

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

# Development helpers
dev-deps:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run all checks (lint + test)
check: lint test
