#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
exec go test -count=1 ./test/e2e/... -timeout 10m -coverprofile=cover-e2e.out
