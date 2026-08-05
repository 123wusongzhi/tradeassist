# P5.2 AI Observability

AI text and AI image batch services are instrumented at batch creation, provider request, timeout/failure and apply/reconciliation paths.

AI image provider timeout increments `ai_image_provider_timeouts_total`; this is the metric used by the alert rule `ai_image_provider_timeout`.

Prompt, generated response body, image URL, signed URL and object key are not emitted as metric labels.

Verification:

```bash
go test ./internal/modules/aiproducttext ./internal/modules/aiproductimage
```
