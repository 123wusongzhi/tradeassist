# P6 Object Storage Inventory

Inventory stores metadata only:

- `objectKeyHash`
- size
- etag/checksum
- mime group
- security status
- created at
- storage provider

Rules:

- Do not display full object keys in normal reports.
- Do not store signed URLs.
- Support missing / mismatch / extra classification.
- Do not automatically delete extra objects.
- Do not promote quarantined objects to clean.
- Large-scale scanning is deferred to P7.

