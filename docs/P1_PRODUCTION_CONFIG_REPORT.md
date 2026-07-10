# P1 Production Config Scan Report

Generated: 2026-07-10T07:55:26.270Z

**Overall:** passed_with_warning (16 passed, 1 warnings, 0 failed)

| ID | Status | Message |
| --- | --- | --- |
| status-no-demo-tag | passed | README.md ok |
| status-p1 | passed | README.md reflects P1 |
| status-no-demo-tag | passed | docs/PROGRESS.md ok |
| status-p1 | passed | docs/PROGRESS.md reflects P1 |
| demo-seed-guard | passed | Demo seed gated by EnableDemoSeed + !production |
| config-demo-seed | passed | production validates ENABLE_DEMO_SEED |
| config-insecure-default | passed | insecure default error code present |
| storage-local-boundary | passed | local storage production boundary |
| public-base-validate | passed | ValidatePublicBase exists |
| deploy-deploy/nginx/trademind.conf | passed | deploy/nginx/trademind.conf exists |
| deploy-deploy/systemd/trademind-api.service | passed | deploy/systemd/trademind-api.service exists |
| deploy-deploy/scripts/check-readiness.sh | passed | deploy/scripts/check-readiness.sh exists |
| deploy-.env.production.example | passed | .env.production.example exists |
| nginx-fallback | passed | Admin history fallback present |
| health-live | passed | liveness route |
| gitignore-env | warning | .gitignore env patterns incomplete |
| admin-sourcemap | passed | admin build source map policy set |
