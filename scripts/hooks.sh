#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
exec go run github.com/evilmartians/lefthook install
