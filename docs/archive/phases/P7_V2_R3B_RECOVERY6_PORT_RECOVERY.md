# P7-V2-R3B Recovery6 Port Recovery

Status: **failed**

- Port: `18080`
- Initial listener: WSL `server` PID `1131` via Windows `wslrelay.exe`
- Classification: `verified_trademind_stale_process`
- Stop action: `SIGTERM`
- Graceful stop succeeded: `true`
- Unknown processes killed: `0`
- Windows listeners after recovery: `0`
- WSL listeners after recovery: `0`
- Bind probe passed: `true`
- Probe listener released: `true`

## Residuals

- Dataset executed: `false`
- k6 executed: `false`
- Raw artifact exists: `false`
- Frozen artifact exists: `false`
- Baseline registry active: `false`
- Recovery6 baseline database dropped: `trademind_p7v2_p7v2_baseline_r3b_recovery6_20260715103749`

## Reuse Gate

- Runtime freeze still valid: `false`
- Execution code changed: `false`
- Formal config changed: `true`
- Baseline run ID reusable: `false`
- Current run ID reusable: `false`
- New runtime freeze required: `true`
- New Recovery6 plan required: `true`

## Issues

- `runtime_freeze_config_fingerprint_mismatch`
- `runtime_freeze_tracked_diff_hash_mismatch`
