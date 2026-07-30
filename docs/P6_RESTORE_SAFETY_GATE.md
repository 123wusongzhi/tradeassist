# P6 Restore Safety Gate

Restore requires:

- backup completed
- backup verification passed
- checksum present
- target environment explicitly isolated
- target database name explicitly provided
- operator reauthentication
- high-risk confirmation
- production target rejected during P6

Forbidden:

- default overwrite of current database
- restore to production by default
- automatic database deletion
- arbitrary shell command input
- backup path passed by URL
- frontend shell execution

