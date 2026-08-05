# P7-V2-R3B Dedicated Benchmark Host Final Gate

Status: **failed**

- Formal host isolation version: 3
- Dedicated benchmark host contract version: 1
- Native Linux filesystem: false
- Exclusive benchmark window: true
- Toolchain attestation passed: false
- Host fingerprint present: true
- Host contract immutable: true
- Failed checks: preflight:nodePlatform, preflight:GOOS, preflight:nativeLinuxFilesystem, preflight:freeDiskHeadroomPassed, preflight:dockerDaemonReachable, preflight:gccPath, preflight:k6VersionMatch, preflight:workingTreeClean, preflight:timeSyncActive, preflight:thermalThrottleDetected, preflightStatusPassed, nativeLinuxFilesystem, toolchainAttestationPassed, dockerDaemonReachable, k6VersionMatch

This gate closes only the dedicated-host tooling contract. It does not make P7-V2 production ready and does not authorize a formal run.
