.PHONY: build test test-integration lint fmt clean run-api run-worker infra infra-down smoke docker-build docker-up docker-down tidy generate

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

# Local dev: broker URL for host-run processes. 127.0.0.1 (not localhost)
# avoids resolving to IPv6 ::1, which Docker's published port doesn't bind.
RABBITMQ_URL ?= amqp://guest:guest@127.0.0.1:5672/

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

# Run unit tests
test:
	$(GOTEST) -race -cover ./...

# Run integration tests (build tag `integration`; requires Docker for testcontainers)
test-integration:
	$(GOTEST) -tags integration -race -count=1 ./...

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

# Start only RabbitMQ (the common loop: broker in Docker, app on host)
infra:
	docker compose up -d rabbitmq

# Stop the local stack
infra-down:
	docker compose down

# Run API server locally (needs `make infra` first)
run-api:
	RABBITMQ_URL=$(RABBITMQ_URL) $(GOCMD) run ./cmd/api

# Run worker locally (needs `make infra` first)
run-worker:
	RABBITMQ_URL=$(RABBITMQ_URL) $(GOCMD) run ./cmd/worker

# Post a sample notification to a locally running API
smoke:
	@curl -s -X POST localhost:8080/api/v1/notifications \
		-H 'Content-Type: application/json' \
		-d '{"type":"email","priority":"transactional","recipient":{"email":"anna.kowalska@example.com"},"template_id":"welcome","template_data":{"name":"Anna"}}'
	@echo

# Docker commands (full stack)
docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

# Development helpers
dev-deps:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run all checks (lint + test)
check: lint test
