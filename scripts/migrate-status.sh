#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
exec go run ariga.io/atlas/cmd/atlas migrate status --dir "file://ent/migrate/migrations"
