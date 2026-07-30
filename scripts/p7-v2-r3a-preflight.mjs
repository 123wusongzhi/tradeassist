import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { readJSON, root, runWSL, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const historicalRunId = 'p7v2-baseline-20260714181000';
const candidateRunId = 'p7v2-r3-baseline-20260714133131';
const candidateSummaryPath = `artifacts/p7-v2/baseline/${candidateRunId}/baseline.summary.json`;
const candidateReport = readJSON(`docs/baselines/p7-v2-baseline-${candidateRunId}.json`) || {};
const historical = readJSON('docs/p7-v2-baseline-report.json') || {};
const summaryPath = path.join(root, candidateSummaryPath);
const summary = fs.existsSync(summaryPath) ? JSON.parse(fs.readFileSync(summaryPath, 'utf8')) : null;
const rawArtifactFound = Boolean(summary);
const rawArtifactSha256 = rawArtifactFound ? crypto.createHash('sha256').update(fs.readFileSync(summaryPath)).digest('hex') : '';
const requests = Number(summary?.metrics?.http_reqs?.values?.count || summary?.metrics?.http_reqs?.count || 0);
const scenarioNames = (candidateReport.scenarios || []).map((item) => item.scenario);
const scenarioCoverage = scenarioNames.length >= 9 && (candidateReport.scenarios || []).every((item) => Number(item.requests) > 0);
const dbRows = runWSL(`psql -h /var/run/postgresql -U root -d postgres -At -F '|' -c "SELECT datname, datallowconn FROM pg_database WHERE datname LIKE 'trademind_p7v2_%' ORDER BY datname;"`, { timeout: 30000 });
const connectionRows = runWSL(`psql -h /var/run/postgresql -U root -d postgres -At -F '|' -c "SELECT datname, pid, coalesce(application_name, ''), state FROM pg_stat_activity WHERE datname LIKE 'trademind_p7v2_%' ORDER BY datname, pid;"`, { timeout: 30000 });
const connectionsByDb = new Map();
for (const line of (connectionRows.stdout || '').split(/\r?\n/).filter(Boolean)) {
  const [databaseName, pid, applicationName, state] = line.split('|');
  const list = connectionsByDb.get(databaseName) || [];
  list.push({ pid: Number(pid), applicationName, state });
  connectionsByDb.set(databaseName, list);
}
const databases = (dbRows.stdout || '').split(/\r?\n/).filter(Boolean).map((line) => {
  const [databaseName, datallowconn] = line.split('|');
  const connections = connectionsByDb.get(databaseName) || [];
  return {
    databaseName,
    databasePrefixValid: /^trademind_p7v2_[a-zA-Z0-9_-]+$/.test(databaseName),
    knownRunId: databaseName.replace(/^trademind_p7v2_/, ''),
    knownPhase: 'P7-V2',
    connectionCount: connections.length,
    connectionPids: connections.map((item) => item.pid),
    applicationNames: [...new Set(connections.map((item) => item.applicationName).filter(Boolean))],
    localEnvironmentVerified: dbRows.status === 0,
    safeToDropCandidate: true,
    reason: `isolated P7-V2 runtime database; datallowconn=${datallowconn}`,
  };
});

const historicalCorrection = {
  runId: historicalRunId,
  previousReportedStatus: historical.status || 'passed',
  committedParsedHttpRequests: Number(historical.completedRequests || 0),
  rawTrafficClaimed: true,
  rawArtifactCurrentlyVerifiable: false,
  validForRegression: false,
  preserved: true,
  superseded: true,
  reason: 'zero parsed HTTP metrics cannot support performance verification',
};
const candidateAudit = {
  phase: 'P7-V2-R3A',
  candidateRunId,
  runStartedAt: candidateReport.environmentFingerprint?.startedAt || '',
  runEndedAt: candidateReport.environmentFingerprint?.endedAt || '',
  requests,
  iterations: Number(summary?.metrics?.iterations?.values?.count || summary?.metrics?.iterations?.count || 0),
  configuredVUs: candidateReport.configuredVUs || 0,
  peakVUs: candidateReport.peakVUs || 0,
  rawK6ObservedMaxVUs: Number(summary?.metrics?.vus_max?.values?.value || summary?.metrics?.vus_max?.value || 0),
  scenarioNames,
  scenarioRequestCounts: Object.fromEntries((candidateReport.scenarios || []).map((item) => [item.scenario, item.requests])),
  scenarioLatencyMetrics: Object.fromEntries((candidateReport.scenarios || []).map((item) => [item.scenario, {
    p50: item.p50, p90: item.p90, p95: item.p95, p99: item.p99, max: item.max,
  }])),
  unexpected401: candidateReport.unexpected401 || 0,
  unexpected403: candidateReport.unexpected403 || 0,
  unexpected404: candidateReport.unexpected404 || 0,
  unexpected5xx: candidateReport.unexpected5xx || 0,
  thresholdsPassed: candidateReport.thresholdsPassed === true,
  absoluteSloPassed: candidateReport.absoluteSloPassed === true,
  rawArtifactPaths: rawArtifactFound ? [candidateSummaryPath] : [],
  rawArtifactSizes: rawArtifactFound ? [fs.statSync(summaryPath).size] : [],
  rawArtifactHashes: rawArtifactFound ? [rawArtifactSha256] : [],
  rawArtifactFound,
  rawArtifactParseable: rawArtifactFound,
  scenarioCoverage,
  immutable: false,
  freezeEligible: false,
  provenanceIncomplete: true,
  reason: 'candidate runtime source hash cannot be proven against a contemporaneous source-file manifest; retain candidate and rerun after approved cleanup',
};
const preflight = {
  phase: 'P7-V2-R3A',
  status: 'blocked_pending_database_approval',
  historicalBaseline: {
    runId: historicalRunId,
    reportedPassed: true,
    committedParsedRequests: historicalCorrection.committedParsedHttpRequests,
    validForRegression: false,
    preserved: true,
    superseded: true,
  },
  candidateBaseline: {
    found: rawArtifactFound,
    runId: candidateRunId,
    requests,
    scenarioCoverage,
    classifiedErrors: (candidateReport.unexpected401 || 0) + (candidateReport.unexpected403 || 0) + (candidateReport.unexpected404 || 0) + (candidateReport.unexpected5xx || 0),
    rawArtifactFound,
    immutable: false,
    freezeEligible: false,
  },
  cleanup: { remainingDatabaseCount: databases.length, approvalRequired: true },
  issues: ['candidate provenance is incomplete; a fresh baseline is required', 'exact database deletion approval is required before runtime cleanup'],
};
const approval = {
  phase: 'P7-V2-R3A',
  status: 'pending_user_approval',
  approvalRequired: true,
  localEnvironment: {
    appEnv: 'performance',
    performanceTestMode: true,
    production: false,
    hostClass: 'wsl2_local_postgresql_socket',
  },
  databases,
  approvalInstructions: 'Approval must name every exact database to delete. No wildcard, prefix, or blanket approval is accepted.',
};

writeJSON('docs/p7-v2-r3a-historical-baseline-correction.json', historicalCorrection);
writeMarkdown('docs/P7_V2_R3A_HISTORICAL_BASELINE_CORRECTION.md', `# P7-V2-R3A Historical Baseline Correction\n\nHistorical R2 Baseline Report Invalid\n\n- Run ID: \`${historicalRunId}\`\n- Previously reported: passed\n- Committed parsed HTTP requests: ${historicalCorrection.committedParsedHttpRequests}\n- Preserved: true\n- Superseded: true\n- Valid for regression: false\n\nThe original evidence remains unchanged.\n`);
writeJSON('docs/p7-v2-r3a-candidate-baseline-audit.json', candidateAudit);
writeMarkdown('docs/P7_V2_R3A_CANDIDATE_BASELINE_AUDIT.md', `# P7-V2-R3A Candidate Baseline Audit\n\nStatus: **provenance_incomplete**\n\n| Field | Value |\n| --- | --- |\n| Run ID | ${candidateRunId} |\n| Requests | ${requests} |\n| Scenario coverage | ${scenarioCoverage} |\n| Raw artifact | ${candidateSummaryPath} |\n| Raw artifact SHA-256 | ${rawArtifactSha256} |\n| Freeze eligible | false |\n\nThe candidate is preserved. A new formal baseline is required because contemporaneous runtime source provenance cannot be proven.\n`);
writeJSON('docs/p7-v2-r3a-preflight-audit.json', preflight);
writeMarkdown('docs/P7_V2_R3A_PREFLIGHT_AUDIT.md', `# P7-V2-R3A Preflight Audit\n\nStatus: **${preflight.status}**\n\n- Historical baseline is invalid for regression but preserved and superseded.\n- Candidate has ${requests} parsed requests and scenario coverage ${scenarioCoverage}, but is not eligible for post-run provenance freeze.\n- Runtime cleanup is blocked on exact-name user approval for ${databases.length} databases.\n`);
writeJSON('docs/p7-v2-r3a-cleanup-approval-request.json', approval);
writeMarkdown('docs/P7_V2_R3A_CLEANUP_APPROVAL_REQUEST.md', `# P7-V2-R3A Runtime Cleanup Approval Request\n\nNo database has been terminated or dropped.\n\n| Database | Connections | PIDs | Safe candidate |\n| --- | ---: | --- | --- |\n${databases.map((item) => `| ${item.databaseName} | ${item.connectionCount} | ${item.connectionPids.join(', ') || '-'} | ${item.safeToDropCandidate} |`).join('\n')}\n\nPlease approve deletion by explicitly listing every exact database name. Wildcards and prefix-only approval are rejected.\n`);
console.log(JSON.stringify({ preflight, approval }, null, 2));
process.exit(0);
