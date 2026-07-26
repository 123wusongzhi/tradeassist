import { runtimeSourceFingerprint, jsonHash } from './p7-v2-r3-lib.mjs';
import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';
import { resolveActiveBaseline } from './p7-v2-evidence-resolver.mjs';

const baseline = resolveActiveBaseline({ verifyArtifact: false }).baseline || {};
const runtime = runtimeSourceFingerprint();
const changes = [
  ['scripts/p7-v2-evidence-resolver.mjs', 'gate_only'],
  ['scripts/p7-v2-r3b-gate-lib.mjs', 'gate_only'],
  ['scripts/p7-v2-r3b-fix-final-gate.mjs', 'gate_only'],
  ['scripts/p7-v2-r3b-dry-gate.mjs', 'gate_only'],
  ['scripts/p1-p7-final-gate.mjs', 'gate_only'],
  ['scripts/p7-v2-final-closure-gate.mjs', 'gate_only'],
  ['scripts/p7-v2-soak.mjs', 'measurement_semantics_impacting'],
  ['scripts/p7-v2-restart-environment.mjs', 'measurement_semantics_impacting'],
  ['scripts/p7-v2-current.mjs', 'measurement_semantics_impacting'],
  ['tests/load/p7v2-current.js', 'load_semantics_impacting'],
  ['docs/', 'documentation_only'],
  ['tests/gates/p7-v2/', 'gate_only'],
];
const report = {
  phase: 'P7-V2-R3B-FIX',
  currentRuntimeSourceTreeHash: runtime.hash,
  baselineRuntimeSourceTreeHash: baseline.runtimeSourceTreeHash || '',
  runtimeSourceTreeChanged: runtime.hash !== baseline.runtimeSourceTreeHash,
  loadScriptsHash: jsonHash(runtime.files.filter((file) => file.path.startsWith('tests/load/'))),
  measurementSemanticsHash: jsonHash(changes.filter(([, kind]) => kind === 'measurement_semantics_impacting')),
  gateLogicHash: jsonHash(changes.filter(([, kind]) => kind === 'gate_only')),
  datasetGeneratorHash: jsonHash(runtime.files.filter((file) => file.path === 'scripts/p7-v2-dataset.mjs')),
  backendRuntimeChanged: false,
  loadGenerationSemanticsChanged: true,
  measurementSemanticsChanged: true,
  gateOnlyChanged: true,
  datasetGeneratorChanged: true,
  SLOChanged: false,
  credentialMatrixChanged: false,
  changes: changes.map(([path, classification]) => ({ path, classification })),
  baselineReuseDecision: 'rebaseline_required',
  reason: 'The frozen raw artifact is absent, and R3B changes measurement and current-load semantic inputs; a strict comparison cannot be proven.',
};
writeJSON('docs/p7-v2-r3b-fix-fingerprint-report.json', report);
writeMarkdown('docs/P7_V2_R3B_FIX_FINGERPRINT_REPORT.md', `# P7-V2-R3B-FIX Fingerprint Report\n\nBaseline reuse decision: **${report.baselineReuseDecision}**\n\n${report.reason}\n`);
console.log(JSON.stringify(report, null, 2));
