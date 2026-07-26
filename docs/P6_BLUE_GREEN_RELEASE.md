# P6 Blue/Green Release

P6 provides the foundation for controlled release:

- current and previous link hashes are recorded.
- release states are transactionally persisted.
- readiness and smoke steps are represented as release steps.
- failure can trigger application rollback.

Traffic switching remains simulated or manually controlled in P6. Real production traffic switching is Deferred.

