import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { readJSON, root, writeJSON } from './p7-v2-lib.mjs';
import { CORE_SCENARIOS, SCENARIO_METRICS } from './p7-v2-regression-metrics.mjs';

const REQUIRED_SCENARIOS = CORE_SCENARIOS;

function sha256(buffer) {
  return crypto.createHash('sha256').update(buffer).digest('hex');
}

function rawMetricRequests(summary, metricName) {
  return Number(summary?.metrics?.[metricName]?.values?.count ?? summary?.metrics?.[metricName]?.count ?? 0);
}

function rawScenarioCoverage(summary) {
  const scenarios = REQUIRED_SCENARIOS.map((scenario) => ({
    scenario,
    requests: rawMetricRequests(summary, SCENARIO_METRICS[scenario][1]),
    p99: (summary?.metrics?.[SCENARIO_METRICS[scenario][0]]?.values || summary?.metrics?.[SCENARIO_METRICS[scenario][0]] || {})['p(99)'],
  }));
  return { scenarios, passed: scenarios.every((item) => item.requests >= 100 && Number.isFinite(item.p99)) };
}

function ensureRegularFile(filePath) {
  const stat = fs.statSync(filePath);
  if (!stat.isFile()) throw new Error(`raw artifact is not a regular file: ${filePath}`);
  return stat;
}

export function frozenArtifactPath(kind, runId) {
  const family = kind === 'baseline' ? 'baselines' : 'currents';
  return path.join(root, 'docs', family, 'frozen', runId);
}

