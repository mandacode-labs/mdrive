#!/usr/bin/env bash
set -euo pipefail
name="${1:?usage: $0 <name>}"
cd "$(dirname "$0")/.."
exec go run ariga.io/atlas/cmd/atlas migrate diff "$name" \
	--dir "file://ent/migrate/migrations" \
	--to "ent://ent/schema" \
	--dev-url "docker://postgres/17/dev?search_path=public"
