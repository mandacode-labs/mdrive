.SHELLFLAGS = -e -c

APP_NAME    := mdrive
BUILD_DIR   := bin
SCRIPTS     := scripts

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
	go run github.com/vektra/mockery/v2/cmd/mockery

# ---------------------------------------------------------------------------
# Lint
# ---------------------------------------------------------------------------
.PHONY: check-format
check-format:
	! grep -IUrn "[[:blank:]]$$" . --include="*.go" --include="*.yaml" --include="*.yml" --include="*.json" --include="*.md"
	git ls-files -- '*.go' '*.yaml' '*.yml' '*.json' '*.md' | while read f; do \
	  if [ -s "$$f" ] && [ "$$(tail -c 1 "$$f")" != "" ]; then \
	    echo "No newline at EOF: $$f"; exit 1; \
	  fi; \
	done
	! gofmt -l . | read -r

.PHONY: check-vet
check-vet:
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

.PHONY: test-all
test-all:
	go test -count=1 ./... -coverprofile=cover.out

.PHONY: test-e2e
test-e2e:
	$(SCRIPTS)/test-e2e.sh

.PHONY: test-integration
test-integration:
	$(SCRIPTS)/test-integration.sh

.PHONY: test-kind
test-kind:
	$(SCRIPTS)/test-kind.sh

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------
.PHONY: build
build:
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/mdrive

.PHONY: run
run:
	go run ./cmd/mdrive serve --config config.yaml

# ---------------------------------------------------------------------------
# Database Migrations
# ---------------------------------------------------------------------------
.PHONY: migrate-diff
migrate-diff:
	$(SCRIPTS)/migrate-diff.sh $(name)

.PHONY: migrate-apply
migrate-apply:
	go run ./cmd/mdrive migrate apply --config config.yaml

.PHONY: migrate-status
migrate-status:
	$(SCRIPTS)/migrate-status.sh

.PHONY: migrate-lint
migrate-lint:
	$(SCRIPTS)/migrate-lint.sh

# ---------------------------------------------------------------------------
# Hooks
# ---------------------------------------------------------------------------
.PHONY: hooks
hooks:
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
