# P7-V2-R2 Route Credential Matrix

Status: passed

| Route | Role | Expected |
| --- | --- | ---: |
| Product List | tenant_admin | 200 |
| Order List | tenant_admin | 200 |
| Inventory List | operator | 200 |
| Task List | operator | 200 |
| Webhook Event List | tenant_admin | 200 |
| Operation Log List | system_admin | 200 |
| Webhook Ingestion | none | 200 |
| Login | none | 200 |
| Refresh | tenant_admin | 401 |
| Health Live | none | 200 |
