# P5.2 Webhook Observability

Webhook receive and worker processing are instrumented in `Service.Ingest` and `processEventRow`.

Covered metrics include request, signature/payload/replay rejection, duplicate, persisted, processed, duration and lag counters/histograms.

`eventId`, platform shop ID and tenant ID are not metric labels.

Verification: `go test ./internal/modules/webhook`.
