import { spawnSync } from 'node:child_process';
import { freezeRawArtifact } from './p7-v2-artifact-freeze.mjs';
import { readJSON, valueOf, writeJSON } from './p7-v2-lib.mjs';

const args = process.argv.slice(2);
const runId = valueOf(args, '--run-id') || `p7v2-current-${new Date().toISOString().replace(/[:.]/g, '-')}`;
if (!/^p7v2-current-r3b-recovery-[a-z0-9_-]+$/.test(runId)) {
  throw new Error('P7-V2-R3B-REBASELINE requires a unique recovery current run ID');
}
const restart = spawnSync(process.execPath, ['scripts/p7-v2-restart-environment.mjs', '--run-id', `${runId}-restart`], {
  stdio: 'inherit',
});
if (restart.status !== 0) process.exit(restart.status ?? 1);
const res = spawnSync(process.execPath, ['scripts/p7-v2-load.mjs', '--kind', 'current', '--run-id', runId, ...args.filter((a) => !a.startsWith('--run-id'))], {
  stdio: 'inherit',
});
const current = readJSON('docs/p7-v2-current-load-report.json');
const restartEvidence = readJSON('docs/p7-v2-environment-restart-report.json') || {};
if (current?.runId === runId) {
  current.restartEvidence = {
    restartPerformed: restartEvidence.restartPerformed === true,
    apiProcessChanged: restartEvidence.apiProcessChanged === true,
    workerProcessChanged: restartEvidence.workerProcessChanged === true,
    redisRestarted: restartEvidence.redisRestarted === true,
    mockProviderRestarted: restartEvidence.mockProviderRestarted === true,
    databaseStateReset: restartEvidence.databaseStateReset === true,
    databaseResetMethod: restartEvidence.databaseResetMethod || '',
    bootstrapPassed: restartEvidence.bootstrapPassed === true,
    authProbePassed: restartEvidence.authProbePassed === true,
    routeProbePassed: restartEvidence.routeProbePassed === true,
    datasetVerified: restartEvidence.datasetVerified === true,
    datasetFingerprint: restartEvidence.datasetFingerprint || '',
    serverReady: restartEvidence.serverReady === true,
  };
  current.currentRunIndependent =
    current.runId !== restartEvidence.runId &&
    current.restartEvidence.apiProcessChanged &&
    current.restartEvidence.databaseStateReset &&
    current.restartEvidence.bootstrapPassed &&
    current.restartEvidence.authProbePassed &&
    current.restartEvidence.routeProbePassed &&
    current.restartEvidence.datasetVerified;
  current.independentRun = current.currentRunIndependent;
  current.status = current.status === 'passed' && current.independentRun ? 'passed' : 'failed';
  writeJSON('docs/p7-v2-current-load-report.json', current);
  writeJSON(`docs/runs/p7-v2-current-${runId}.json`, current);
}
if (res.status !== 0) process.exit(res.status ?? 1);
try {
  const frozen = freezeRawArtifact({
    kind: 'current',
    runId,
    reportPath: 'docs/p7-v2-current-load-report.json',
  });
  writeJSON(`docs/currents/p7-v2-current-${runId}.json`, {
    ...current,
    immutable: true,
    rawArtifactPath: frozen.originalPath,
    rawArtifactSha256: frozen.sha256,
    rawArtifactSizeBytes: frozen.sizeBytes,
    rawArtifactHashVerified: true,
    scenarioCoverage: frozen.scenarioCoverage,
    frozenArtifactPath: frozen.frozenPath,
    frozenAt: frozen.createdAt,
  });
  console.log(JSON.stringify({ phase: 'P7-V2-R3B-REBASELINE', kind: 'current', runId, freeze: 'passed', sha256: frozen.sha256 }, null, 2));
} catch (error) {
  console.error(JSON.stringify({ phase: 'P7-V2-R3B-REBASELINE', kind: 'current', runId, freeze: 'failed', error: error.message }, null, 2));
  process.exit(1);
}
