# P6 Backup Verification

Verification states:

- `pending`
- `passed`
- `failed`
- `manual_review`

Checks:

- file exists
- size above minimum
- SHA-256 checksum
- manifest checksum
- encryption metadata present when encryption is enabled
- `pg_restore --list` command foundation
- schema and migration version recorded

Failed verification keeps the backup out of the restore-safe list.

