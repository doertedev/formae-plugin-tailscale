# Formae Plugin Makefile
#
# Targets:
#   build   - Build the plugin binary
#   test    - Run tests
#   lint    - Run linter
#   clean   - Remove build artifacts
#   install - Build and install plugin locally (binary + schema + manifest)

# Plugin metadata - extracted from formae-plugin.pkl
PLUGIN_NAME := $(shell pkl eval -x 'name' formae-plugin.pkl 2>/dev/null || echo "example")
PLUGIN_VERSION := $(shell pkl eval -x 'version' formae-plugin.pkl 2>/dev/null || echo "0.0.0")
PLUGIN_NAMESPACE := $(shell pkl eval -x 'namespace' formae-plugin.pkl 2>/dev/null || echo "EXAMPLE")

# Build settings
GO := go
GOFLAGS := -trimpath
BINARY := $(PLUGIN_NAME)

# Installation paths
# Plugin discovery expects lowercase directory names matching the plugin name
PLUGIN_BASE_DIR := $(HOME)/.pel/formae/plugins
INSTALL_DIR := $(PLUGIN_BASE_DIR)/$(PLUGIN_NAME)/v$(PLUGIN_VERSION)

.PHONY: all build test test-unit test-integration test-live-tailscale setup-credentials lint verify-schema clean install help clean-environment conformance-cleanup conformance-test conformance-test-crud conformance-test-discovery

all: build