export function freezeRawArtifact({ kind, runId, reportPath }) {
  if (!['baseline', 'current'].includes(kind)) throw new Error(`unsupported freeze kind: ${kind}`);
  if (!/^p7v2-(baseline|current)-r3b-recovery6-[a-z0-9_-]+$/.test(runId)) {
    throw new Error('a unique Recovery6 run ID is required');
  }

  const report = readJSON(reportPath);
  if (!report || report.runId !== runId || report.status !== 'passed') {
    throw new Error('passed report with matching run ID is required before freeze');
  }
  if (kind === 'current' && (report.currentRunIndependent !== true || report.independentRun !== true)) {
    throw new Error('independent Current evidence is required before freeze');
  }

  const rawPath = path.join(root, 'artifacts', 'p7-v2', kind, runId, `${kind}.summary.json`);
  if (!fs.existsSync(rawPath)) throw new Error(`raw k6 artifact missing: ${rawPath}`);
  const rawStat = ensureRegularFile(rawPath);
  const raw = fs.readFileSync(rawPath);
  let summary;
  try {
    summary = JSON.parse(raw.toString('utf8'));
  } catch {
    throw new Error('raw k6 artifact JSON cannot be parsed');
  }

  const requests = rawMetricRequests(summary, 'http_reqs');
  const coverage = rawScenarioCoverage(summary);
  if (requests <= 0) throw new Error('raw k6 artifact has zero requests');
  if (!coverage.passed) throw new Error('raw k6 artifact lacks required steady scenario coverage, p99, or 100 samples');

  const destination = frozenArtifactPath(kind, runId);
  if (fs.existsSync(destination)) throw new Error(`frozen artifact already exists: ${destination}`);
  const parent = path.dirname(destination);
  const staging = path.join(parent, `.${runId}.staging-${process.pid}-${Date.now()}`);
  fs.mkdirSync(staging, { recursive: true });

  try {
    const frozenRawPath = path.join(staging, 'raw-summary.json');
    fs.copyFileSync(rawPath, frozenRawPath, fs.constants.COPYFILE_EXCL);
    const originalSha256 = sha256(raw);
    const copied = fs.readFileSync(frozenRawPath);
    const frozenSha256 = sha256(copied);
    if (frozenSha256 !== originalSha256 || copied.length !== rawStat.size) {
      throw new Error('frozen raw artifact verification failed');
    }

    const now = new Date().toISOString();
    const rawArtifact = {
      relativePath: 'raw-summary.json',
      sha256: originalSha256,
      sizeBytes: rawStat.size,
    };
    const manifest = {
      runId,
      runKind: kind,
      baselineRunId: kind === 'current' ? report.baselineRunId || '' : undefined,
      status: report.status,
      independentRun: kind === 'current' ? report.independentRun === true : undefined,
      validForRegression: true,
      originalPath: path.relative(root, rawPath).replaceAll('\\', '/'),
      frozenPath: path.relative(root, path.join(destination, 'raw-summary.json')).replaceAll('\\', '/'),
      sizeBytes: rawStat.size,
      sha256: originalSha256,
      rawArtifact,
      requests,
      scenarioCoverage: coverage.passed,
      createdAt: now,
      runtimeSourceTreeHash: report.runtimeSourceTreeHash || '',
      loadScriptsHash: report.loadScriptsHash || report.loadScriptHash || '',
      metricSemanticsHash: report.metricSemanticsHash || '',
      datasetFingerprint: report.datasetFingerprint || '',
      configFingerprint: report.configFingerprint || '',
      loadProfileFingerprint: report.loadProfileFingerprint || '',
      canonicalLoadProfileVersion: report.canonicalLoadProfile?.schemaVersion || 0,
      loadProfileFingerprintVersion: report.loadProfileFingerprintVersion || 0,
      sloFingerprint: report.sloFingerprint || '',
      routeCredentialMatrixFingerprint: report.routeCredentialMatrixFingerprint || '',
      regressionPolicyFingerprint: report.regressionPolicyFingerprint || '',
      selectedHost: report.selectedHost || '',
      selectedPort: report.selectedPort || 0,
      baseUrl: report.baseUrl || '',
      environmentFingerprint: report.environmentFingerprint || {},
      immutable: true,
      frozenAt: now,
    };
    writeJSON(path.relative(root, path.join(staging, 'manifest.json')), manifest);
    fs.writeFileSync(path.join(staging, 'raw-summary.sha256'), `${originalSha256}  raw-summary.json\n`, 'utf8');
    writeJSON(path.relative(root, path.join(staging, 'runtime-fingerprint.json')), {
      runtimeSourceTreeHash: manifest.runtimeSourceTreeHash,
      environmentFingerprint: report.environmentFingerprint || {},
    });
    writeJSON(path.relative(root, path.join(staging, 'dataset-fingerprint.json')), {
      datasetFingerprint: manifest.datasetFingerprint,
    });
    writeJSON(path.relative(root, path.join(staging, 'load-profile.json')), {
      loadProfile: report.loadProfile || {},
      canonicalLoadProfile: report.canonicalLoadProfile || {},
      loadProfileFingerprint: manifest.loadProfileFingerprint,
      fingerprintVersion: report.loadProfileFingerprintVersion || 0,
    });
    writeJSON(path.relative(root, path.join(staging, 'config-fingerprint.json')), {
      configFingerprint: manifest.configFingerprint,
    });
    writeJSON(path.relative(root, path.join(staging, 'route-credential-matrix-fingerprint.json')), {
      routeCredentialMatrixFingerprint: manifest.routeCredentialMatrixFingerprint,
    });
    writeJSON(path.relative(root, path.join(staging, 'regression-policy.json')), {
      regressionPolicyFingerprint: manifest.regressionPolicyFingerprint,
    });
    if (kind === 'current') {
      writeJSON(path.relative(root, path.join(staging, 'restart-evidence.json')), report.restartEvidence || {});
    }
    fs.renameSync(staging, destination);
    if (report.loadProfileFingerprintVersion === 3 && report.canonicalLoadProfile) {
      const sidecarPath = `docs/fingerprints/p7-v2/load-profile/v3/${runId}.json`;
      writeJSON(sidecarPath, {
        fingerprintVersion: 3,
        runId,
        runKind: kind,
        loadProfileFingerprint: manifest.loadProfileFingerprint,
        canonicalProfile: report.canonicalLoadProfile,
        derivedEvidence: true,
        sourceArtifactsModified: false,
      });
      const registryPath = 'docs/fingerprints/p7-v2/load-profile-registry.json';
      const registry = readJSON(registryPath) || { fingerprintType: 'load_profile', activeVersion: 3, entries: [] };
      if ((registry.entries || []).some((entry) => entry.runId === runId)) throw new Error(`load-profile registry already contains run ID: ${runId}`);
      writeJSON(registryPath, {
        ...registry,
        activeVersion: 3,
        entries: [...(registry.entries || []), { runId, path: sidecarPath, fingerprintVersion: 3, loadProfileFingerprint: manifest.loadProfileFingerprint }],
      });
    }
    return { ...manifest, archivePath: path.relative(root, destination).replaceAll('\\', '/') };
  } catch (error) {
    fs.rmSync(staging, { recursive: true, force: true });
    throw error;
  }
}

if (import.meta.url === `file://${process.argv[1]?.replaceAll('\\', '/')}`) {
  const args = process.argv.slice(2);
  const value = (key) => args[args.indexOf(key) + 1] || '';
  const kind = value('--kind');
  const runId = value('--run-id');
  const reportPath = value('--report');
  try {
    console.log(JSON.stringify({ status: 'passed', ...freezeRawArtifact({ kind, runId, reportPath }) }, null, 2));
  } catch (error) {
    console.error(JSON.stringify({ status: 'failed', runId, kind, error: error.message }, null, 2));
    process.exit(1);
  }
}
