#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
exec go test -count=1 ./test/kind/... -tags kind -timeout 30m
