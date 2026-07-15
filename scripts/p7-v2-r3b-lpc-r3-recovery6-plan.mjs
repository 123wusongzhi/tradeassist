import fs from 'node:fs';
import path from 'node:path';
import { readJSON, root, writeJSON } from './p7-v2-lib.mjs';

const required = [
  'docs/p7-v2-r3b-lpc-r3-preflight-audit.json',
  'docs/p7-v2-r3b-lpc-r3-determinism-report.json',
  'docs/p7-v2-r3b-lpc-r3-consumer-compatibility.json',
];
if (required.some((file) => readJSON(file)?.status !== 'passed')) throw new Error('passed LPC-R3 evidence is required before planning Recovery6');
const stamp = new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14);
const runIds = {
  baselineRunId: `p7v2-baseline-r3b-recovery6-${stamp}`, currentRunId: `p7v2-current-r3b-recovery6-${stamp}`,
  soakRunId: `p7v2-soak-r3b-recovery6-${stamp}`, demoRun1Id: `p7v2-demo1-r3b-recovery6-${stamp}`, demoRun2Id: `p7v2-demo2-r3b-recovery6-${stamp}`,
};
const allRunIds = Object.values(runIds);
if (new Set(allRunIds).size !== allRunIds.length) throw new Error('Recovery6 run IDs are not unique');
const registries = [
  readJSON('docs/baselines/p7-v2-baseline-registry.json') || {},
  readJSON('docs/currents/p7-v2-current-registry.json') || {},
  readJSON('docs/fingerprints/p7-v2/load-profile-registry.json') || {},
];
for (const runId of allRunIds) {
  if (fs.existsSync(path.join(root, 'artifacts', 'p7-v2', 'baseline', runId)) || fs.existsSync(path.join(root, 'artifacts', 'p7-v2', 'current', runId)) ||
      fs.existsSync(path.join(root, 'docs', 'baselines', 'frozen', runId)) || fs.existsSync(path.join(root, 'docs', 'currents', 'frozen', runId)) ||
      JSON.stringify(registries).includes(runId)) throw new Error(`Recovery6 run ID already exists: ${runId}`);
}
const previousPlan = {
  phase: 'P7-V2-R3B-FAST-CLOSE-R2', baselineRunId: 'p7v2-baseline-r3b-recovery5-20260715091700',
  status: 'aborted_before_execution', active: false, validForExecution: false, baselineArtifactCreated: false,
  currentExecuted: false, reason: 'canonical_load_profile_stage_validation_failed',
};
const manifest = {
  phase: 'P7-V2-R3B-FAST-CLOSE-R3', status: 'planned', canonicalSchemaVersion: 3, loadProfileFingerprintVersion: 3,
  ...runIds, selectedHost: '127.0.0.1', selectedPort: 18080, baseUrl: 'http://127.0.0.1:18080',
  runtimeFreezeId: null, executionStarted: false, runIdsUnique: true, previousPlan, updatedAt: new Date().toISOString(),
};
writeJSON('docs/p7-v2-r3b-run-manifest.json', manifest);
console.log(JSON.stringify(manifest, null, 2));
