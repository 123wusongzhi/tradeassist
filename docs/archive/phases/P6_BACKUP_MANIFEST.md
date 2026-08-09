# P6 Backup Manifest

Manifest version: `p6-v1`.

Required fields include backup id, backup type, environment, service version, database engine, schema/migration version, artifact name, artifact size, checksum, encryption status, key id, storage provider, storage location hash, backup status, verification status, and manifest checksum.

Forbidden fields:

- database password
- object storage secret
- master key
- full private object path
- signed URL
- full PII

The manifest checksum is calculated separately from the backup artifact checksum.

