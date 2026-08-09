# P5.1 Database Observability

Implemented code-level pieces:

- `StartDBStatsCollector` reads `sql.DB.Stats()` and exports pool metrics.
- `InstrumentedDB` wraps `QueryContext`, `QueryRowContext`, `ExecContext`, `BeginTx`, commit, and rollback.
- Slow query logging records only `operation`, `table_group`, and duration.
- Tests cover stats collection, query duration, and rollback metrics.

Labels stay low-cardinality: `db_role`, `operation`, `table_group`, `result`, and `driver`. SQL text and SQL parameters are not metric labels and are not logged by the wrapper.
