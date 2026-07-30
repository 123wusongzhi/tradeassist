# systemd Deployment Guide

## Units

- `deploy/systemd/trademind-api.service` — main API + in-process workers
- `deploy/systemd/trademind-worker.service` — placeholder (workers run in API process)

## Install

```bash
sudo cp deploy/systemd/trademind-api.service /etc/systemd/system/
sudo mkdir -p /etc/trademind
sudo cp .env.production.example /etc/trademind/trademind.env
sudo chmod 600 /etc/trademind/trademind.env
# edit secrets
sudo systemctl daemon-reload
sudo systemctl enable --now trademind-api
```

## Migrations

Run once before first start or via release script:

```bash
cd backend && go run ./cmd/server/  # AutoMigrate on boot with advisory lock via single instance
```

Multi-instance: run migration from one node only; readiness fails until migrations complete.

## Logs

`journalctl -u trademind-api -f`
