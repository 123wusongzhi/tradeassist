# P7-V2 Performance Regression Report

Status: **failed**

| Scenario | Metric | Baseline | Current | Delta % | Status |
| --- | --- | ---: | ---: | ---: | --- |
| Product List | p95 | 2.4256135999999997 | 2.4447661000000003 | 0.79 | passed |
| Product List | p99 | 0 | 0 | n/a | not_comparable |
| Product List | rps | 3.8900813571078965 | 3.9421841590808793 | 1.34 | passed |
| Product List | errorRate | 0 | 0 | n/a | not_comparable |
| Product List | timeouts | 0 | 0 | n/a | not_comparable |
| Order List | p95 | 2.6476643999999996 | 2.8400961999999996 | 7.27 | passed |
| Order List | p99 | 0 | 0 | n/a | not_comparable |
| Order List | rps | 3.886223680236508 | 3.9328258584065034 | 1.20 | passed |
| Order List | errorRate | 0 | 0 | n/a | not_comparable |
| Order List | timeouts | 0 | 0 | n/a | not_comparable |
| Inventory List | p95 | 3.11891225 | 3.0996734999999997 | -0.62 | passed |
| Inventory List | p99 | 0 | 0 | n/a | not_comparable |
| Inventory List | rps | 1.94118300168256 | 1.9644632832294233 | 1.20 | passed |
| Inventory List | errorRate | 0 | 0 | n/a | not_comparable |
| Inventory List | timeouts | 0 | 0 | n/a | not_comparable |
| Task List | p95 | 63.54676575 | 61.2985839 | -3.54 | passed |
| Task List | p99 | 0 | 0 | n/a | not_comparable |
| Task List | rps | 1.94118300168256 | 1.963683424839892 | 1.16 | passed |
| Task List | errorRate | 0 | 0 | n/a | not_comparable |
| Task List | timeouts | 0 | 0 | n/a | not_comparable |
| Webhook Event List | p95 | 2.43364725 | 2.162655 | -11.14 | passed |
| Webhook Event List | p99 | 0 | 0 | n/a | not_comparable |
| Webhook Event List | rps | 1.94118300168256 | 1.9621237080608294 | 1.08 | passed |
| Webhook Event List | errorRate | 0 | 0 | n/a | not_comparable |
| Webhook Event List | timeouts | 0 | 0 | n/a | not_comparable |
| Operation Log List | p95 | 3.1436995 | 4.500724799999998 | 43.17 | failed |
| Operation Log List | p99 | 0 | 0 | n/a | not_comparable |
| Operation Log List | rps | 1.937325324811172 | 1.9629035664503607 | 1.32 | passed |
| Operation Log List | errorRate | 0 | 0 | n/a | not_comparable |
| Operation Log List | timeouts | 0 | 0 | n/a | not_comparable |
| Webhook Ingestion | p95 | 17.8835675 | 16.710432 | -6.56 | passed |
| Webhook Ingestion | p99 | 0 | 0 | n/a | not_comparable |
| Webhook Ingestion | rps | 1.937325324811172 | 1.9629035664503607 | 1.32 | passed |
| Webhook Ingestion | errorRate | 0 | 0 | n/a | not_comparable |
| Webhook Ingestion | timeouts | 0 | 0 | n/a | not_comparable |
| Provider Mock Flow | p95 | 1.067946 | 1.1354857999999999 | 6.32 | passed |
| Provider Mock Flow | p99 | 0 | 0 | n/a | not_comparable |
| Provider Mock Flow | rps | 1.937325324811172 | 1.961343849671298 | 1.24 | passed |
| Provider Mock Flow | errorRate | 0 | 0 | n/a | not_comparable |
| Provider Mock Flow | timeouts | 0 | 0 | n/a | not_comparable |
| Auth/Security | p95 | 7.409594850000002 | 7.6237416999999885 | 2.89 | passed |
| Auth/Security | p99 | 0 | 0 | n/a | not_comparable |
| Auth/Security | rps | 1.8347111200322448 | 1.8560629670845683 | 1.16 | passed |
| Auth/Security | errorRate | 0 | 0 | n/a | not_comparable |
| Auth/Security | timeouts | 0 | 0 | n/a | not_comparable |

## Issues
- none
