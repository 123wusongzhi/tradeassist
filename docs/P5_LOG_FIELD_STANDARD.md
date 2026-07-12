# P5 Log Field Standard

## 统一字段
request_id, trace_id, span_id, tenant_id, shop_id, user_id_hash, session_id_hash, task_id, execution_id, module, operation, resource_type, resource_id_hash, provider, platform, result, error_code, retryable, duration_ms

## 禁止字段
password, access_token, refresh_token, authorization, cookie, app_secret, api_key, full_phone, full_email, signed_url, raw_payload

业务 ID 使用 safe_hash；不得作为 Metrics Label。
