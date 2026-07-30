#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT/backend"
echo "[build-backend] go build ./cmd/server/..."
go build -o "$ROOT/backend/bin/server" ./cmd/server/...
echo "[build-backend] done: backend/bin/server"
