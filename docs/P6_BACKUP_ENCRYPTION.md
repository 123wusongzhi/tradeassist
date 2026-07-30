# P6 Backup Encryption

Backups use authenticated encryption:

- Random 256-bit data key per backup.
- AES-GCM chunked file encryption.
- Data key wrapped by existing P4 `encrypt.Service`.
- Manifest stores `encryptionKeyId` and `wrappedDataKey`, never plaintext keys.
- Integrity failure blocks restore.
- Temporary plaintext backup files are removed after encryption when the local code path runs.

Windows and Linux file permission behavior differs; code writes encrypted outputs with restrictive permissions where supported. Linux isolated drill remains the preferred validation environment.

