#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
exec go test -count=1 ./test/integration/... -tags integration -timeout 5m -coverprofile=cover-integration.out
