# P5.2 Auth Security Observability

Auth observability covers login attempt/success/failure, session create/revoke, refresh success/failure and refresh-token reuse detection.

Refresh reuse increments `auth_refresh_reuse_detected_total`, which is used by the critical alert rule `auth_refresh_reuse`.

Credentials, tokens, cookies and session IDs are not logged or used as metric labels.

Verification: `go test ./internal/modules/auth`.
