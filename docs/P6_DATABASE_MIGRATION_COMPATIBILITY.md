# P6 Database Migration Compatibility

Default production migration strategy:

`Expand -> Deploy compatible app -> Backfill -> Switch reads/writes -> Contract in later release`

Rules:

- New fields should be nullable or have safe defaults.
- Do not delete or rename production-critical fields in the same release.
- Use Advisory Lock for startup migration.
- Run pre-release backup before migration when release is enabled.
- Migration failure stops release.
- Down migration is not the default rollback path.
