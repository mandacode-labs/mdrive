#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
exec go run ariga.io/atlas/cmd/atlas migrate lint \
	--dir "file://ent/migrate/migrations" \
	--dev-url "docker://postgres/17/dev?search_path=public"
