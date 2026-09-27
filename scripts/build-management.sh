#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
frontend_dir="${root_dir}/web/management-center"
output_dir="${root_dir}/static"
version="${VERSION:-dev}"

if ! command -v bun >/dev/null 2>&1; then
  echo "bun is required to build the management center" >&2
  exit 1
fi

cd "${frontend_dir}"
bun install --frozen-lockfile
VERSION="${version}" bun run build

test -f dist/index.html
if find dist -mindepth 1 -type f ! -name index.html -print -quit | grep -q .; then
  echo "management center build produced external assets; expected one HTML file" >&2
  exit 1
fi

mkdir -p "${output_dir}"
cp dist/index.html "${output_dir}/management.html"

echo "Built ${output_dir}/management.html"
