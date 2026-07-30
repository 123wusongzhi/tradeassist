# Memory Growth

Meaning: process memory grows without returning during steady load.

Check: export/upload paths, cache entries, goroutine count, heap profile, temp files and large `ReadAll` usage.

Mitigate: pause exports, reduce batch size, lower concurrency and collect internal-only heap profile.

Scale: memory limit increases require capacity model update.

Forbidden: do not expose pprof publicly or record request bodies/secrets in profiles.

Recovery: memory stabilizes across soak windows.
