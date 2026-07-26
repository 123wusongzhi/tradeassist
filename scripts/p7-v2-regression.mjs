import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { readJSON, root, valueOf, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';
import { CORE_SCENARIOS, METRIC_METADATA, SCENARIO_METRICS } from './p7-v2-regression-metrics.mjs';
import { resolveActiveBaseline, resolveActiveCurrent, resolveFormalPairEvidence } from './p7-v2-evidence-resolver.mjs';
import { supportedLoadProfileFingerprintVersions, validateLoadProfileFingerprintEvidence } from './p7-v2-regression-fingerprint.mjs';

const args = process.argv.slice(2);
const fingerprintVersion = Number(valueOf(args, '--fingerprint-version') || 1);
if (!supportedLoadProfileFingerprintVersions.includes(fingerprintVersion)) throw new Error('fingerprint version must be 1, 2, or 3');
const resolvedBaseline = resolveActiveBaseline();
const resolvedCurrent = resolveActiveCurrent();
const formalPair = fingerprintVersion === 3 ? resolveFormalPairEvidence({ requireFrozen: true, requireComparability: true }) : null;
const baselinePath = valueOf(args, '--baseline') || (fingerprintVersion === 3 ? (formalPair.baselineRegistryEntry?.reportPath || `docs/baselines/p7-v2-baseline-${formalPair.baselineRunId}.json`) : resolvedBaseline.reportPath);
const currentPath = valueOf(args, '--current') || (fingerprintVersion === 3 ? `docs/currents/p7-v2-current-${formalPair.currentRunId}.json` : 'docs/p7-v2-current-load-report.json');
const policyPath = valueOf(args, '--policy') || 'docs/p7-v2-regression-policy-v2.json';
const baseline = fingerprintVersion === 3 ? { ...(readJSON(baselinePath) || {}), ...(formalPair.baselineRegistryEntry || {}) } : (valueOf(args, '--baseline') ? readJSON(baselinePath) : resolvedBaseline.baseline);
const current = fingerprintVersion === 3 ? { ...(readJSON(currentPath) || {}), ...(formalPair.currentRegistryEntry || {}) } : { ...(readJSON(currentPath) || {}), ...(resolvedCurrent.entry || {}) };
const policy = readJSON(policyPath);
const comparabilityPath = valueOf(args, '--comparability-report') || (fingerprintVersion === 3
  ? 'docs/p7-v2-r3b-fast-close-r3-comparability-report.json'
  : fingerprintVersion === 2
  ? 'docs/p7-v2-r3b-lpf-comparability-v2-report.json'
  : 'docs/p7-v2-r3b-rebaseline2-comparability-report.json');
const comparability = readJSON(comparabilityPath) || {};
const hash = (data) => crypto.createHash('sha256').update(data).digest('hex');
const policyFingerprint = policy ? hash(JSON.stringify(policy)) : '';
const issues = [];
const fingerprintEvidence = validateLoadProfileFingerprintEvidence(baseline, current);
if (!fingerprintEvidence.regressionAllowed) issues.push(fingerprintEvidence.classification);

function frozen(kind, runId) {
  const group = kind === 'baseline' ? 'baselines' : 'currents';
  const dir = path.join(root, 'docs', group, 'frozen', runId);
  const manifest = readJSON(path.relative(root, path.join(dir, 'manifest.json')));
  const rawPath = path.join(dir, manifest?.rawArtifact?.relativePath || (manifest?.frozenPath ? path.basename(manifest.frozenPath) : '') || 'raw-summary.json');
  if (!manifest || !fs.existsSync(rawPath)) return { valid: false, issues: [`${kind} frozen raw artifact is missing`] };
  const raw = fs.readFileSync(rawPath);
  let json;
  try { json = JSON.parse(raw.toString('utf8')); } catch { return { valid: false, issues: [`${kind} frozen raw artifact is invalid JSON`] }; }
  const actualHash = hash(raw);
  const requests = Number(json?.metrics?.http_reqs?.values?.count ?? json?.metrics?.http_reqs?.count ?? 0);
  const covered = CORE_SCENARIOS.every((name) => {
    const [, requestMetric] = SCENARIO_METRICS[name];
    return Number(json?.metrics?.[requestMetric]?.values?.count ?? json?.metrics?.[requestMetric]?.count ?? 0) > 0;
  });
  const expectedHash = manifest?.rawArtifact?.sha256 || manifest.sha256;
  const expectedSize = manifest?.rawArtifact?.sizeBytes ?? manifest.sizeBytes;
  const valid = manifest.runId === runId && manifest.runKind === kind && manifest.immutable === true &&
    expectedHash === actualHash && Number(expectedSize) === raw.length && requests > 0 && covered;
  return { valid, issues: valid ? [] : [`${kind} frozen raw artifact verification failed`], manifest, raw: json, actualHash, requests };
}

const baselineFrozen = baseline?.runId ? frozen('baseline', baseline.runId) : { valid: false, issues: ['baseline run ID is missing'] };
const currentFrozen = current?.runId ? frozen('current', current.runId) : { valid: false, issues: ['current run ID is missing'] };
issues.push(...baselineFrozen.issues, ...currentFrozen.issues);
if (fingerprintVersion === 3 && !formalPair.regressionAllowed) issues.push(formalPair.classification);
if (fingerprintVersion !== 3 && (!resolvedBaseline.valid || !resolvedCurrent.valid)) issues.push('active frozen baseline or Current registry entry is invalid');
if (!baseline || baseline.status !== 'passed' || !current || current.status !== 'passed' || current.independentRun !== true) issues.push('baseline or independent Current report is not passed');
if (!policy || policy.version !== 2) issues.push('Regression policy version 2 is required');
if (comparability.status !== 'passed' || (fingerprintVersion >= 2 && comparability.currentFingerprintVersion !== fingerprintVersion)) {
  issues.push('passed Comparability evidence for the selected fingerprint version is required');
}
if (fingerprintVersion === 3) {
  if (comparability.baselineRunId !== baseline.runId || comparability.currentRunId !== current.runId) issues.push('comparability_pair_binding_mismatch');
  if (formalPair.selectedBaselineArtifactSha256 && comparability.baselineArtifactSha256 !== formalPair.selectedBaselineArtifactSha256) issues.push('comparability_artifact_hash_binding_mismatch');
  if (formalPair.selectedCurrentArtifactSha256 && comparability.currentArtifactSha256 !== formalPair.selectedCurrentArtifactSha256) issues.push('comparability_artifact_hash_binding_mismatch');
  if (formalPair.runtimeFreezeId && comparability.runtimeFreezeId !== formalPair.runtimeFreezeId) issues.push('comparability_pair_binding_mismatch');
}
if (baseline?.runId === current?.runId || baselineFrozen.actualHash === currentFrozen.actualHash) issues.push('baseline and current artifacts are not independent');

function reportScenarioValue(run, scenario, metric) {
  const row = (run?.scenarios || []).find((item) => item.scenario === scenario);
  return row?.[metric];
}

function metricValue(raw, run, scenario, metric) {
  const [durationMetric, requestMetric] = SCENARIO_METRICS[scenario];
  const values = raw?.metrics?.[durationMetric]?.values || raw?.metrics?.[durationMetric] || {};
  const requestValues = raw?.metrics?.[requestMetric]?.values || raw?.metrics?.[requestMetric] || {};
  if (metric === 'p95') return values['p(95)'];
  if (metric === 'p99') return values['p(99)'];
  if (metric === 'rps') return requestValues.rate ?? reportScenarioValue(run, scenario, metric);
  if (metric === 'errorRate') return raw?.metrics?.http_req_failed?.values?.rate ?? raw?.metrics?.http_req_failed?.rate ?? reportScenarioValue(run, scenario, metric);
  if (metric === 'timeouts') return raw?.metrics?.p7_timeouts?.values?.count ?? raw?.metrics?.p7_timeouts?.count ?? reportScenarioValue(run, scenario, metric);
  return undefined;
}

function compare(scenario, metric) {
  const metadata = METRIC_METADATA[metric];
  const baselineValue = metricValue(baselineFrozen.raw, baseline, scenario, metric);
  const currentValue = metricValue(currentFrozen.raw, current, scenario, metric);
  const baselineSamples = Number(baselineFrozen.raw?.metrics?.[SCENARIO_METRICS[scenario][1]]?.values?.count ?? baselineFrozen.raw?.metrics?.[SCENARIO_METRICS[scenario][1]]?.count ?? 0);
  const currentSamples = Number(currentFrozen.raw?.metrics?.[SCENARIO_METRICS[scenario][1]]?.values?.count ?? currentFrozen.raw?.metrics?.[SCENARIO_METRICS[scenario][1]]?.count ?? 0);
  const basePresent = baselineValue !== null && baselineValue !== undefined;
  const currentPresent = currentValue !== null && currentValue !== undefined;
  const common = { scenario, metric, metricFamily: metadata.metricFamily, direction: metadata.direction, unit: metadata.unit,
    baselinePresent: basePresent, baselineValue: basePresent ? Number(baselineValue) : null, baselineSampleCount: baselineSamples,
    currentPresent, currentValue: currentPresent ? Number(currentValue) : null, currentSampleCount: currentSamples,
    relativeThreshold: metadata.relativeThreshold, materialityFloor: metadata.materialityFloor, absoluteSlo: current?.absoluteSloPassed === true };
  if (!basePresent || !currentPresent) {
    const reason = metric === 'p99' ? 'summary_stat_missing' : 'missing_metric';
    return { ...common, finalVerdict: metric === 'p99' ? 'summary_stat_missing' : 'not_comparable', reason, missingSide: !basePresent && !currentPresent ? 'both' : !basePresent ? 'baseline' : 'current' };
  }
  if (!Number.isFinite(Number(baselineValue)) || !Number.isFinite(Number(currentValue))) return { ...common, finalVerdict: 'invalid_metric', reason: 'non_finite_value' };
  if (baselineSamples < metadata.minimumSampleCount || currentSamples < metadata.minimumSampleCount) return { ...common, finalVerdict: 'insufficient_samples', reason: 'minimum_sample_count_not_met' };
  const absoluteDelta = Number(currentValue) - Number(baselineValue);
  if (Number(baselineValue) === 0 && Number(currentValue) === 0 && metadata.zeroPolicy === 'valid_value') return { ...common, absoluteDelta, relativeDelta: 0, finalVerdict: 'passed_no_change_zero_to_zero', reason: 'explicit_zero_semantics' };
  if (Number(baselineValue) === 0 && metadata.zeroPolicy === 'invalid_when_zero') return { ...common, absoluteDelta, relativeDelta: null, finalVerdict: 'invalid_metric', reason: 'zero_is_not_valid_for_metric' };
  if (Number(baselineValue) > 0 && Number(currentValue) === 0 && metadata.direction === 'lower_is_better') return { ...common, absoluteDelta, relativeDelta: -1, finalVerdict: 'passed_improved_to_zero', reason: 'directional_improvement' };
  const relativeDelta = metadata.direction === 'higher_is_better'
    ? (Number(baselineValue) - Number(currentValue)) / Number(baselineValue)
    : absoluteDelta / Number(baselineValue);
  if (current?.absoluteSloPassed !== true) return { ...common, absoluteDelta, relativeDelta, finalVerdict: 'failed_absolute_slo', reason: 'absolute_slo_failed' };
  if (metadata.comparisonMode === 'absolute_budget') {
    const budget = metadata.materialityFloor?.value ?? 0;
    return { ...common, absoluteDelta, relativeDelta, finalVerdict: absoluteDelta <= budget ? 'passed' : 'failed_absolute_budget', reason: absoluteDelta <= budget ? 'within_absolute_budget' : 'absolute_budget_exceeded' };
  }
  const relativeExceeded = relativeDelta > metadata.relativeThreshold;
  const materialityExceeded = metadata.materialityFloor ? absoluteDelta > metadata.materialityFloor.value : true;
  const finalVerdict = relativeExceeded && materialityExceeded ? 'failed_material_regression' : relativeExceeded ? 'passed_relative_noise_below_materiality_floor' : 'passed';
  return { ...common, absoluteDelta, relativeDelta, relativeExceeded, materialityExceeded, finalVerdict, reason: finalVerdict };
}

const comparisons = baselineFrozen.valid && currentFrozen.valid ? CORE_SCENARIOS.flatMap((scenario) => Object.keys(METRIC_METADATA).map((metric) => compare(scenario, metric))) : [];
const failedMetricCount = comparisons.filter((item) => item.finalVerdict.startsWith('failed')).length;
const notComparableCount = comparisons.filter((item) => item.finalVerdict === 'not_comparable').length;
const invalidMetricCount = comparisons.filter((item) => item.finalVerdict === 'invalid_metric').length;
const insufficientSampleCount = comparisons.filter((item) => item.finalVerdict === 'insufficient_samples').length;
const summaryStatMissingCount = comparisons.filter((item) => item.finalVerdict === 'summary_stat_missing').length;
const status = issues.length || failedMetricCount || notComparableCount || invalidMetricCount || insufficientSampleCount || summaryStatMissingCount ? 'failed' : 'passed';
const report = { phase: fingerprintVersion === 3 ? 'P7-V2-R3B-FAST-CLOSE-R3-FORMAL' : fingerprintVersion === 2 ? 'P7-V2-R3B-LPF-V2' : 'P7-V2-R3B-REBASELINE2', status, evaluationVersion: 2, policyVersion: policy?.version || 0, policyFingerprint,
  baseline: { path: baselinePath, runId: baseline?.runId || '', artifactSha256: baselineFrozen.actualHash || '', artifactHashVerified: baselineFrozen.valid },
  current: { path: currentPath, runId: current?.runId || '', artifactSha256: currentFrozen.actualHash || '', artifactHashVerified: currentFrozen.valid, independentRun: current?.independentRun === true },
  absoluteSloPassed: current?.absoluteSloPassed === true, relativeRegressionPassed: failedMetricCount === 0 && notComparableCount === 0,
  materialityGatePassed: failedMetricCount === 0, failedMetricCount, notComparableCount, invalidMetricCount, insufficientSampleCount,
  summaryStatMissingCount, zeroSemanticErrors: 0, fingerprintVersion, supportedLoadProfileFingerprintVersions, fingerprintEvidence, formalPairEvidence: fingerprintVersion === 3 ? formalPair : undefined, comparabilityPath, comparisons, issues };
if (fingerprintVersion === 1 && !fs.existsSync(path.join(root, 'docs/regressions/p7-v2-r3b-regression-v1-failed.json'))) {
  const previous = readJSON('docs/p7-v2-performance-regression-report.json');
  if (previous) writeJSON('docs/regressions/p7-v2-r3b-regression-v1-failed.json', { ...previous, evaluationVersion: 1 });
}
const output = fingerprintVersion === 3
  ? ['docs/p7-v2-r3b-fast-close-r3-regression-v2-report.json', 'docs/P7_V2_R3B_FAST_CLOSE_R3_REGRESSION_V2_REPORT.md', 'P7-V2-R3B-FAST-CLOSE-R3-FORMAL Regression V2']
  : fingerprintVersion === 2
  ? ['docs/p7-v2-r3b-lpf-regression-v2-report.json', 'docs/P7_V2_R3B_LPF_REGRESSION_V2_REPORT.md', 'P7-V2-R3B-LPF-V2 Regression V2']
  : ['docs/p7-v2-r3b-rebaseline2-regression-v2-report.json', 'docs/P7_V2_R3B_REBASELINE2_REGRESSION_V2_REPORT.md', 'P7-V2-R3B-REBASELINE2 Regression V2'];
writeJSON(output[0], report);
writeMarkdown(output[1], `# ${output[2]}\n\nStatus: **${status}**\n\n- Evaluation version: 2\n- Failed metrics: ${failedMetricCount}\n- Not comparable: ${notComparableCount}\n- Invalid metrics: ${invalidMetricCount}\n- Insufficient samples: ${insufficientSampleCount}\n\n## Issues\n${issues.length ? issues.map((item) => `- ${item}`).join('\n') : '- none'}\n`);
console.log(JSON.stringify({ phase: report.phase, status, failedMetricCount, notComparableCount, invalidMetricCount, insufficientSampleCount, summaryStatMissingCount }, null, 2));
process.exit(status === 'passed' ? 0 : 1);
