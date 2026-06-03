# =============================================================================
# Retrowin Makefile
# =============================================================================

# Default shell
.SHELLFLAGS = -e -c

# ---------------------------------------------------------------------------
# Variables
# ---------------------------------------------------------------------------
APP_NAME    := retrowin-server
BUILD_DIR   := bin
MIGRATION_DIR = ent/migrate/migrations

# Test packages to exclude from unit tests
TEST_FILTER := grep -v -e /mocks -e /test/e2e -e /test/integration -e /test/kind
UNIT_PKGS   := $(shell go list ./... | $(TEST_FILTER))

# ---------------------------------------------------------------------------
# Code Generation
# ---------------------------------------------------------------------------
.PHONY: gen
gen: ent-gen ogen mock ## Generate all code (ent, ogen, mocks)

.PHONY: ent-gen
ent-gen: ## Generate ent code from schema
	go run -mod=mod entgo.io/ent/cmd/ent generate ./ent/schema

.PHONY: openapi
openapi: ## Bundle and validate OpenAPI spec
	npx @apidevtools/swagger-cli bundle api/openapi.yaml --outfile api/openapi.bundled.json --type json
	npx @apidevtools/swagger-cli validate api/openapi.bundled.json

.PHONY: ogen
ogen: openapi ## Generate API server code from OpenAPI spec
	@rm -f pkg/api/oas_*.go
	go tool ogen -config ogen.yaml -target ./pkg/api -package api api/openapi.bundled.json

.PHONY: mock
mock: ## Generate mocks with mockery
	@find ./internal -type d -name "mocks" -exec rm -rf {} + 2>/dev/null || true
	mockery

# ---------------------------------------------------------------------------
# Lint & Security
# ---------------------------------------------------------------------------
.PHONY: lint
lint: ## Run golangci-lint
	@which golangci-lint > /dev/null || go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	golangci-lint run

.PHONY: lint-fix
lint-fix: ## Run golangci-lint with auto-fix
	@which golangci-lint > /dev/null || go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	golangci-lint fmt
	golangci-lint run --fix

.PHONY: sec
sec: ## Run gosec security scanner
	@which gosec > /dev/null || go install github.com/securego/gosec/v2/cmd/gosec@latest
	gosec ./...

# ---------------------------------------------------------------------------
# Testing
# ---------------------------------------------------------------------------
.PHONY: test
test: ## Run unit tests only
	go test -v $(UNIT_PKGS) -coverprofile=cover-unit.out

.PHONY: test-e2e
test-e2e: ## Run e2e tests (requires Docker)
	go test -v ./test/e2e/... -timeout 10m -coverprofile=cover-e2e.out

.PHONY: test-integration
test-integration: ## Run integration tests (requires Docker)
	go test -v ./test/integration/... -tags integration -timeout 5m -coverprofile=cover-integration.out

.PHONY: test-kind
test-kind: ## Run kind tests (requires kind, kubectl, helm, docker)
	go test -v ./test/kind/... -tags kind -timeout 15m

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------
.PHONY: build
build: ## Build server binary
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/retrowin-server

.PHONY: run
run: ## Run server in development mode
	go run ./cmd/retrowin-server serve --config config.yaml

# ---------------------------------------------------------------------------
# Database Migrations
# ---------------------------------------------------------------------------
.PHONY: migrate-diff
migrate-diff: ## Generate migration diff. Usage: make migrate-diff name=add_column
	atlas migrate diff $(name) \
		--dir "file://$(MIGRATION_DIR)" \
		--to "ent://ent/schema" \
		--dev-url "docker://postgres/17/dev?search_path=public"

.PHONY: migrate-apply
migrate-apply: ## Apply pending migrations
	go run ./cmd/retrowin-server migrate apply --config config.yaml

.PHONY: migrate-status
migrate-status: ## Show migration status
	atlas migrate status --dir "file://$(MIGRATION_DIR)"

.PHONY: migrate-lint
migrate-lint: ## Lint migration files for safety
	atlas migrate lint --dir "file://$(MIGRATION_DIR)" \
		--dev-url "docker://postgres/17/dev?search_path=public"

# ---------------------------------------------------------------------------
# Pre-commit Hooks
# ---------------------------------------------------------------------------
.PHONY: pre-commit-install
pre-commit-install: ## Install pre-commit and pre-push hooks
	@which pre-commit > /dev/null || (pip install pre-commit)
	pre-commit install
	pre-commit install --hook-type pre-push
	@echo "Pre-commit hooks installed. Run 'make pre-commit-run' to test."

.PHONY: pre-commit-run
pre-commit-run: ## Run all pre-commit hooks on all files
	pre-commit run --all-files

.PHONY: pre-commit-update
pre-commit-update: ## Update pre-commit hooks to latest versions
	pre-commit autoupdate

# ---------------------------------------------------------------------------
# Cleanup
# ---------------------------------------------------------------------------
.PHONY: clean
clean: ## Clean build artifacts and generated files
	rm -rf $(BUILD_DIR)/
	rm -f api/openapi.bundled.json
	rm -f cover-*.out

# ---------------------------------------------------------------------------
# Help
# ---------------------------------------------------------------------------
.PHONY: help
help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
