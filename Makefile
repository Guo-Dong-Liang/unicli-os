# UniCLI OS — Build System
#
# Targets:
#   build          Build unicli and unicli-validate binaries
#   test           Run all tests
#   bench          Run benchmarks
#   lint           Run golangci-lint
#   proto          Generate Go code from protobuf definitions
#   docker-build   Build all example CLI Docker images
#   validate       Validate all example manifests against JSON Schema
#   clean          Remove build artifacts

GO ?= go
GOPATH ?= $(HOME)/go
GOBIN ?= $(GOPATH)/bin
PROTOC ?= protoc
PROTOC_GEN_GO ?= $(GOBIN)/protoc-gen-go

BIN_DIR := bin
BINARY_UNICLI := $(BIN_DIR)/unicli
BINARY_VALIDATE := $(BIN_DIR)/unicli-validate

.PHONY: all build test bench lint proto docker-build validate clean

all: proto validate build

# --- Build ---

build: $(BINARY_UNICLI) $(BINARY_VALIDATE)
	@echo "Build complete."

$(BINARY_UNICLI): $(shell find cmd/unicli/ pkg/ -name '*.go' 2>/dev/null)
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags "-X main.version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)" -o $(BINARY_UNICLI) ./cmd/unicli 2>/dev/null || true

$(BINARY_VALIDATE): $(shell find cmd/unicli-validate/ pkg/ -name '*.go')
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BINARY_VALIDATE) ./cmd/unicli-validate

build-linux:
	GOOS=linux GOARCH=arm64 $(GO) build -o $(BINARY_UNICLI)-linux-arm64 ./cmd/unicli

# --- Protobuf ---

proto: protos/cpl.proto
	@mkdir -p pkg/cpl/v1
	$(PROTOC) --go_out=pkg/cpl/v1 --go_opt=paths=source_relative \
		--go_opt=Mprotos/cpl.proto=cpl/v1 \
		protos/cpl.proto

install-protoc-gen-go:
	$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@latest

# --- Test ---

test:
	$(GO) test -v -race -count=1 ./...

test-short:
	$(GO) test -short -count=1 ./...

test-coverage:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

# --- Lint ---

lint:
	@which golangci-lint >/dev/null 2>&1 || (echo "Installing golangci-lint..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
		| sh -s -- -b $(GOBIN) v1.59.1)
	golangci-lint run ./...

# --- Benchmarks ---

bench:
	$(GO) test -bench=. -benchmem -count=5 ./... > benchmarks/results.txt
	@echo "Benchmarks written to benchmarks/results.txt"

# --- Docker ---

docker-build-examples:
	@echo "Building example CLI images..."
	@for dir in examples/*; do \
		if [ -f "$$dir/Dockerfile" ]; then \
			name=$$(basename $$dir); \
			echo "Building $$name..."; \
			docker build -t "ghcr.io/unixcli/$$name:latest" "$$dir"; \
		fi; \
	done

# --- Validation ---

validate: $(BINARY_VALIDATE)
	@echo "Validating example manifests..."
	@for f in examples/*.cpl.json; do \
		echo "  Checking $$f..."; \
		$(BINARY_VALIDATE) manifest --file "$$f" || exit 1; \
	done

validate-all: $(BINARY_VALIDATE)
	@echo "Running all validations on example manifests..."
	@for f in examples/*.cpl.json; do \
		echo "--- $$f ---"; \
		$(BINARY_VALIDATE) all --file "$$f" || exit 1; \
		echo; \
	done

validate-schema:
	@echo "Validating against JSON Schema..."
	@which check-jsonschema >/dev/null 2>&1 || pip3 install check-jsonschema -q
	check-jsonschema --schemafile schemas/cpl-manifest.schema.json \
		examples/*.cpl.json

# --- Utilities ---

clean:
	rm -rf $(BIN_DIR) coverage.out coverage.html pkg/cpl/v1/*.go
	$(GO) clean -cache

fmt:
	$(GO) fmt ./...

tidy:
	$(GO) mod tidy

# --- Help ---

help:
	@echo "UniCLI OS Build System"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Available targets:"
	@echo "  build               Build both binaries"
	@echo "  test                Run all tests"
	@echo "  bench               Run benchmarks"
	@echo "  proto               Generate protobuf Go code"
	@echo "  lint                Run golangci-lint"
	@echo "  validate            Validate all example manifests"
	@echo "  clean               Clean build artifacts"
