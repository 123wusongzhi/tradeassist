#!/usr/bin/env bash
set -euo pipefail
DRY_RUN=false
if [[ "${1:-}" == "--dry-run" ]]; then DRY_RUN=true; fi
PRE_START=false
if [[ "${1:-}" == "--pre-start" ]]; then PRE_START=true; fi

BASE_URL="${API_PUBLIC_URL:-http://127.0.0.1:8080}"
LIVE_URL="$BASE_URL/health/live"
READY_URL="$BASE_URL/health/ready"

check() {
  local url="$1" name="$2"
  echo "[check-readiness] $name → $url"
  if $DRY_RUN; then return 0; fi
  code="$(curl -sf -o /tmp/tm-ready.json -w '%{http_code}' "$url" || true)"
  if [[ "$code" != "200" ]]; then
    echo "[check-readiness] FAILED $name HTTP $code"
    cat /tmp/tm-ready.json 2>/dev/null || true
    exit 1
  fi
  echo "[check-readiness] OK $name"
}

if $PRE_START; then
  echo "[check-readiness] pre-start skip (API not running yet)"
  exit 0
fi

check "$LIVE_URL" "liveness"
check "$READY_URL" "readiness"
