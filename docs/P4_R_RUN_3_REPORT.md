# TradeMind Phase P4-R Demo Regression Auto Acceptance Report

> Generated: 2026-07-12T03:11:29.5687896Z
> API: http://127.0.0.1:8080 | Backend: reachable
> Run command: `AI_IMAGE_TRIAL_TIMEOUT_SECONDS=120 AI_IMAGE_POLL_INTERVAL_SECONDS=5 pnpm demo:auto-acceptance`

## Phase

**Phase P4-R** - Demo regression stabilization with isolated Go test environment (not final manual acceptance)

## Summary

| Metric | Value |
| --- | --- |
| Conclusion | **passed_with_blocked** |
| Failed steps | 0 |
| Blocked steps | 1 |
| Code failed | 0 |
| Non-AI failed | 0 |

## Step results

| Step | Category | Status | Exit | Reason | Detail |
| --- | --- | --- | --- | --- | --- |
| go test regression | code_check | passed | 0 | passed |  |
| go build backend | build | passed | 0 | passed |  |
| pnpm build:admin | build | passed | 0 | passed |  |
| git diff --check | code_check | passed | 0 | passed |  |
| check-ui-copy | code_check | passed | 0 | passed |  |
| demo-empty-state-scan | smoke | passed | 0 | passed |  |
| demo-sensitive-confirm-scan | smoke | passed | 0 | passed |  |
| security-release-check | code_check | passed | 0 | passed |  |
| check-doc-links | code_check | passed | 0 | passed |  |
| demo-route-smoke | smoke | passed | 0 | passed |  |
| seed-demo-data | seed | passed | 0 | passed |  |
| seed-demo-permissions | seed | passed | 0 | passed |  |
| demo-dashboard-smoke | smoke | passed | 0 | passed |  |
| demo-rbac-smoke | smoke | passed | 0 | passed |  |
| demo-order-inventory-customer-smoke | smoke | passed | 0 | passed |  |
| ai-text-route-smoke | smoke | passed | 0 | passed |  |
| ai-text-trial-run | external_provider | blocked | 3 | environment_blocked | blocked_by_config_or_credentials |
| ai-image-route-smoke | smoke | passed | 0 | passed |  |
| ai-image-trial-run | external_provider | warning | 5 | warning | completed_with_warning |
| publish-batch-perf | smoke | passed | 0 | passed |  |
| ai-operation-workbench-perf | smoke | passed | 0 | passed |  |

## Artifacts

- [demo-route-smoke.auto.json](demo-route-smoke.auto.json)
- [demo-dataset.auto.json](demo-dataset.auto.json)
- [ai-text-trial-run.auto.json](ai-text-trial-run.auto.json)
- [ai-image-trial-run.auto.json](ai-image-trial-run.auto.json)
- [publish-batch-perf.auto.json](publish-batch-perf.auto.json)
- [ai-operation-workbench-perf.auto.json](ai-operation-workbench-perf.auto.json)
- [COPYWRITING_AUDIT.auto.md](COPYWRITING_AUDIT.auto.md)
- [SECURITY_RELEASE_CHECK.auto.md](SECURITY_RELEASE_CHECK.auto.md)
- [DOCS_CONSISTENCY_CHECK.md](DOCS_CONSISTENCY_CHECK.md)

## Manual test checklist (out of scope for automation)

- [ ] Real preprod SSH deployment
- [ ] Nginx / HTTPS
- [ ] Storage public access
- [ ] Preprod backup and rollback
- [ ] 1366 / 1024 visual walkthrough
- [ ] Douyin real OAuth
- [ ] Douyin readonly E2E
- [ ] Douyin write E2E
- [ ] 48-72h gray observation
- [ ] Tag deferred review

## Final status

```text
Production Capability Development In Progress
MVP Demo Ready
Tag deferred
Not Production Ready
Douyin Release Candidate
```

Tag remains deferred in this phase. No real Douyin E2E. No production gray release.
