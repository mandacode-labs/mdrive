.SHELLFLAGS = -e -c

APP_NAME    := mdrive
BUILD_DIR   := bin
ATLAS_VERSION := $(shell cat ATLAS_VERSION)

# ---------------------------------------------------------------------------
# Code Generation
# ---------------------------------------------------------------------------
.PHONY: generate
generate: gen-ent gen-api gen-mock gen-fga

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
	mockery

.PHONY: gen-fga
gen-fga:
	fga model transform --file=internal/permission/model.fga \
		> internal/permission/model.json

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
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint run --timeout=5m

# ---------------------------------------------------------------------------
# Test
# ---------------------------------------------------------------------------
PKGS_WITH_TESTS := $(shell go list -f '{{.ImportPath}} {{if .TestGoFiles}}X{{end}}' ./... | awk 'NF==2 && $$2=="X" {print $$1}' | grep -v -e /mocks -e /test/e2e -e /test/integration -e /test/kind)

.PHONY: test
test:
	go test -count=1 -coverprofile=cover-unit.out $(PKGS_WITH_TESTS)

.PHONY: test-integration
test-integration:
	go test -count=1 ./test/integration/... -tags integration -timeout 5m -coverprofile=cover-integration.out

.PHONY: test-integration-ent
test-integration-ent:
	go test -count=1 ./test/integration/ent/... -tags integration_ent -timeout 5m -coverprofile=cover-integration-ent.out

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
	go run ./cmd/$(APP_NAME) api-server run --config config.yaml

# ---------------------------------------------------------------------------
# Migrations
# ---------------------------------------------------------------------------
.PHONY: migrate
migrate:
	go run ariga.io/atlas/cmd/atlas@$(ATLAS_VERSION) migrate diff $(name) \
		--dir "file://internal/cli/migrate/migrations" \
		--to "ent://ent/schema" \
		--dev-url "docker://postgres/17/dev?search_path=public"

migrate-hash:
	go run ariga.io/atlas/cmd/atlas@$(ATLAS_VERSION) migrate hash \
		--dir "file://internal/cli/migrate/migrations"

migrate-validate:
	go run ariga.io/atlas/cmd/atlas@$(ATLAS_VERSION) migrate validate \
		--dir "file://internal/cli/migrate/migrations" \
		--dev-url "docker://postgres/17/dev?search_path=public"

.PHONY: migrate-apply
migrate-apply:
	go run ./cmd/$(APP_NAME) migrate apply --config config.yaml

# ---------------------------------------------------------------------------
# Helm
# ---------------------------------------------------------------------------
.PHONY: helm-lint
helm-lint:
	helm lint charts/mdrive

.PHONY: helm-template
helm-template:
	helm template mdrive charts/mdrive

# ---------------------------------------------------------------------------
# Hooks
# ---------------------------------------------------------------------------
.PHONY: install-hooks
install-hooks:
	go run github.com/evilmartians/lefthook install

# ---------------------------------------------------------------------------
# Cleanup
# ---------------------------------------------------------------------------
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)/
	rm -f api/openapi.bundled.json
	rm -f cover-*.out

.PHONY: help
.DEFAULT_GOAL := help
help:
	@grep -E '^[a-z0-9-]+:' Makefile | cut -d: -f1 | sort
