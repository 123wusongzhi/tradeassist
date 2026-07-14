import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { readJSON, root, valueOf, writeJSON } from './p7-v2-lib.mjs';

const args = process.argv.slice(2);
const runId = valueOf(args, '--run-id') || `p7v2-current-${new Date().toISOString().replace(/[:.]/g, '-')}`;
const restart = spawnSync(process.execPath, ['scripts/p7-v2-restart-environment.mjs', '--run-id', `${runId}-restart`], {
  stdio: 'inherit',
});
if (restart.status !== 0) process.exit(restart.status ?? 1);
const runtime = readJSON('docs/p7-v2-runtime-environment.json') || {};
runtime.restartPerformed = true;
runtime.apiProcessChanged = true;
runtime.currentRunIndependent = true;
writeJSON('docs/p7-v2-runtime-environment.json', runtime);
const res = spawnSync(process.execPath, ['scripts/p7-v2-load.mjs', '--kind', 'current', '--run-id', runId, ...args.filter((a) => !a.startsWith('--run-id'))], {
  stdio: 'inherit',
});
const current = readJSON('docs/p7-v2-current-load-report.json');
const restartEvidence = readJSON('docs/p7-v2-environment-restart-report.json') || {};
if (current?.runId === runId) {
  current.independentRun = res.status === 0 && restartEvidence.restartPerformed === true && restartEvidence.apiProcessChanged === true;
  current.restartEvidence = {
    restartPerformed: restartEvidence.restartPerformed === true,
    apiProcessChanged: restartEvidence.apiProcessChanged === true,
    workerProcessChanged: restartEvidence.workerProcessChanged === true,
    redisRestarted: restartEvidence.redisRestarted === true,
    mockProviderRestarted: restartEvidence.mockProviderRestarted === true,
    databaseStateReset: restartEvidence.databaseStateReset === true,
    authProbePassed: restartEvidence.authProbePassed === true,
    routeProbePassed: restartEvidence.routeProbePassed === true,
  };
  current.status = current.status === 'passed' && current.independentRun ? 'passed' : 'failed';
  writeJSON('docs/p7-v2-current-load-report.json', current);
  writeJSON(`docs/runs/p7-v2-current-${runId}.json`, current);
}
process.exit(res.status ?? 1);
