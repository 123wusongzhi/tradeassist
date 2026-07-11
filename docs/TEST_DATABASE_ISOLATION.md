# Test Database Isolation

P3.2 regression tests use isolated SQLite DSNs and avoid shared fixed database files.

Current pattern:

- Webhook tests use `file:webhook_<uuid>?mode=memory&cache=shared`.
- SQLite tests set `SetMaxOpenConns(1)` where needed to avoid driver lock noise during concurrency tests.
- No fixed `test.db` file is shared across packages.
- HTTP tests use `httptest` and do not bind fixed ports.
- Worker-style tests call service methods directly or use cancellable contexts.

When adding tests for webhook, order sync, token refresh, or workers:

- Prefer `t.TempDir()` or a UUID memory DSN.
- Close SQL handles when a test intentionally simulates persistence failure.
- Do not start background workers without a cancellable context and cleanup.
- Do not run hidden concurrent `go test` processes from acceptance scripts.
