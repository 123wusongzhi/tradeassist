#!/usr/bin/env bash
set -euo pipefail
export PATH=/usr/bin:/bin
GOVER=1.26.2
GOROOT_LOCAL="$HOME/.local/go-${GOVER}"
URL="https://mirrors.aliyun.com/golang/go${GOVER}.linux-amd64.tar.gz"
ALT="https://go.dev/dl/go${GOVER}.linux-amd64.tar.gz"
mkdir -p "$HOME/.local"
if [ ! -x "$GOROOT_LOCAL/bin/go" ]; then
  echo "Downloading Go ${GOVER} to ${GOROOT_LOCAL}..."
  if ! curl -fsSL "$URL" -o /tmp/go.tgz; then
    curl -fsSL "$ALT" -o /tmp/go.tgz
  fi
  rm -rf "$GOROOT_LOCAL" "$HOME/.local/go"
  tar -C "$HOME/.local" -xzf /tmp/go.tgz
  mv "$HOME/.local/go" "$GOROOT_LOCAL"
fi
export GOROOT="$GOROOT_LOCAL"
export PATH="$GOROOT_LOCAL/bin:/usr/bin:/bin"
export GOTOOLCHAIN=local
export GOPROXY=https://goproxy.cn,direct
echo "Using $(command -v go)"
go version
cd /mnt/d/project/trademind-ai/backend
CGO_ENABLED=1 go test -race \
  ./internal/modules/idempotency/... \
  ./internal/pkg/tasklease/... \
  ./internal/modules/aiproducttext/... \
  ./internal/modules/aiproductimage/... \
  ./internal/modules/webhook/... \
  ./internal/modules/collect/... \
  ./internal/modules/imagetask/... \
  ./internal/modules/customersync/...
