#!/usr/bin/env bash
set -euo pipefail
DRY_RUN=false
[[ "${1:-}" == "--dry-run" ]] && DRY_RUN=true
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
VERSION="${APP_VERSION:-$(date +%Y%m%d%H%M%S)}"
RELEASE_DIR="/opt/trademind/releases/$VERSION"
echo "[deploy-staging] version=$VERSION"
if $DRY_RUN; then echo "[deploy-staging] dry-run"; exit 0; fi
"$ROOT/deploy/scripts/build-backend.sh"
"$ROOT/deploy/scripts/build-admin.sh"
sudo mkdir -p "$RELEASE_DIR"
sudo rsync -a --delete "$ROOT/backend/bin" "$ROOT/admin/dist" "$ROOT/deploy" "$RELEASE_DIR/"
sudo ln -sfn "$RELEASE_DIR" /opt/trademind/current
sudo systemctl restart trademind-api.service
"$ROOT/deploy/scripts/check-readiness.sh"
