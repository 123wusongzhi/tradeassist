# P6 Restore Architecture

Restore chain:

`Verified Backup -> Restore Safety Gate -> Decrypt to controlled temp file -> pg_restore to explicit isolated database -> Compatibility Check -> Read-only App Check -> Integrity Validation -> Report -> Cleanup`

P6 implementation records safe restore plans and validation rows. It does not restore into production.

API:

- `GET /api/v1/ops/restores`
- `POST /api/v1/ops/restores`
- `GET /api/v1/ops/restores/:id`
- `POST /api/v1/ops/restores/:id/verify`

