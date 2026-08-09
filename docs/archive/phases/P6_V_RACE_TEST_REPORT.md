# P6-V Linux Race Test Report

Status: passed

Run ID: p6-vr-2026-07-13T09-22-23-945Z

Generated At: 2026-07-13T09:22:23.945Z

Runner: WSL2 Ubuntu

Distribution: Ubuntu 22.04.5 LTS

Kernel: Linux SK-20250814VKAY 6.6.87.2-microsoft-standard-WSL2 #1 SMP PREEMPT_DYNAMIC Thu Jun  5 18:30:46 UTC 2025 x86_64 x86_64 x86_64 GNU/Linux

Go: go version go1.25.12 linux/amd64

Required Go: go1.25.0

Go Path: /usr/local/go/bin/go

CGO_ENABLED: true

GCC: gcc (Ubuntu 11.4.0-1ubuntu1~22.04.3) 11.4.0

Repository Path: /mnt/d/project/trademind-ai

Git Commit: 61173235d460ea5eec223ee4cc1244014e454abb

Git Tree State: dirty

## Baseline

| Check | Status | Exit Code |
| --- | --- | --- |
| go mod download | passed | 0 |
| go mod verify | passed | 0 |
| go test ./... | passed | 0 |
| go build | passed | 0 |

## Race Matrix

| Package | Status | Exit Code |
| --- | --- | --- |
| backup | passed | 0 |
| restore | passed | 0 |
| release | passed | 0 |
| disasterrecovery | passed | 0 |
| backupruntime | passed | 0 |
| artifact | passed | 0 |
| taskcenter | passed | 0 |
| alerting | passed | 0 |
| operationlog | passed | 0 |
| combined | passed | 0 |

Data races: 0

Deadlocks: 0

Environment blocked: false

## Previous Environment Blocker

The first P6-V Linux race attempt was `environment_blocked` because the WSL default Go path was `/usr/bin/go` and the WSL Go version was `go version go1.18.1 linux/amd64`, while `backend/go.mod` requires `go1.25.0`. The remediated run uses `/usr/local/go/bin/go` with `go version go1.25.12 linux/amd64`.

This report is valid only for the recorded Linux / WSL2 run. Real production backup, restore, PITR, release, telemetry, and Douyin credential verification remain Deferred.
