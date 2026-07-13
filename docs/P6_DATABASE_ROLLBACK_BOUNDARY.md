# P6 Database Rollback Boundary

Application rollback and database recovery are separate.

Application rollback:

- new app version returns to previous app version
- allowed only if old app remains schema-compatible

Database recovery:

- used for corruption or irreversible migration failure
- requires verified backup or PITR
- requires isolated target or explicit human approval
- remains high-risk and manual in P6

