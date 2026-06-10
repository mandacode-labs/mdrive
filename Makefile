.SHELLFLAGS = -e -c

APP_NAME    := mdrive
BUILD_DIR   := bin
SCRIPTS     := scripts

# ---------------------------------------------------------------------------
# Code Generation
# ---------------------------------------------------------------------------
.PHONY: generate
generate: gen-ent gen-api gen-mock

.PHONY: gen-ent
gen-ent:
	go run -mod=mod entgo.io/ent/cmd/ent generate ./ent/schema

.PHONY: gen-api
gen-api:
	npx @apidevtools/swagger-cli bundle api/openapi.yaml --outfile api/openapi.bundled.json --type json
	npx @apidevtools/swagger-cli validate api/openapi.bundled.json
	@rm -f pkg/api/oas_*.go
	go tool ogen -config ogen.yaml -target ./pkg/api -package api api/openapi.bundled.json

.PHONY: gen-mock
gen-mock:
	@find ./internal -type d -name "mocks" -exec rm -rf {} + 2>/dev/null || true
	go run github.com/vektra/mockery/v2/cmd/mockery

# ---------------------------------------------------------------------------
# Format & Vet
# ---------------------------------------------------------------------------
.PHONY: fmt
fmt:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint fmt

.PHONY: vet
vet:
	go vet ./...

.PHONY: lint
lint:
	$(SCRIPTS)/lint.sh

# ---------------------------------------------------------------------------
# Test
# ---------------------------------------------------------------------------
.PHONY: test
test:
	go test -count=1 $(shell go list ./... | grep -v -e /mocks -e /test/e2e -e /test/integration -e /test/kind)

.PHONY: test-integration
test-integration:
	go test -count=1 ./test/integration/... -tags integration -timeout 5m -coverprofile=cover-integration.out

.PHONY: test-e2e
test-e2e:
	go test -count=1 ./test/e2e/... -timeout 10m -coverprofile=cover-e2e.out

.PHONY: test-kind
test-kind:
	go test -count=1 ./test/kind/... -tags kind -timeout 30m

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------
.PHONY: build
build:
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/$(APP_NAME)

.PHONY: run
run:
	go run ./cmd/$(APP_NAME) serve --config config.yaml

# ---------------------------------------------------------------------------
# Database Migrations
# ---------------------------------------------------------------------------
.PHONY: migrate-diff
migrate-diff:
	go run ariga.io/atlas/cmd/atlas migrate diff $(name) \
		--dir "file://ent/migrate/migrations" \
		--to "ent://ent/schema" \
		--dev-url "docker://postgres/17/dev?search_path=public"

.PHONY: migrate-apply
migrate-apply:
	go run ./cmd/$(APP_NAME) migrate apply --config config.yaml

.PHONY: migrate-status
migrate-status:
	go run ariga.io/atlas/cmd/atlas migrate status --dir "file://ent/migrate/migrations"

.PHONY: migrate-lint
migrate-lint:
	go run ariga.io/atlas/cmd/atlas migrate lint \
		--dir "file://ent/migrate/migrations" \
		--dev-url "docker://postgres/17/dev?search_path=public"

# ---------------------------------------------------------------------------
# Hooks
# ---------------------------------------------------------------------------
.PHONY: install-hooks
install-hooks:
	$(SCRIPTS)/hooks.sh

# ---------------------------------------------------------------------------
# Cleanup
# ---------------------------------------------------------------------------
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)/
	rm -f api/openapi.bundled.json
	rm -f cover-*.out

.DEFAULT_GOAL := help
help:
	@grep -E '^[a-z0-9-]+:' Makefile | cut -d: -f1 | sort
