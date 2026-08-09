# P7-V2-R3B Auth Operation Log Tail Repair

Status: **local repair completed; formal rerun not started**

This document records the minimal Auth invalid-login operation-log tail repair. It does not pass P7-V2 and is not valid for closure.

## Root Cause

- primaryRootCause: `B_auth_audit_or_operation_log_db_tail`
- confidence: `high`
- repairPath: `auth_operation_log_hash_chain_or_commit_path_minimal_fix`

## Actual Call Chain

```text
POST /api/v1/auth/login
-> auth.Handler.Login
-> auth.LoginService.Login
-> auth.SessionService.CreateSession
-> LoginGuard.CheckAllowed
-> admin.Store.ByLoginAccount
-> admin.CheckPassword bcrypt verification
-> LoginGuard.RecordFailure
-> operationlog.Service.Write
-> operationlog.Service.appendHashChain
-> operation_logs previous hash lookup
-> operation_logs insert
-> transaction commit
-> 401 response
```

## Repair

- Added `idx_operation_logs_p7_chain_partition_created_id` on `(chain_partition, created_at DESC, id DESC)`.
- Changed previous hash lookup to `WHERE chain_partition = ? ORDER BY created_at DESC, id DESC LIMIT 1`.
- Added PostgreSQL transaction-scoped advisory lock and predecessor `FOR UPDATE`.
- Added same-process per-partition mutex to serialize local same-scope writers.
- Preserved failed-login operation log, logical security audit, hash-chain fields, bcrypt verification, lockout/rate-limit behavior, and transaction semantics.
- Disabled P7 diagnostic JSONL writer/samplers when `APP_ENV=performance`, `formal=true`, and `diagnosticOnly=false`.

## Explain Summary

Isolated WSL PostgreSQL 14.23 temporary database, 20k `operation_logs` rows:

| Operation | Plan | Index | Rows Removed | Execution |
| --- | --- | --- | ---: | ---: |
| before previous hash lookup | `Limit -> Sort -> Seq Scan` | none | 13333 | 2.634 ms |
| after previous hash lookup | `Limit -> Index Scan` | `idx_operation_logs_p7_chain_partition_created_id` | 0 | 0.044 ms |

The after plan has no explicit Sort.

## Local Verification

- `go test ./internal/modules/auth/...`: passed
- `go test ./internal/modules/operationlog/...`: passed
- `go test ./internal/pkg/p7diag/...`: passed
- `go test -run '^$' -bench 'InvalidLogin|OperationLog|HashChain' -benchmem ./internal/modules/auth ./internal/modules/operationlog`: passed

Benchmarks are local repair evidence only, not Formal Closure Evidence.

## Formal Rerun

- formalRerunStarted: `false`
- Phase P7-V2 remains incomplete.
- Phase P7 Development Closure remains incomplete.
- Tag deferred.
- 非 Production Ready.
