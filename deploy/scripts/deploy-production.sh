#!/usr/bin/env bash
set -euo pipefail
echo "[deploy-production] use deploy-staging.sh with production env file on target host"
echo "[deploy-production] requires manual approval — no auto gray release"
exec "$(dirname "$0")/deploy-staging.sh" "$@"
