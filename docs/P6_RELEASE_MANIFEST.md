# P6 Release Manifest

Release manifest fields:

- release id
- version
- git commit
- git tree state
- build time
- Go / Node / pnpm versions
- artifact SHA-256 values
- migration compatibility range
- configuration schema version
- rollback compatibility
- required features
- manifest hash

Forbidden:

- `.env`
- database password
- OAuth token
- API key
- master key
- local logs
- `node_modules`
- build cache

