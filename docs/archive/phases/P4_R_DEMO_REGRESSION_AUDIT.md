# Phase P4-R Demo Regression Audit

> Date: 2026-07-12

## Scope

P4-R closes demo regression stability only. It does not run real Douyin credential E2E, real environment security verification, production gray release, tag creation, or Production Ready marking.

## Step Audit

| Step | Command | Inherited Env | Dotenv | Database | Port | Background Process | Exit Semantics | Root Cause | Fix |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| go test regression | `node scripts/go-test-isolated.mjs test ./...` | Whitelist only | No | test defaults only | none | no | real `go test` exit | `demo:auto-acceptance` imported root `.env` before tests | isolated env builder + log |
| seed-demo-data | `pnpm seed:demo-data` | shell + root `.env` for API login | root `.env` | running demo backend DB | API base | no | 0 passed/warning, 2 blocked, 3 conflict | repeat seed treated validation gaps as exit 2 | dataset version + structured result |
| seed-demo-permissions | `pnpm seed:demo-permissions` | shell + root `.env` for API login | root `.env` | running demo backend DB | API base | no | 0 passed, 2 blocked, 4 manual | existing users/grants could be interpreted as failure | template version + structured result |
| ai-image-trial-run | `scripts/ai-image-trial-run.ps1` | shell + root `.env` for API login | root `.env` | running demo backend DB | API base | backend worker expected | 0 pass, 3 blocked, 4 failed, 5 warning | 600s timeout was not classified | progress stage + reason code |
| closure scan | `pnpm check:p4-r` | shell only | no | none | none | no | failed count | no P4-R scan existed | added static closure check |

## Current Root Causes

- Go tests were vulnerable to parent process pollution because the demo orchestrator loaded root `.env`.
- Demo seed scripts had no version marker or normalized JSON result.
- AI image trial timeout had insufficient evidence fields for deciding provider, worker, lease, storage, scan, polling, or state-machine cause.

## Boundary

Production and staging demo seed are forbidden unless explicitly enabled. Real Douyin credentials and real environment security verification remain deferred.
