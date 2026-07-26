#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"
echo "[build-admin] pnpm build:admin"
pnpm build:admin
echo "[build-admin] done: admin/dist"
