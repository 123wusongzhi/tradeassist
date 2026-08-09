# P6 Release Architecture

Release chain:

`Source -> Build -> Test -> Artifact -> Manifest -> Dependency Manifest -> Checksum -> Preflight -> Pre-Migration Backup -> Migration -> Deploy Candidate -> Readiness -> Traffic Switch -> Smoke -> Complete / Rollback`

Implementation:

- Module: `backend/internal/modules/release`.
- Tables: `release_runs`, `release_artifacts`, `release_steps`, `release_rollbacks`.
- API: `/api/v1/ops/releases`.
- Strategy config: `RELEASE_STRATEGY=in_place|rolling|blue_green`.

P6 does not perform real production traffic switching.

