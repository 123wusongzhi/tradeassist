# P1 Production Config Scan Report

Generated: 2026-07-11T09:56:30.694Z

**Overall:** passed (18 passed, 0 warnings, 0 failed)

| ID | Status | Message |
| --- | --- | --- |
| status-no-demo-tag | passed | README.md ok |
| status-p1 | passed | README.md reflects P1 |
| status-no-demo-tag | passed | docs/PROGRESS.md ok |
| status-p1 | passed | docs/PROGRESS.md reflects P1 |
| demo-seed-guard | passed | Demo seed gated by EnableDemoSeed + !production |
| config-demo-seed | passed | production validates ENABLE_DEMO_SEED |
| config-insecure-default | passed | insecure default error code present |
| storage-validate-failfast | passed | STORAGE_PROVIDER fail-fast in Validate() |
| storage-local-boundary | passed | local storage production boundary |
| public-base-validate | passed | ValidatePublicBase exists |
| deploy-deploy/nginx/trademind.conf | passed | deploy/nginx/trademind.conf exists |
| deploy-deploy/systemd/trademind-api.service | passed | deploy/systemd/trademind-api.service exists |
| deploy-deploy/scripts/check-readiness.sh | passed | deploy/scripts/check-readiness.sh exists |
| deploy-.env.production.example | passed | .env.production.example exists |
| nginx-fallback | passed | Admin history fallback present |
| health-live | passed | liveness route |
| gitignore-env | passed | .gitignore covers production env |
| admin-sourcemap | passed | admin build source map policy set |
