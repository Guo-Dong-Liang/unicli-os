.PHONY: build test clean lint bench docker-build

# Binary
BINARY=unicli
OUTPUT_DIR=build/

# Go
GO=go
GOPATH=$(shell $(GO) env GOPATH)
GOFLAGS=-ldflags="-s -w"

# Docker
DOCKER=docker
REGISTRY=ghcr.io/unicli

# Version (from git tag or default)
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

## Build

build: ## Build unicli binary
	@mkdir -p $(OUTPUT_DIR)
	$(GO) build $(GOFLAGS) -o $(OUTPUT_DIR)$(BINARY) -ldflags="-X main.Version=$(VERSION) -X main.Commit=$(COMMIT)" ./cmd/unicli

build-all: ## Build for multiple platforms
	@mkdir -p $(OUTPUT_DIR)
	GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(OUTPUT_DIR)$(BINARY)-linux-amd64 ./cmd/unicli
	GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) -o $(OUTPUT_DIR)$(BINARY)-darwin-arm64 ./cmd/unicli
	GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(OUTPUT_DIR)$(BINARY)-darwin-amd64 ./cmd/unicli

## Test

test: ## Run all tests
	$(GO) test ./... -v -count=1

test-short: ## Run short tests only
	$(GO) test ./... -short -count=1

test-race: ## Run tests with race detector
	$(GO) test ./... -race -count=1

cover: ## Run tests with coverage
	$(GO) test ./... -coverprofile=coverage.out -count=1
	$(GO) tool cover -html=coverage.out -o coverage.html

## Lint

lint: ## Run linters
	$(GO) vet ./...
	@which golangci-lint > /dev/null 2>&1 && golangci-lint run || echo "golangci-lint not installed, skipping"

## Bench

bench: ## Run benchmarks
	$(GO) test ./benchmarks/... -bench=. -benchmem -count=3

## Proto

proto: ## Generate protobuf code
	protoc --go_out=. --go_opt=paths=source_relative \
		--go_opt=Mprotos/cpl.proto=github.com/admin/unicli-os/pkg/cpl/v1 \
		protos/cpl.proto

## Docker

docker-build: ## Build example CLI images
	$(DOCKER) build -t $(REGISTRY)/hello.say:1.0.0 -f examples/hello.say/Dockerfile .
	$(DOCKER) build -t $(REGISTRY)/image.resize:1.0.0 -f examples/image.resize/Dockerfile .

## Clean

clean: ## Clean build artifacts
	rm -rf $(OUTPUT_DIR)
	rm -f coverage.out coverage.html
	rm -rf tmp/

## Help

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
