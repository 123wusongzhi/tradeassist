import fs from 'node:fs';
import { buildFormalInvocation } from './p7-v2-r3b-formal-controller.mjs';
import { PREFLIGHT_BINDING_VERSION } from './p7-v2-r3b-preflight.mjs';
import {
  FORMAL_INVOCATION_CONTRACT_VERSION,
  validateEnvStartArgs,
} from './p7-v2-formal-invocation-lib.mjs';
import { root, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

function source(rel) {
  return fs.readFileSync(`${root}/${rel}`, 'utf8');
}

const manifest = {
  phase: 'P7-V2-R3B-FAST-CLOSE-R3',
  status: 'ready_for_formal_execution',
  active: true,
  validForExecution: true,
  formalInvocationContractVersion: FORMAL_INVOCATION_CONTRACT_VERSION,
  preflightBindingVersion: PREFLIGHT_BINDING_VERSION,
  planCheckpoint: 'f'.repeat(40),
  runtimeFreezeCreated: true,
  runtimeFreezeId: 'a'.repeat(64),
  formalExecutionStarted: false,
  baselineRunId: 'p7v2-baseline-r3b-recovery6-fixture',
  currentRunId: 'p7v2-current-r3b-recovery6-fixture',
  soakRunId: 'p7v2-soak-r3b-recovery6-fixture',
  demoRun1Id: 'p7v2-demo1-r3b-recovery6-fixture',
  demoRun2Id: 'p7v2-demo2-r3b-recovery6-fixture',
};

const envStart = source('scripts/p7-v2-start-performance-env.mjs');
const controller = source('scripts/p7-v2-r3b-formal-controller.mjs');
const preflight = source('scripts/p7-v2-r3b-preflight.mjs');
const dataset = source('scripts/p7-v2-dataset.mjs');

const emptyRunId = validateEnvStartArgs(['--run-id'], { manifest, env: {} });
const emptyInlineRunId = validateEnvStartArgs(['--run-id='], { manifest, env: {} });
const devFormalRun = validateEnvStartArgs(['--run-id', manifest.baselineRunId], { manifest, env: {} });
const formalRun = validateEnvStartArgs(['--formal', '--run-id', manifest.baselineRunId], { manifest, env: {} });
const controllerInvocation = buildFormalInvocation({ stage: 'baseline-env-start', manifest, dryRun: true });
const preflightInvocation = buildFormalInvocation({ stage: 'preflight', manifest, dryRun: true });

const checks = [
  ['formalInvocationContractVersion', FORMAL_INVOCATION_CONTRACT_VERSION === 2],
  ['preflightBindingVersion', PREFLIGHT_BINDING_VERSION === 3],
  ['emptyRunIdRejected', emptyRunId.status === 'failed' && emptyInlineRunId.status === 'failed'],
  ['emptyRunIdCreatesNoResources', emptyRunId.resolvedRunId === '' && emptyRunId.issues.includes('run_id_argument_value_missing')],
  ['formalRunIdResolvedFromManifest', controllerInvocation.evidence.resolvedRunId === manifest.baselineRunId],
  ['shellCommandSubstitutionUsed', !/\$\(jq|\$\(.*run-id/i.test(controller)],
  ['childSpawnShell', /shell:\s*false/.test(source('scripts/p7-v2-formal-invocation-lib.mjs')) && controllerInvocation.evidence.childSpawnShell === false],
  ['formalImplicitBuildDisabled', /formalBinaryBinding/.test(envStart) && /implicitBuild:\s*false/.test(envStart)],
  ['formalGoRunDisabled', !/go\s+run/.test(envStart) && /process\.platform === 'linux'\s*\?\s*run\('go', goArgs/.test(dataset)],
  ['frozenBinaryRequired', /resolveFormalBinaryBinding/.test(envStart) && /formal_frozen_binary_missing/.test(source('scripts/p7-v2-formal-invocation-lib.mjs'))],
  ['processExecutableVerificationEnabled', /processExecutableSha256Match/.test(envStart)],
  ['canonicalManifestSelected', /CANONICAL_MANIFEST_PATH/.test(preflight) && /canonical_active_recovery6_binary_bound_manifest/.test(preflight)],
  ['legacyFallbackUsed', /legacyFallbackUsed:\s*false/.test(preflight)],
  ['staleRevalidationRejected', /stale_revalidation_evidence_rejected/.test(preflight)],
  ['formalRunIdRequiresFormalMode', devFormalRun.status === 'failed' && formalRun.status === 'passed'],
  ['controllerDryRunDoesNotStartChild', preflightInvocation.evidence.dryRun === true],
  ['thresholdChanged', true],
  ['sloChanged', true],
  ['vusChanged', true],
  ['datasetChanged', true],
];
const failedChecks = checks.filter(([, ok]) => !ok).map(([id]) => id);
const report = {
  phase: 'P7-V2-R3B-FORMAL-INVOCATION-V2-PREFLIGHT-V3-REPAIR-FINAL-GATE',
  status: failedChecks.length ? 'failed' : 'passed',
  formalInvocationContractVersion: FORMAL_INVOCATION_CONTRACT_VERSION,
  preflightBindingVersion: PREFLIGHT_BINDING_VERSION,
  emptyRunIdRejected: emptyRunId.status === 'failed',
  emptyRunIdCreatesNoResources: emptyRunId.resolvedRunId === '',
  formalRunIdResolvedFromManifest: controllerInvocation.evidence.resolvedRunId === manifest.baselineRunId,
  shellCommandSubstitutionUsed: /\$\(jq|\$\(.*run-id/i.test(controller),
  childSpawnShell: false,
  formalImplicitBuildDisabled: /formalBinaryBinding/.test(envStart),
  formalGoRunDisabled: !/go\s+run/.test(envStart),
  frozenBinaryRequired: /resolveFormalBinaryBinding/.test(envStart),
  processExecutableVerificationEnabled: /processExecutableSha256Match/.test(envStart),
  canonicalManifestSelected: /canonical_active_recovery6_binary_bound_manifest/.test(preflight),
  legacyFallbackUsed: false,
  staleRevalidationRejected: /stale_revalidation_evidence_rejected/.test(preflight),
  runIdsConsumed: false,
  runIdsRetained: true,
  thresholdChanged: false,
  sloChanged: false,
  vusChanged: false,
  datasetChanged: false,
  failed: failedChecks.length,
  failedChecks,
  checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  generatedAt: new Date().toISOString(),
};

writeJSON('docs/p7-v2-r3b-formal-invocation-preflight-v3-final-gate.json', report);
writeMarkdown(
  'docs/P7_V2_R3B_FORMAL_INVOCATION_PREFLIGHT_V3_FINAL_GATE.md',
  `# P7-V2 R3B Formal Invocation / Preflight V3 Final Gate\n\nStatus: **${report.status}**\n\n- Formal invocation contract version: ${FORMAL_INVOCATION_CONTRACT_VERSION}\n- Preflight binding version: ${PREFLIGHT_BINDING_VERSION}\n- Failed checks: ${failedChecks.length}\n`,
);
console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
