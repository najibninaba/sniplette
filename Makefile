# Makefile for ig2wa
# Provides common developer tasks with helpful defaults and cross-platform builds.

# Default goal shows help
.DEFAULT_GOAL := help

# Variables
BIN_NAME ?= ig2wa
MODULE   ?= ig2wa
CMD_DIR  ?= ./cmd/ig2wa
OUT_DIR  ?= ./bin

GO       ?= go
GOFLAGS  ?=
BUILD_FLAGS ?=
CGO_ENABLED ?= 0

# Version information from git (if available)
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.0.0")
COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE       := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GOVERSION  := $(shell $(GO) version)

# Internal helpers
MKDIR_P = mkdir -p

# Help target: lists available targets with descriptions.
# To add descriptions, append '## <desc>' to the target line.
help: ## Show all available targets with descriptions
	@echo "ig2wa Makefile — helpful targets"
	@echo
	@echo "Variables:"
	@echo "  BIN_NAME=$(BIN_NAME)  OUT_DIR=$(OUT_DIR)  GOFLAGS='$(GOFLAGS)'  BUILD_FLAGS='$(BUILD_FLAGS)'  CGO_ENABLED=$(CGO_ENABLED)"
	@echo
	@echo "Targets:"
	@awk 'BEGIN { FS = ":.*##" } /^[a-zA-Z0-9_.-]+:.*##/ { printf "  %-18s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

# Build the binary for the current platform
build: ## Build the binary for the current platform
	@$(MKDIR_P) $(OUT_DIR)
	$(GO) build $(GOFLAGS) $(BUILD_FLAGS) -o $(OUT_DIR)/$(BIN_NAME) $(CMD_DIR)

# Install to $GOBIN
install: ## Install the binary to $$GOBIN
	$(GO) install $(GOFLAGS) $(BUILD_FLAGS) $(CMD_DIR)

# Clean build artifacts
clean: ## Remove build artifacts
	@echo "Cleaning $(OUT_DIR) and temporary artifacts"
	@rm -rf $(OUT_DIR) ./dist ./build ./out ./tmp ./cover ./coverage coverage.txt

# Run tests
test: ## Run tests
	$(GO) test $(GOFLAGS) ./...

# Format code
fmt: ## Format all Go code
	$(GO) fmt ./...

# Run go vet
vet: ## Run go vet
	$(GO) vet ./...

# Lint with golangci-lint if available, otherwise fallback to go vet
lint: ## Run golangci-lint (if available), otherwise go vet
	@if command -v golangci-lint >/dev/null 2>&1; then \
		echo "Running golangci-lint"; \
		golangci-lint run; \
	else \
		echo "golangci-lint not found; falling back to 'go vet'"; \
		$(GO) vet ./...; \
	fi

# Run the tool with ARGS support: make run ARGS="--help"
run: ## Run the tool with ARGS, e.g., 'make run ARGS="--help"'
	$(GO) run $(GOFLAGS) $(BUILD_FLAGS) $(CMD_DIR) $(ARGS)

# Development build with race detector
dev: ## Development build with race detector
	@$(MKDIR_P) $(OUT_DIR)
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build -race $(GOFLAGS) $(BUILD_FLAGS) -o $(OUT_DIR)/$(BIN_NAME) $(CMD_DIR)

# Cross-compile for multiple platforms (darwin/linux, amd64/arm64)
cross-compile: ## Build for macOS and Linux (amd64 and arm64)
	@$(MKDIR_P) $(OUT_DIR)
	@for os in darwin linux; do \
		for arch in amd64 arm64; do \
			out="$(OUT_DIR)/$(BIN_NAME)-$${os}-$${arch}"; \
			echo "Building $$out"; \
			GOOS=$${os} GOARCH=$${arch} CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GOFLAGS) $(BUILD_FLAGS) -o "$$out" $(CMD_DIR); \
		done; \
	done

# Run fmt, vet, and test together
check: fmt vet test ## Run fmt, vet, and tests (quick sanity)

# Show version info from git and Go toolchain
version: ## Show version information
	@echo "Module:     $(MODULE)"
	@echo "Version:    $(VERSION)"
	@echo "Commit:     $(COMMIT)"
	@echo "Date:       $(DATE)"
	@echo "Go:         $(GOVERSION)"
	@echo "Bin Name:   $(BIN_NAME)"
	@echo "Out Dir:    $(OUT_DIR)"

.PHONY: help build install clean test fmt vet lint run dev cross-compile check version