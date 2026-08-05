# P7 Database Pool And Transaction

Implemented:

- `DB_MAX_OPEN_CONNECTIONS`
- `DB_MAX_IDLE_CONNECTIONS`
- `DB_CONN_MAX_LIFETIME_SECONDS`
- `DB_CONN_MAX_IDLE_TIME_SECONDS`
- `DB_QUERY_TIMEOUT_MS`
- `DB_TRANSACTION_TIMEOUT_MS`

`database.Open` applies pool limits. Production validation fails fast for invalid bounds.

Remaining closure: instrument query/transaction timeout wrappers across repositories, run connection leak tests, and record DB wait metrics under load.
