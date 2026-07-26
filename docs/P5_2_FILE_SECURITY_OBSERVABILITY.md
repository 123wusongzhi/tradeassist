# P5.2 File And Security Observability

File scan observability covers enqueue, claim, queue age and terminal scan result. MIME is reduced to a controlled `mime_group`.

Security observability covers audit chain mismatch and controlled security event categories. Tenant/user/IP/resource identifiers are not labels.

Verification:

```bash
go test ./internal/modules/files ./internal/modules/securitymod
```
