# P7 Export And Upload Streaming

Config foundation:

- `EXPORT_BATCH_SIZE`
- `EXPORT_MAX_ROWS`
- `EXPORT_MAX_BYTES`
- `EXPORT_MAX_CONCURRENT`
- `UPLOAD_MAX_MB`

Closure requires streaming export/upload tests covering cancellation, timeouts, max rows, max bytes, temp cleanup and memory peak evidence.
