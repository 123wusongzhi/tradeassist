#!/usr/bin/env bash
set -euo pipefail
DRY_RUN=false
[[ "${1:-}" == "--dry-run" ]] && DRY_RUN=true
UNIT="${2:-trademind-api.service}"
echo "[install-service] installing $UNIT"
if $DRY_RUN; then exit 0; fi
sudo cp deploy/systemd/trademind-api.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable trademind-api.service
echo "[install-service] enable complete — start with: sudo systemctl start trademind-api"
