# P4.2 Security Center UI

Admin security center: unified dashboard at `/settings/security`.

## Status Banner

**Security Center UI Implemented** · **Rotation + Sessions + File Stats** · **NOT Production Ready**

---

## Page structure

File: `admin/src/pages/Settings/Security/index.tsx`  
Service: `admin/src/services/security.ts`

| Section | Content |
| --- | --- |
| **安全中心** (hero) | Overview copy + refresh button |
| **运行概览** | Auth mode, active sessions, JWT/Master key IDs, TTL stats |
| **认证与会话** | ProTable of sessions; revoke-other / logout-all actions |
| **租户隔离状态** | P4 configstatus items (tenant context, shop scope, etc.) |
| **主密钥轮换** | Prepare/start/pause/resume/verify; progress + key references |
| **文件安全** | Scan status counts (`pending_scan`, `clean`, `quarantined`, …) |
| **审计完整性** | Hash chain status + verify action |
| **策略配置** | `session_idle_timeout_min` + presets |
| **传输安全** | `force_https` toggle |
| **回调签名校验** | `ops_webhook_secret` (encrypted) |

---

## API wiring

| UI action | API |
| --- | --- |
| Overview | `GET /api/v1/security/overview` |
| Sessions list | `GET /api/v1/auth/sessions` |
| Revoke session | `DELETE /api/v1/auth/sessions/:id` |
| Revoke others | `POST /api/v1/auth/sessions/revoke-others` |
| Logout all | `POST /api/v1/auth/sessions/logout-all` |
| Rotation prepare/start | `POST /api/v1/security/keys/rotation/prepare|start` |
| Rotation progress | `GET /api/v1/security/keys/rotation/:id/progress` |
| Key references | `GET /api/v1/security/keys/references` |
| Audit integrity | `GET/POST /api/v1/security/audit/integrity/*` |
| File security stats | `GET /api/v1/security/files/stats` (or equivalent files module endpoint) |
| Settings save | `PUT /api/v1/settings` (`security` group) |

Confirm phrases match backend: `ROTATE-KEYS-DRY-RUN`, `ROTATE-KEYS-START`.

---

## Permissions

- Page menu: `SETTINGS_MANAGE`
- Rotation actions: `security.key_rotate` (admin) enforced server-side

---

## Related

- `/settings/config-status` — full environment readiness (links to security page for P4 items)
- `/settings/alert-notify` — separate alert webhook secret
