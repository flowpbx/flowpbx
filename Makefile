# FlowPBX Makefile
# Single-binary PBX with visual call flow editor

BINARY_NAME := flowpbx
PUSHGW_NAME := pushgw
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT      := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME  := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS     := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.buildTime=$(BUILD_TIME)

GO       := go
GOFLAGS  := -trimpath
LINT     := golangci-lint

# Output directories
BUILD_DIR  := build
WEB_DIR    := web
WEB_DIST   := $(WEB_DIR)/dist
MOBILE_DIR := mobile

.PHONY: build dev test lint ui-build mobile-deps mobile-build mobile-test mobile-lint mobile-clean release clean help

## build: Compile flowpbx and pushgw binaries
build: ui-build
	$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/flowpbx
	$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BUILD_DIR)/$(PUSHGW_NAME) ./cmd/pushgw

## dev: Run flowpbx in development mode with race detector
dev:
	$(GO) run -race ./cmd/flowpbx --log-level debug

## test: Run all tests with race detector
test:
	$(GO) test -race -count=1 ./...

## lint: Run golangci-lint and go vet
lint:
	$(GO) vet ./...
	@if command -v $(LINT) >/dev/null 2>&1; then \
		$(LINT) run ./...; \
	else \
		echo "golangci-lint not installed, skipping (install: https://golangci-lint.run/usage/install/)"; \
	fi

## ui-build: Build React admin UI (requires Node.js)
ui-build:
	@if [ -d "$(WEB_DIR)" ] && [ -f "$(WEB_DIR)/package.json" ]; then \
		cd $(WEB_DIR) && npm ci && npm run build; \
	else \
		echo "web/ not yet scaffolded, skipping UI build"; \
		mkdir -p internal/web/dist; \
	fi

## mobile-deps: Install Flutter dependencies
mobile-deps:
	@if [ -d "$(MOBILE_DIR)" ] && [ -f "$(MOBILE_DIR)/pubspec.yaml" ]; then \
		cd $(MOBILE_DIR) && flutter pub get; \
	else \
		echo "mobile/ not yet scaffolded, skipping"; \
	fi

## mobile-build: Build Flutter app (APK + iOS archive)
mobile-build: mobile-deps
	@if [ -d "$(MOBILE_DIR)" ] && [ -f "$(MOBILE_DIR)/pubspec.yaml" ]; then \
		cd $(MOBILE_DIR) && flutter build apk --release; \
		echo "Android APK built successfully"; \
		if [ "$$(uname)" = "Darwin" ]; then \
			cd $(MOBILE_DIR) && flutter build ios --release --no-codesign; \
			echo "iOS build completed (no-codesign)"; \
		else \
			echo "Skipping iOS build (requires macOS)"; \
		fi \
	else \
		echo "mobile/ not yet scaffolded, skipping"; \
	fi

## mobile-test: Run Flutter tests
mobile-test:
	@if [ -d "$(MOBILE_DIR)" ] && [ -f "$(MOBILE_DIR)/pubspec.yaml" ]; then \
		cd $(MOBILE_DIR) && flutter test; \
	else \
		echo "mobile/ not yet scaffolded, skipping"; \
	fi

## mobile-lint: Run Flutter analyzer
mobile-lint:
	@if [ -d "$(MOBILE_DIR)" ] && [ -f "$(MOBILE_DIR)/pubspec.yaml" ]; then \
		cd $(MOBILE_DIR) && flutter analyze; \
	else \
		echo "mobile/ not yet scaffolded, skipping"; \
	fi

## mobile-clean: Clean Flutter build artifacts
mobile-clean:
	@if [ -d "$(MOBILE_DIR)" ]; then \
		cd $(MOBILE_DIR) && flutter clean; \
	fi

## release: Cross-compile release binaries for linux/amd64 and linux/arm64
release: ui-build
	GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/flowpbx
	GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/flowpbx
	GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BUILD_DIR)/$(PUSHGW_NAME)-linux-amd64 ./cmd/pushgw
	GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BUILD_DIR)/$(PUSHGW_NAME)-linux-arm64 ./cmd/pushgw

## clean: Remove build artifacts
clean:
	rm -rf $(BUILD_DIR)
	@if [ -d "$(MOBILE_DIR)" ]; then \
		cd $(MOBILE_DIR) && flutter clean 2>/dev/null || true; \
	fi

## help: Show this help message
help:
	@echo "FlowPBX build targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
