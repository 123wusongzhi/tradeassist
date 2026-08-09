# P4 Audit Integrity

Hash-chained operation logs for tamper-evident admin audit trail.

## Status Banner

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**

---

## Overview

`operation_logs` rows are append-only with optional hash chain fields linking each entry to its predecessor within a **partition** (tenant + UTC date).

Implementation: `backend/internal/modules/operationlog/hash_chain.go`.

---

## OperationLog Schema (Security Fields)

| Column | Purpose |
| --- | --- |
| `tenant_id` | Partition component |
| `action`, `resource`, `resource_id` | Event semantics |
| `status`, `message` | Outcome |
| `request_id` | Correlation |
| `admin_user_id`, `session_id` | Actor binding |
| `shop_id`, `platform` | Scope |
| `ip_hash`, `user_agent_summary` | Client fingerprint (hashed/summary) |
| `prev_hash` | Previous entry hash in partition |
| `entry_hash` | SHA-256 chain link |
| `hash_version` | Schema version (currently `1`) |
| `chain_partition` | `t{tenantID}:YYYY-MM-DD` |

Model: `backend/internal/modules/operationlog/model.go`.

---

## Hash Computation

```text
message_digest = SHA256(message)
payload = join("|",
  prev_hash,
  tenant_id,
  action,
  resource,
  resource_id,
  status,
  request_id,
  created_at_RFC3339Nano,
  hex(message_digest)
)
entry_hash = SHA256(payload) as hex
```

First entry in partition has empty `prev_hash`.

Function: `computeEntryHash` in `hash_chain.go`.

---

## Write Path

On `operationlog.Service.Write`:

1. Build row from Gin context (admin, session, request ID, tenant from ctx)
2. Transaction: `appendHashChain(tx, row)` before INSERT
3. Lookup latest row in same `chain_partition` for `prev_hash`
4. Set `entry_hash` and insert

Rows without `entry_hash` are skipped during verify (legacy rows).

---

## Verification API

Module: `securitymod`

| Endpoint | Permission | Action |
| --- | --- | --- |
| `GET /security/audit/integrity/status` | `audit.read` | Verify last 7 days, tenant 0 |
| `POST /security/audit/integrity/verify` | `audit.read` | JSON `{ "days": N }` |

Service method:

```go
OpLogs.VerifyChain(ctx, tenantID, from, to) → (count, mismatchTime, error)
```

Walks rows ordered by `created_at ASC`, recomputes hash, checks `prev_hash` linkage.

Mismatch returns: `audit chain mismatch at index N`.

---

## Partition Strategy

```text
chain_partition = fmt.Sprintf("t%d:%s", tenantID, date.UTC.Format("2006-01-02"))
```

- New partition daily per tenant
- Cross-day tamper requires breaking multiple chains
- High-volume tenants: consider hourly partitions (future)

---

## Logged Actions (Examples)

| Action | Resource | Trigger |
| --- | --- | --- |
| `session_revoke` | auth_session | User revokes session |
| `session_revoke_others` | auth_session | Revoke other devices |
| `logout_all` | auth | Logout everywhere |
| Settings changes | settings | Via settings handler OpLog |
| Shop OAuth | shops | Token refresh/revoke |

Sensitive actions should always call `operationlog.Write`.

---

## Immutability

- No soft delete on `operation_logs`
- Updates/deletes not exposed via API
- DB-level append-only policy recommended for production (GRANT INSERT only)

---

## Limitations

| Limitation | Impact |
| --- | --- |
| Verify defaults to tenant 0 | Multi-tenant needs per-tenant job |
| Hash excludes full request body | Message field only digested |
| No external anchor ( blockchain ) | DBA with UPDATE could rewrite if DB ACL weak |
| Clock skew | Uses row `created_at` at insert |

---

## Hardening Recommendations

1. DB role: application user INSERT-only on `operation_logs`
2. Ship logs to WORM/SIEM export asynchronously
3. Scheduled `VerifyChain` cron per tenant with alert on failure
4. Include `permission` field in hash payload v2 (breaking change)

---

## Deferred Verification

- [ ] Tamper simulation test (modify mid-chain row → verify fails)
- [ ] Multi-tenant verify automation
- [ ] Performance test on 100k row partition

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**
