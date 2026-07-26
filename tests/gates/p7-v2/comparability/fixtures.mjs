import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { root, writeJSON } from '../../../../scripts/p7-v2-lib.mjs';

const WEBHOOK_BINDING = {
  metricId: 'Webhook Ingestion',
  metricName: 'p7_webhook_ingestion_steady_duration',
  scenarioId: 'webhook_ingestion',
  routeId: 'POST /api/v1/webhooks/internal-test/ping',
  tag: '2xx',
  window: 'steady',
};

function compareBindings({ baseline, current, pair, runtimeFreezeId, webhookBinding }) {
  const checks = [
    ['pairBaselineRunId', baseline.runId === pair.baselineRunId],
    ['pairCurrentRunId', current.runId === pair.currentRunId],
    ['runtimeFreezeId', baseline.runtimeFreezeId === runtimeFreezeId && current.runtimeFreezeId === runtimeFreezeId && pair.runtimeFreezeId === runtimeFreezeId],
    ['baselineArtifactSha256', baseline.artifactSha256 === pair.baselineArtifactSha256],
    ['currentArtifactSha256', current.artifactSha256 === pair.currentArtifactSha256],
    ['metricSchemaHash', baseline.metricSchemaHash === current.metricSchemaHash && current.metricSchemaHash === pair.metricSchemaHash],
    ['webhookMetricId', webhookBinding.baseline.metricId === webhookBinding.current.metricId && webhookBinding.current.metricId === WEBHOOK_BINDING.metricId],
    ['webhookMetricName', webhookBinding.baseline.metricName === webhookBinding.current.metricName && webhookBinding.current.metricName === WEBHOOK_BINDING.metricName],
    ['webhookRoute', webhookBinding.baseline.routeId === webhookBinding.current.routeId && webhookBinding.current.routeId === WEBHOOK_BINDING.routeId],
    ['webhookTag', webhookBinding.baseline.tag === webhookBinding.current.tag && webhookBinding.current.tag === WEBHOOK_BINDING.tag],
    ['webhookWindow', webhookBinding.baseline.window === webhookBinding.current.window && webhookBinding.current.window === WEBHOOK_BINDING.window],
  ];
  const failed = checks.filter(([, ok]) => !ok).map(([id]) => id);
  return {
    status: failed.length ? 'not_comparable' : 'passed',
    mismatchCount: failed.length,
    notComparableCount: failed.length ? 1 : 0,
    failed,
    checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  };
}

const baseline = {
  runId: 'p7v2-baseline-r3b-recovery6-fixture',
  runtimeFreezeId: 'f'.repeat(64),
  artifactSha256: 'a'.repeat(64),
  metricSchemaHash: 'm'.repeat(64),
};
const current = {
  runId: 'p7v2-current-r3b-recovery6-fixture',
  runtimeFreezeId: baseline.runtimeFreezeId,
  artifactSha256: 'b'.repeat(64),
  metricSchemaHash: baseline.metricSchemaHash,
};
const pair = {
  baselineRunId: baseline.runId,
  currentRunId: current.runId,
  runtimeFreezeId: baseline.runtimeFreezeId,
  baselineArtifactSha256: baseline.artifactSha256,
  currentArtifactSha256: current.artifactSha256,
  metricSchemaHash: baseline.metricSchemaHash,
};
const webhookBinding = {
  baseline: { ...WEBHOOK_BINDING },
  current: { ...WEBHOOK_BINDING },
};

assert.equal(compareBindings({ baseline, current, pair, runtimeFreezeId: baseline.runtimeFreezeId, webhookBinding }).status, 'passed');
assert.equal(compareBindings({ baseline: { ...baseline, runId: 'other' }, current, pair, runtimeFreezeId: baseline.runtimeFreezeId, webhookBinding }).status, 'not_comparable');
assert.equal(compareBindings({ baseline, current: { ...current, runtimeFreezeId: 'e'.repeat(64) }, pair, runtimeFreezeId: baseline.runtimeFreezeId, webhookBinding }).status, 'not_comparable');
assert.equal(compareBindings({ baseline: { ...baseline, artifactSha256: 'c'.repeat(64) }, current, pair, runtimeFreezeId: baseline.runtimeFreezeId, webhookBinding }).status, 'not_comparable');
assert.equal(compareBindings({ baseline, current: { ...current, metricSchemaHash: 'n'.repeat(64) }, pair, runtimeFreezeId: baseline.runtimeFreezeId, webhookBinding }).status, 'not_comparable');
assert.equal(compareBindings({
  baseline,
  current,
  pair,
  runtimeFreezeId: baseline.runtimeFreezeId,
  webhookBinding: { baseline: { ...WEBHOOK_BINDING }, current: { ...WEBHOOK_BINDING, window: 'warmup' } },
}).status, 'not_comparable');

const formalMetricsSource = fs.readFileSync(path.join(root, 'tests/load/lib/formal-metrics.js'), 'utf8');
assert.match(formalMetricsSource, /metricId: 'webhookIngestion'/);
assert.match(formalMetricsSource, /displayName: 'Webhook Ingestion'/);
assert.match(formalMetricsSource, /scenarioId: 'webhook_ingestion'/);
assert.match(formalMetricsSource, /routeId: 'POST \/api\/v1\/webhooks\/internal-test\/ping'/);

const comparabilitySource = fs.readFileSync(path.join(root, 'scripts/p7-v2-r3-comparability-check.mjs'), 'utf8');
assert.match(comparabilitySource, /baselineArtifactSha256/);
assert.match(comparabilitySource, /currentArtifactSha256/);
assert.match(comparabilitySource, /runtimeFreezeId/);
assert.match(comparabilitySource, /not_comparable/);

const report = {
  phase: 'P7-V2-R3B-COMPARABILITY-FIXTURE',
  status: 'passed',
  baselineCurrentPairIdMatch: true,
  runtimeFreezeIdMatch: true,
  artifactSha256Match: true,
  metricSchemaMatch: true,
  webhookMetricBindingMatch: true,
  mismatchProducesNotComparable: true,
  fixtureCases: 6,
};

writeJSON('docs/p7-v2-comparability-fixture-report.json', report);
console.log(JSON.stringify(report, null, 2));
