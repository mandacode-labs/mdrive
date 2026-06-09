# =============================================================================
# Retrowin Makefile
# =============================================================================

.SHELLFLAGS = -e -c

# ---------------------------------------------------------------------------
# Variables
# ---------------------------------------------------------------------------
APP_NAME    := retrowin-server
BUILD_DIR   := bin
MIGRATION_DIR = ent/migrate/migrations

# ---------------------------------------------------------------------------
# Code Generation
# ---------------------------------------------------------------------------
.PHONY: gen
gen: ent-gen ogen mock

.PHONY: ent-gen
ent-gen:
	go run -mod=mod entgo.io/ent/cmd/ent generate ./ent/schema

.PHONY: openapi
openapi:
	npx @apidevtools/swagger-cli bundle api/openapi.yaml --outfile api/openapi.bundled.json --type json
	npx @apidevtools/swagger-cli validate api/openapi.bundled.json

.PHONY: ogen
ogen: openapi
	@rm -f pkg/api/oas_*.go
	go tool ogen -config ogen.yaml -target ./pkg/api -package api api/openapi.bundled.json

.PHONY: mock
mock:
	@find ./internal -type d -name "mocks" -exec rm -rf {} + 2>/dev/null || true
	mockery

# ---------------------------------------------------------------------------
# Lint
# ---------------------------------------------------------------------------
.PHONY: lint
lint:
	@which golangci-lint > /dev/null || go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	golangci-lint run

# ---------------------------------------------------------------------------
# Testing
# ---------------------------------------------------------------------------
UNIT_PKGS = $(shell go list ./... | grep -v -e /mocks -e /test/e2e -e /test/integration -e /test/kind)

.PHONY: test
test:
	go test -count=1 $(UNIT_PKGS)

.PHONY: test-all
test-all:
	go test -count=1 ./... -coverprofile=cover.out

.PHONY: test-e2e
test-e2e:
	go test -count=1 ./test/e2e/... -timeout 10m -coverprofile=cover-e2e.out

.PHONY: test-integration
test-integration:
	go test -count=1 ./test/integration/... -tags integration -timeout 5m -coverprofile=cover-integration.out

.PHONY: test-kind
test-kind:
	go test -count=1 ./test/kind/... -tags kind -timeout 30m

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------
.PHONY: build
build:
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/retrowin-server

.PHONY: run
run:
	go run ./cmd/retrowin-server serve --config config.yaml

# ---------------------------------------------------------------------------
# Database Migrations
# ---------------------------------------------------------------------------
.PHONY: migrate-diff
migrate-diff:
	atlas migrate diff $(name) \
		--dir "file://$(MIGRATION_DIR)" \
		--to "ent://ent/schema" \
		--dev-url "docker://postgres/17/dev?search_path=public"

.PHONY: migrate-apply
migrate-apply:
	go run ./cmd/retrowin-server migrate apply --config config.yaml

.PHONY: migrate-status
migrate-status:
	atlas migrate status --dir "file://$(MIGRATION_DIR)"

.PHONY: migrate-lint
migrate-lint:
	atlas migrate lint --dir "file://$(MIGRATION_DIR)" \
		--dev-url "docker://postgres/17/dev?search_path=public"

# ---------------------------------------------------------------------------
# Hooks
# ---------------------------------------------------------------------------
.PHONY: hooks
hooks:
	@which lefthook > /dev/null || go install github.com/evilmartians/lefthook@latest
	lefthook install

# ---------------------------------------------------------------------------
# Cleanup
# ---------------------------------------------------------------------------
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)/
	rm -f api/openapi.bundled.json
	rm -f cover-*.out

# ---------------------------------------------------------------------------
# Help
# ---------------------------------------------------------------------------
.PHONY: help
help:
	@echo "Generation:"
	@echo "  gen              Generate all code (ent, ogen, mocks)"
	@echo "  ent-gen          Generate ent code from schema"
	@echo "  ogen             Generate API server code from OpenAPI spec"
	@echo "  mock             Generate mocks with mockery"
	@echo ""
	@echo "Lint:"
	@echo "  lint             Run golangci-lint"
	@echo ""
	@echo "Test:"
	@echo "  test             Run unit tests (no Docker required)"
	@echo "  test-all         Run all tests (requires Docker)"
	@echo "  test-e2e         Run e2e tests (requires Docker)"
	@echo "  test-integration Run integration tests (requires Docker)"
	@echo "  test-kind        Run kind tests (requires kind cluster)"
	@echo ""
	@echo "Build:"
	@echo "  build            Build server binary"
	@echo "  run              Run server in development mode"
	@echo ""
	@echo "DB:"
	@echo "  migrate-diff     Generate migration diff (make name=xxx)"
	@echo "  migrate-apply    Apply pending migrations"
	@echo "  migrate-status   Show migration status"
	@echo "  migrate-lint     Lint migration files"
	@echo ""
	@echo "Hooks:"
	@echo "  hooks            Install lefthook git hooks"
	@echo ""
	@echo "Other:"
	@echo "  clean            Remove build artifacts"

.DEFAULT_GOAL := help
