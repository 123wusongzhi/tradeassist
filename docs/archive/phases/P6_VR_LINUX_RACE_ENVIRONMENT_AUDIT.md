# P6-VR Linux Race Environment Audit

Status: completed

Generated: 2026-07-13

## Scope

This audit covers only the P6-VR Linux race environment remediation and closure gate. It does not access real production databases, production backups, production traffic, Nginx/systemd, or real Douyin credentials.

## Repository Requirement

| Item | Value |
| --- | --- |
| Source | `backend/go.mod` |
| `go` directive | `1.25.0` |
| `toolchain` directive | not set |
| Required Go version | `go1.25.0` or compatible newer `go1.25.x` |
| Target Go version installed | `go1.25.12` |

## Host And WSL

| Item | Value |
| --- | --- |
| Windows host | `Windows_NT 10.0.19045 x64` |
| WSL distribution | `Ubuntu-22.04` |
| WSL version | `2` |
| WSL distribution release | `Ubuntu 22.04.5 LTS` |
| WSL kernel | `Linux SK-20250814VKAY 6.6.87.2-microsoft-standard-WSL2` |
| Architecture | `x86_64` |
| Repository path used for verification | `/mnt/d/project/trademind-ai` |
| Git commit | `61173235d460ea5eec223ee4cc1244014e454abb` |
| Git tree state | `dirty` |

## Before Remediation

| Item | Value |
| --- | --- |
| Default WSL Go path | `/usr/bin/go` |
| Default WSL Go version | `go version go1.18.1 linux/amd64` |
| Existing manual Go path | `/usr/local/go/bin/go` |
| Existing manual Go version | `go version go1.24.2 linux/amd64` |
| Initial race status | `environment_blocked` |
| Initial reason code | `GO_VERSION_INCOMPATIBLE` |

The original `environment_blocked` evidence is retained in `docs/p6-v-race-test-report.json` under `previousEnvironmentBlocked`.

## After Remediation

| Item | Value |
| --- | --- |
| Active WSL Go path | `/usr/local/go/bin/go` |
| Active WSL Go version | `go version go1.25.12 linux/amd64` |
| GOOS | `linux` |
| GOARCH | `amd64` |
| CGO_ENABLED | `1` |
| GOROOT | `/usr/local/go` |
| GOPATH | `/root/go` |
| GCC path | `/usr/bin/gcc` |
| GCC version | `gcc (Ubuntu 11.4.0-1ubuntu1~22.04.3) 11.4.0` |
| Node version | `v22.22.1` |
| pnpm version | `9.15.4` |
| Go module proxy used for WSL download | `https://goproxy.cn,direct` |

## Installation Evidence

| Item | Value |
| --- | --- |
| Go archive | `go1.25.12.linux-amd64.tar.gz` |
| Source | `https://go.dev/dl/go1.25.12.linux-amd64.tar.gz` |
| SHA-256 | `234828b7a89e0e303d2556310ee549fbcf253d28de937bac3da13d6294262ac1` |
| SHA-256 verification | passed |
| Install directory | `/usr/local/go` |
| WSL profile path | `/etc/profile.d/trademind-go-toolchain.sh` |

## Environment Conclusion

WSL2 Ubuntu is available, the active Linux Go toolchain satisfies `backend/go.mod`, CGO is enabled, GCC is available, and the repository path is accessible. P6-VR Linux race verification is no longer environment-blocked.
