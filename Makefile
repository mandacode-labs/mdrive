# Tools
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p "$(LOCALBIN)"

GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
GOLANGCI_LINT_VERSION ?= v2.0.2
GOSEC = $(LOCALBIN)/gosec
GOSEC_VERSION ?= v2.22.3
GORELEASER = $(LOCALBIN)/goreleaser
GORELEASER_VERSION ?= v2.8.1
ATLAS ?= atlas

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] && [ "$$(readlink "$(1)" 2>/dev/null)" = "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f "$(1)" ;\
GOBIN="$(LOCALBIN)" go install $${package} ;\
mv "$(LOCALBIN)/$$(basename "$(1)")" "$(1)-$(3)" ;\
} ;\
ln -sf "$$(realpath "$(1)-$(3)")" "$(1)"
endef

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

.PHONY: gosec
gosec: $(GOSEC) ## Download gosec locally if necessary.
$(GOSEC): $(LOCALBIN)
	$(call go-install-tool,$(GOSEC),github.com/securego/gosec/v2/cmd/gosec,$(GOSEC_VERSION))

.PHONY: goreleaser
goreleaser: $(GORELEASER) ## Download goreleaser locally if necessary.
$(GORELEASER): $(LOCALBIN)
	$(call go-install-tool,$(GORELEASER),github.com/goreleaser/goreleaser/v2,$(GORELEASER_VERSION))

# Linting
.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	"$(GOLANGCI_LINT)" run

.PHONY: sec
sec: gosec ## Run gosec security scanner
	"$(GOSEC)" ./...

.PHONY: fmt
fmt: golangci-lint ## Run go fmt and fix lint issues
	"$(GOLANGCI_LINT)" fmt
	"$(GOLANGCI_LINT)" run --fix

# Code Generation
.PHONY: ent-gen
ent-gen: ## Generate ent code
	go run -mod=mod entgo.io/ent/cmd/ent generate ./ent/schema

.PHONY: openapi-bundle
openapi-bundle: ## Bundle OpenAPI spec into single JSON file
	npx @apidevtools/swagger-cli bundle api/openapi.yaml --outfile api/openapi.bundled.json --type json

.PHONY: openapi-validate
openapi-validate: ## Validate bundled OpenAPI spec
	npx @apidevtools/swagger-cli validate api/openapi.bundled.json

.PHONY: openapi
openapi: openapi-bundle openapi-validate ## Bundle and validate OpenAPI spec

.PHONY: ogen
ogen: openapi-bundle ## Generate API code from OpenAPI spec
	@rm -f pkg/api/oas_*.go
	go tool ogen -config ogen.yaml -target ./pkg/api -package api api/openapi.bundled.json

.PHONY: mock
mock: ## Generate mocks
	@find ./internal -type d -name "mocks" -exec rm -rf {} + 2>/dev/null || true
	mockery

.PHONY: gen
gen: ent-gen ogen mock ## Generate all code (ent, ogen, mocks)

# Testing
.PHONY: test
test: ## Run unit tests
	go test -v $$(go list ./... | grep -v -e /mocks -e /test/e2e) --coverprofile cover-unit.out

.PHONY: test-e2e
test-e2e: ## Run e2e tests
	go test -v ./test/e2e/... -timeout 10m --coverprofile cover-e2e.out

.PHONY: test-integration
test-integration: ## Run integration tests
	go test -v $$(go list ./... | grep -v -e /mocks -e /test/e2e) -tags integration -timeout 5m --coverprofile cover-integration.out

# Migrations
MIGRATION_DIR ?= ent/migrate/migrations

.PHONY: migrate-diff
migrate-diff: ## Generate a new migration diff. Usage: make migrate-diff name=add_column
	$(ATLAS) migrate diff $(name) \
		--dir "file://$(MIGRATION_DIR)" \
		--to "ent://ent/schema" \
		--dev-url "docker://postgres/17/dev?search_path=public"

.PHONY: migrate-apply
migrate-apply: ## Apply pending migrations
	go run ./cmd/retrowin-server migrate apply --config config.yaml

.PHONY: migrate-status
migrate-status: ## Show migration status
	"$(ATLAS)" migrate status --dir "file://$(MIGRATION_DIR)"

.PHONY: migrate-lint
migrate-lint: ## Lint migration files for safety
	"$(ATLAS)" migrate lint --dir "file://$(MIGRATION_DIR)" \
		--dev-url "docker://postgres/17/dev?search_path=public"

# Building
.PHONY: build
build: ## Build server binary
	go build -o bin/retrowin-server ./cmd/retrowin-server

.PHONY: release
release: goreleaser ## Release with goreleaser
	goreleaser release --clean

.PHONY: release-snapshot
release-snapshot: goreleaser ## Release snapshot (for testing)
	goreleaser release --snapshot --clean

# All-in-one
.PHONY: all
all: gen build ## Generate all code and build

# Cleanup
.PHONY: clean
clean: ## Clean build artifacts
	rm -rf bin/
	rm -f api/openapi.bundled.json
	rm -f cover.out

.PHONY: help
help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
