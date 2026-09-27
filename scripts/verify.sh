#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "${root_dir}/web/management-center"
bun install --frozen-lockfile
bun run verify

cd "${root_dir}"
go test ./...
go build -o test-output ./cmd/server
rm -f test-output
