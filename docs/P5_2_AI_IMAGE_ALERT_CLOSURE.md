# P5.2 AI Image Alert Closure

Code-level closure:

1. AI image service records provider timeout through `ObserveAIImage(..., "timeout", ...)`.
2. `metrics.Catalog` increments `ai_image_provider_timeouts_total`.
3. Alert rule `ai_image_provider_timeout` reads that metric from the registry snapshot.
4. `AlertService.EvaluateRules` creates firing alerts and delivery rows.
5. Recovery is verified when the metric snapshot returns to zero.

Verification: `go test ./internal/modules/aiproductimage ./internal/modules/alerting`.

Real external alert channel verification remains deferred.
