#!/bin/sh
set -eu

cd "$(dirname "$0")/.."
go test -mod=readonly -race -timeout=60s ./...
go vet -mod=readonly ./...
node --check internal/panel/assets/app.js

check_dir=$(mktemp -d "${TMPDIR:-/tmp}/pulsewise-check.XXXXXX")
trap 'rm -rf "$check_dir"' EXIT HUP INT TERM
go build -mod=readonly -o "$check_dir/pulsewise" .
python3 tools/smoke_web.py "$check_dir/pulsewise"