## build: Build the plugin binary and update manifest
build:
	@mkdir -p schema/pkl && echo "$(PLUGIN_VERSION)" > schema/pkl/VERSION
	$(GO) build $(GOFLAGS) -o bin/$(BINARY) ./pkg
	@SDK_MIN=$$($(GO) list -m -f '{{.Dir}}' github.com/platform-engineering-labs/formae/pkg/plugin 2>/dev/null | xargs -I{} grep 'MinFormaeVersion' {}/version.go 2>/dev/null | grep -oE '"[0-9]+\.[0-9]+\.[0-9]+"' | tr -d '"'); \
	DECLARED=$$(pkl eval -x minFormaeVersion formae-plugin.pkl 2>/dev/null); \
	EFFECTIVE=$$(printf '%s\n%s\n' "$$SDK_MIN" "$$DECLARED" | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$$' | sort -t. -k1,1n -k2,2n -k3,3n | tail -1); \
	if [ -n "$$EFFECTIVE" ] && [ "$$EFFECTIVE" != "$$DECLARED" ]; then \
		echo "Raising minFormaeVersion to $$EFFECTIVE (sdk=$$SDK_MIN, declared=$$DECLARED)"; \
		if [ "$$(uname)" = "Darwin" ]; then \
			sed -i '' 's/^minFormaeVersion = .*/minFormaeVersion = "'"$$EFFECTIVE"'"/' formae-plugin.pkl; \
		else \
			sed -i 's/^minFormaeVersion = .*/minFormaeVersion = "'"$$EFFECTIVE"'"/' formae-plugin.pkl; \
		fi; \
	else \
		echo "Keeping declared minFormaeVersion=$$DECLARED (sdk=$$SDK_MIN, never downgrade below declared)"; \
	fi

## test: Run all tests
test:
	$(GO) test -v ./...

## test-unit: Run unit tests
test-unit:
	$(GO) test -v -tags=unit ./...

## test-live-tailscale: Run direct live Tailscale API smoke tests
## Loads .env, requires credentials, and sets TAILSCALE_INTEGRATION=1.
test-live-tailscale:
	@if [ -f .env ]; then set -a; . ./.env; set +a; fi; \
	if [ -z "$$TAILSCALE_API_KEY" ] && { [ -z "$$TAILSCALE_OAUTH_CLIENT_ID" ] || [ -z "$$TAILSCALE_OAUTH_CLIENT_SECRET" ]; }; then \
		echo "ERROR: set TAILSCALE_API_KEY or TAILSCALE_OAUTH_CLIENT_ID/TAILSCALE_OAUTH_CLIENT_SECRET."; \
		echo "Use a dedicated Tailscale test tailnet or test-scoped credentials."; \
		exit 1; \
	fi; \
	TAILSCALE_INTEGRATION=1 TAILSCALE_PLUGIN_DEBUG="$${TAILSCALE_PLUGIN_DEBUG:-1}" \
		$(GO) test -v -tags=integration -run '^TestIntegration_' -count=1 -timeout $(or $(TIMEOUT),10m) ./...

## test-integration: Alias for test-live-tailscale
test-integration: test-live-tailscale

## lint: Run golangci-lint
lint:
	golangci-lint run

## verify-schema: Validate PKL schema files
## Checks that schema files are well-formed and follow formae conventions.
verify-schema:
	$(GO) run github.com/platform-engineering-labs/formae/pkg/plugin/testutil/cmd/verify-schema --namespace $(PLUGIN_NAMESPACE) ./schema/pkl

## clean: Remove build artifacts
clean:
	rm -rf bin/ dist/

## install: Build and install plugin locally (binary + schema + manifest)
## Installs to ~/.pel/formae/plugins/<name>/v<version>/
## Removes any existing versions of the plugin first to ensure clean state.
install: build
	@echo "Installing $(PLUGIN_NAME) v$(PLUGIN_VERSION) (namespace: $(PLUGIN_NAMESPACE))..."
	@rm -rf $(PLUGIN_BASE_DIR)/$(PLUGIN_NAME)
	@mkdir -p $(INSTALL_DIR)/schema
	@cp bin/$(BINARY) $(INSTALL_DIR)/$(BINARY)
	@cp -r schema/* $(INSTALL_DIR)/schema/
	@cp formae-plugin.pkl $(INSTALL_DIR)/
	@echo "Installed to $(INSTALL_DIR)"
	@echo "  - Binary: $(INSTALL_DIR)/$(BINARY)"
	@echo "  - Schema: $(INSTALL_DIR)/schema/"
	@echo "  - Manifest: $(INSTALL_DIR)/formae-plugin.pkl"

## help: Show this help message
help:
	@echo "Available targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'

## clean-environment: Clean up Tailscale test resources
clean-environment:
	@./scripts/ci/clean-environment.sh

## conformance-cleanup: Alias for clean-environment
conformance-cleanup: clean-environment

## setup-credentials: Verify Tailscale credentials before live/conformance tests
setup-credentials:
	@if [ -f .env ]; then set -a; . ./.env; set +a; fi; \
	if [ -z "$$TAILSCALE_API_KEY" ] && { [ -z "$$TAILSCALE_OAUTH_CLIENT_ID" ] || [ -z "$$TAILSCALE_OAUTH_CLIENT_SECRET" ]; }; then \
		echo "ERROR: set TAILSCALE_API_KEY or TAILSCALE_OAUTH_CLIENT_ID/TAILSCALE_OAUTH_CLIENT_SECRET."; \
		echo "Conformance tests hit the real Tailscale API."; \
		exit 1; \
	fi

## conformance-test: Run all conformance tests (CRUD + discovery)
## Usage: make conformance-test [TEST=auth-key] [TIMEOUT=30m]
## Calls clean-environment before and after tests.
conformance-test: conformance-test-crud conformance-test-discovery

## conformance-test-crud: Run only CRUD lifecycle tests
## Usage: make conformance-test-crud [TEST=auth-key] [TIMEOUT=30m]
conformance-test-crud: install setup-credentials
	@echo "Pre-test cleanup..."
	@./scripts/ci/clean-environment.sh || true
	@echo ""
	@echo "Running CRUD conformance tests..."
	@if [ -f .env ]; then set -a; . ./.env; set +a; fi; \
	FORMAE_TEST_FILTER="$(TEST)" FORMAE_TEST_TYPE=crud TAILSCALE_PLUGIN_DEBUG="$${TAILSCALE_PLUGIN_DEBUG:-1}" \
		$(GO) test -tags=conformance -run '^TestPluginConformance$$' -v -timeout $(or $(TIMEOUT),30m) ./...; \
	TEST_EXIT=$$?; \
	echo ""; \
	echo "Post-test cleanup..."; \
	./scripts/ci/clean-environment.sh || true; \
	exit $$TEST_EXIT

## conformance-test-discovery: Run only discovery tests
## Usage: make conformance-test-discovery [TEST=auth-key] [TIMEOUT=30m]
conformance-test-discovery: install setup-credentials
	@echo "Pre-test cleanup..."
	@./scripts/ci/clean-environment.sh || true
	@echo ""
	@echo "Running discovery conformance tests..."
	@if [ -f .env ]; then set -a; . ./.env; set +a; fi; \
	FORMAE_TEST_FILTER="$(TEST)" FORMAE_TEST_TYPE=discovery TAILSCALE_PLUGIN_DEBUG="$${TAILSCALE_PLUGIN_DEBUG:-1}" \
		$(GO) test -tags=conformance -run '^TestPluginDiscovery$$' -v -timeout $(or $(TIMEOUT),30m) ./...; \
	TEST_EXIT=$$?; \
	echo ""; \
	echo "Post-test cleanup..."; \
	./scripts/ci/clean-environment.sh || true; \
	exit $$TEST_EXIT
