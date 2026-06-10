#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
exec go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint run "$@"
