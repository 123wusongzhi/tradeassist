# P7 Memory Budget

Current limits:

- Upload multipart memory bounded by `UPLOAD_MAX_MB`.
- Export limits are configured through P7 env.
- Dataset loader dry-run avoids materializing large data in memory.

Closure requires load/soak reports with memory peak, GC trend and no unbounded goroutine growth.
