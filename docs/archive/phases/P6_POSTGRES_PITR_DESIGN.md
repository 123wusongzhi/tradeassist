# P6 PostgreSQL PITR Design

PITR foundation includes:

- base backup strategy
- WAL archive metadata
- restore command generation
- recovery target time validation
- timeline awareness
- WAL continuity check

Code-level checks:

- future recovery target rejected
- target earlier than earliest recoverable time rejected
- WAL inventory gaps rejected
- restore command does not log storage secrets

Real production PITR drill remains Deferred until P10.

