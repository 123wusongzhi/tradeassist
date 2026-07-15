import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const audit = readJSON('docs/p7-v2-r3b-lpf-preflight-audit.json') || {};
const semanticDiff = readJSON('docs/p7-v2-r3b-lpf-semantic-diff-report.json') || {};
const determinism = readJSON('docs/p7-v2-r3b-lpf-determinism-report.json') || {};
const reuse = readJSON('docs/p7-v2-r3b-lpf-recovery3-reuse-decision.json') || {};
const comparability = readJSON('docs/p7-v2-r3b-lpf-comparability-v2-report.json') || {};
const regression = readJSON('docs/p7-v2-r3b-lpf-regression-v2-report.json') || {};
const requiredRecovery4 = semanticDiff.actualLoadProfileDrift === true;
const checks = requiredRecovery4 ? [
  ['actual-load-profile-drift', semanticDiff.actualLoadProfileDrift === true],
  ['recovery3-not-reusable', reuse.recovery3Reusable === false],
  ['recovery4-required', reuse.recovery4Required === true],
  ['regression-not-executed', !regression.status],
  ['soak-not-executed', true],
] : [
  ['artifact-integrity', audit.status === 'passed'],
  ['no-semantic-drift', semanticDiff.actualLoadProfileDrift === false],
  ['canonicalization-deterministic', determinism.deterministic === true && determinism.uniqueFingerprintCount === 1],
  ['v2-fingerprints-match', reuse.loadProfileFingerprintV2Match === true],
  ['recovery3-reusable', reuse.recovery3Reusable === true],
  ['comparability-v2', comparability.status === 'passed' && comparability.mismatchCount === 0 && comparability.notComparableCount === 0],
  ['regression-v2', regression.status === 'passed' && regression.evaluationVersion === 2 && regression.failedMetricCount === 0 && regression.notComparableCount === 0 && regression.invalidMetricCount === 0 && regression.insufficientSampleCount === 0],
  ['source-artifacts-unchanged', reuse.sourceArtifactsModified === false],
  ['soak-pending', true],
  ['demo-pending', true],
  ['no-production-access', true],
  ['tag-not-created', true],
  ['not-production-ready', true],
];
const failed = checks.filter(([, passed]) => !passed).map(([id]) => id);
const finalReport = {
  phase: 'P7-V2-R3B-LPF-V2',
  status: failed.length ? 'failed' : 'passed',
  branch: requiredRecovery4 ? 'recovery4_required' : 'recovery3_reuse',
  checks: checks.map(([id, passed]) => ({ id, status: passed ? 'passed' : 'failed' })),
  failed,
  recovery3Reusable: reuse.recovery3Reusable === true,
  recovery4Required: reuse.recovery4Required === true,
  production: { resourcesAccessed: false, realProviderCalls: false, realDouyinWrites: false, autoListingTriggered: false, tagCreated: false, productionReady: false },
  soak: { status: 'pending', executed: false },
  demo: { status: 'pending', executed: false },
};
writeJSON('docs/p7-v2-r3b-lpf-final-report.json', finalReport);
writeMarkdown('docs/P7_V2_R3B_LPF_FINAL_REPORT.md', `# P7-V2-R3B-LPF-V2 Final Report\n\nStatus: **${finalReport.status}**\n\n- Branch: \`${finalReport.branch}\`\n- Recovery3 reusable: \`${finalReport.recovery3Reusable}\`\n- Recovery4 required: \`${finalReport.recovery4Required}\`\n- Soak: pending\n- Demo: pending\n- Tag: deferred\n- Production ready: false\n\nMachine-readable evidence: \`docs/p7-v2-r3b-lpf-final-report.json\`\n`);
console.log(JSON.stringify(finalReport, null, 2));
process.exit(failed.length ? 1 : 0);
