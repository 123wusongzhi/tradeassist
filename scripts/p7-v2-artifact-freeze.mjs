import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { readJSON, root, writeJSON } from './p7-v2-lib.mjs';

const REQUIRED_SCENARIOS = [
  'Product List',
  'Order List',
  'Inventory List',
  'Task List',
  'Webhook Event List',
  'Operation Log List',
  'Webhook Ingestion',
  'Provider Mock Flow',
  'Auth/Security',
];

function sha256(buffer) {
  return crypto.createHash('sha256').update(buffer).digest('hex');
}

function rawMetricRequests(summary, metricName) {
  return Number(summary?.metrics?.[metricName]?.values?.count ?? summary?.metrics?.[metricName]?.count ?? 0);
}

function rawScenarioCoverage(summary) {
  const metricNames = {
    'Product List': 'p7_product_list_requests',
    'Order List': 'p7_order_list_requests',
    'Inventory List': 'p7_inventory_list_requests',
    'Task List': 'p7_task_list_requests',
    'Webhook Event List': 'p7_webhook_event_list_requests',
    'Operation Log List': 'p7_operation_log_list_requests',
    'Webhook Ingestion': 'p7_webhook_ingestion_requests',
    'Provider Mock Flow': 'p7_provider_mock_flow_requests',
    'Auth/Security': 'p7_auth_security_requests',
  };
  const scenarios = REQUIRED_SCENARIOS.map((scenario) => ({
    scenario,
    requests: rawMetricRequests(summary, metricNames[scenario]),
  }));
  return { scenarios, passed: scenarios.every((item) => item.requests > 0) };
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
  if (!/^p7v2-(baseline|current)-r3b-recovery2-[a-z0-9_-]+$/.test(runId)) {
    throw new Error('a unique Rebaseline2 run ID is required');
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
  if (!coverage.passed) throw new Error('raw k6 artifact lacks required scenario coverage');

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
      sloFingerprint: report.sloFingerprint || '',
      routeCredentialMatrixFingerprint: report.routeCredentialMatrixFingerprint || '',
      regressionPolicyFingerprint: report.regressionPolicyFingerprint || '',
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
      loadProfileFingerprint: manifest.loadProfileFingerprint,
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
