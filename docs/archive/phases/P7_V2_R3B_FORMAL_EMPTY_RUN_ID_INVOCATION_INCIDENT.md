# P7-V2 R3B Formal Empty Run ID Invocation Incident

Status: preserved for historical audit.

- Failed stage: Stage L
- Failed step: formal baseline environment start
- Failure cause: PowerShell expanded command substitution before WSL received the command.
- Effective command: `pnpm p7-v2:env:start -- --run-id`
- Invalid run ID: `p7v2-20260718041821`
- Invalid binary mode: `implicit_build`
- Invalid listener PID: `5142`

This incident happened before the Formal Invocation Contract was enforced. It is not recorded as a Formal Baseline failure because the Formal Baseline had not started.

Cleanup evidence:

- Planned Run IDs consumed: false
- Planned Run IDs retained: true
- Dataset executed: false
- k6 executed: false
- Invalid database deleted: true
- Invalid process stopped: true
- Listener count on port 18080 after cleanup: 0
- Historical evidence preserved: true
