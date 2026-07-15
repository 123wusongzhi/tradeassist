import { spawnSync } from 'node:child_process';
import { freezeRawArtifact } from './p7-v2-artifact-freeze.mjs';
import { readJSON, valueOf, writeJSON } from './p7-v2-lib.mjs';

const args = process.argv.slice(2);
const runId = valueOf(args, '--run-id') || `p7v2-baseline-${new Date().toISOString().replace(/[:.]/g, '-')}`;
if (!/^p7v2-baseline-r3b-recovery3-[a-z0-9_-]+$/.test(runId)) {
  throw new Error('P7-V2-R3B-REBASELINE2 requires a unique baseline run ID');
}
const res = spawnSync(process.execPath, ['scripts/p7-v2-load.mjs', '--kind', 'baseline', '--run-id', runId, ...args.filter((a) => !a.startsWith('--run-id'))], {
  stdio: 'inherit',
});
if (res.status !== 0) process.exit(res.status ?? 1);

try {
  const reportPath = `docs/baselines/p7-v2-baseline-${runId}.json`;
  const report = readJSON(reportPath);
  const frozen = freezeRawArtifact({ kind: 'baseline', runId, reportPath });
  const registryPath = 'docs/baselines/p7-v2-baseline-registry.json';
  const registry = readJSON(registryPath) || { phase: 'P7-V2-R3B-REBASELINE2', baselines: [] };
  if ((registry.baselines || []).some((item) => item.runId === runId)) throw new Error('baseline registry already contains this run ID');
  const baseline = {
    ...report,
    immutable: true,
    validForRegression: true,
    rawArtifactPath: frozen.originalPath,
    rawArtifactSha256: frozen.sha256,
    rawArtifactSizeBytes: frozen.sizeBytes,
    rawArtifactHashVerified: true,
    scenarioCoverage: frozen.scenarioCoverage,
    frozenAt: frozen.createdAt,
    frozenArtifactPath: frozen.frozenPath,
    reportPath,
  };
  registry.phase = 'P7-V2-R3B-REBASELINE2';
  registry.baselines = (registry.baselines || []).map((entry) =>
    entry.runId === 'p7v2-baseline-r3b-recovery-20260714-1719'
      ? {
          ...entry,
          validForRegression: false,
          superseded: true,
          supersededBy: runId,
          reason: 'runtime_source_tree_changed_after_harness_repair',
        }
      : entry,
  );
  registry.activeRegressionBaseline = runId;
  registry.baselines = [...registry.baselines, baseline];
  writeJSON(registryPath, registry);
  writeJSON(`docs/baselines/p7-v2-baseline-${runId}.json`, baseline);
  console.log(JSON.stringify({ phase: 'P7-V2-R3B-REBASELINE', kind: 'baseline', runId, freeze: 'passed', sha256: frozen.sha256 }, null, 2));
} catch (error) {
  console.error(JSON.stringify({ phase: 'P7-V2-R3B-REBASELINE', kind: 'baseline', runId, freeze: 'failed', error: error.message }, null, 2));
  process.exit(1);
}
