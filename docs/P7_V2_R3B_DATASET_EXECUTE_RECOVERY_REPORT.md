# P7-V2-R3B Dataset Execute Recovery Report

Status: **failed**

Fail-fast stopped at formal regression. No soak, demo, stability, race, cleanup, final gates, tag, or production readiness declaration was executed.

## Failure

- failedStage: regression
- failedStep: formal-regression-v3
- failedCommand: pnpm p7-v2:r3b:regression
- exitCode: 1
- failedMetricCount: 1
- failed metric: Webhook Ingestion / p99
- baselineValue: 25.18280538000002 ms
- currentValue: 29.425643459999993 ms
- absoluteDelta: 4.242838079999974 ms
- relativeDelta: 0.16848154985026415
- finalVerdict: failed_material_regression

## Preserved Evidence

- runtimeFreezeId: a39c1f26e709d612670525759d0d2badc9d54f6c508d98d0965b2a919a95d53b
- runtimeFreezeStillValid: true
- immutableMismatchFields: none
- baselineRunId: p7v2-baseline-r3b-recovery6-20260715153726
- currentRunId: p7v2-current-r3b-recovery6-20260715153726
- soakRunId: p7v2-soak-r3b-recovery6-20260715153726
- demoRun1Id: p7v2-demo1-r3b-recovery6-20260715153726
- demoRun2Id: p7v2-demo2-r3b-recovery6-20260715153726
- baselineArtifactSha256: fa45980de02afe107c765207ab843f3d2c09ddde9ff02bce9336226c5b30f8f6
- currentArtifactSha256: 32924fa596c9d4270497fb8fcb6475a7ce9273b5bae6e24837daf7ef19a05d00
- comparability: passed, mismatchCount=0, notComparableCount=0

## Dataset

- datasetMode: formal_execute
- executeRequested: true
- dryRun: false
- expectedRows: 1900150
- actualRows: 1900150
- rowCountMatch: true
- databaseIdentityVerified: true
- datasetFingerprint: a3a77845644ba799a439d1f45195039bfcbeecb902afc842d7ab8da195e7e50d

## Cleanup Preflight

- cleanupGateSemanticsValid: true
- currentFormalResidualCount: 2
- failedAttemptResidualCount: 0
- historicalEvidenceDatabaseCount: 7
- unknownDatabaseCount: 0
- cleanupCodeChangeRequired: false
- cleanupExecuted: false

## Not Executed

soak, demo-preflight, demo-run-1, demo-run-2, stability, race, cleanup, final gates.

## Minimum Repair Action

Investigate and repair the Webhook Ingestion p99 material regression without changing thresholds, SLOs, VUs, stages, or dataset shape. If any immutable execution input changes, create a new runtime freeze and a new formal run-id set before rerunning baseline/current/comparability/regression/soak/demo/stability/race/cleanup/final gates.
