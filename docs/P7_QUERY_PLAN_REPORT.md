# P7 Query Plan Report

Status: not_executed.

Required command shape for isolated PostgreSQL only:

```sql
EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) ...
```

No real query-plan evidence was generated in this pass. Do not mark Database Query Performance Ready until this report contains execution time, planning time, rows scanned, buffer reads/hits, scan type and sort spill status for core queries.
