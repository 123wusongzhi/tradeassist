import fs from 'node:fs';
import path from 'node:path';
import { readJSON, root, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const historical = readJSON('docs/p7-v2-r3a-historical-baseline-correction.json') || {};
const freeze = readJSON('docs/p7-v2-r3a-baseline-freeze-report.json') || {};
const comparability = readJSON('docs/p7-v2-r3a-comparability-precondition-report.json') || {};
const cleanup = readJSON('docs/p7-v2-r3a-runtime-cleanup-report.json') || {};
const registry = readJSON('docs/baselines/p7-v2-baseline-registry.json') || { baselines: [] };
const baseline = (registry.baselines || []).find((item) => item.runId === freeze.runId);
const rawArtifact = baseline?.rawArtifactPath ? path.join(root, baseline.rawArtifactPath) : '';
const checks = {
  historicalInvalid: historical.validForRegression === false,
  historicalPreserved: historical.preserved === true,
  historicalSuperseded: historical.superseded === true,
  cleanupPrefixEmpty: cleanup.cleanup?.remainingDatabasesWithPrefix === 0,
  baselinePassed: baseline?.status === 'passed',
  baselineTraffic: Number(baseline?.completedRequests || 0) > 0,
  scenarioCoverage: baseline?.scenarioCoverage === true,
  immutable: baseline?.immutable === true,
  rawArtifactExists: Boolean(rawArtifact && fs.existsSync(rawArtifact)),
  rawArtifactHashVerified: baseline?.rawArtifactHashVerified === true,
  validForRegression: baseline?.validForRegression === true,
  comparabilityPassed: comparability.status === 'passed',
  currentExecutionAllowed: comparability.currentExecutionAllowed === true,
};
const failed = Object.entries(checks).filter(([, ok]) => !ok).map(([id]) => id);
const report = {
  phase: 'P7-V2-R3A',
  status: failed.length ? 'incomplete' : 'passed_ready_for_p7_v2_r3b',
  historicalBaseline: historical,
  baseline: baseline || {},
  cleanup: {
    status: cleanup.cleanup?.remainingDatabasesWithPrefix === 0 ? 'passed' : 'incomplete',
    remainingDatabasesWithPrefix: cleanup.cleanup?.remainingDatabasesWithPrefix ?? null,
  },
  comparabilityPrecondition: {
    status: comparability.status || 'missing',
    currentExecutionAllowed: comparability.currentExecutionAllowed === true,
  },
  next: {
    current: 'pending_p7_v2_r3b',
    regression: 'pending_p7_v2_r3b',
    soak: 'pending_p7_v2_r3b',
    demoRun1: 'pending_p7_v2_r3b',
    demoRun2: 'pending_p7_v2_r3b',
    finalGates: 'pending_p7_v2_r3b',
  },
  production: {
    resourcesAccessed: false,
    realProviderCalls: 0,
    realDouyinWrites: 0,
    autoListingTriggered: false,
    tagCreated: false,
    productionReady: false,
  },
  checks: Object.entries(checks).map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  issues: failed,
};
writeJSON('docs/p7-v2-r3a-final-report.json', report);
writeMarkdown('docs/P7_V2_R3A_FINAL_REPORT.md', `# P7-V2-R3A Final Report\n\nStatus: **${report.status}**\n\n- Baseline: \`${baseline?.runId || ''}\`\n- Remaining P7-V2 databases: ${report.cleanup.remainingDatabasesWithPrefix}\n- Comparability: ${report.comparabilityPrecondition.status}\n- Current / Regression / Soak / Demo remain pending for R3B.\n\nIssues:\n${failed.length ? failed.map((item) => `- ${item}`).join('\n') : '- none'}\n`);
console.log(JSON.stringify(report, null, 2));
process.exit(failed.length ? 1 : 0);
