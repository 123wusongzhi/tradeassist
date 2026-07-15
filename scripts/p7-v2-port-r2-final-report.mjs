import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const owner = readJSON('docs/p7-v2-r3b-port-owner-audit.json') || {};
const selection = readJSON('docs/p7-v2-r3b-port-selection-report.json') || {};
const probe = readJSON('docs/p7-v2-r3b-port-restart-probe-report.json') || {};
const config = readJSON('docs/p7-v2-r3b-port-configuration-audit.json') || {};
const status = owner.status === 'passed' && selection.status === 'passed' && config.status === 'passed' && probe.status === 'passed' ? 'passed' : 'incomplete';
const report = {
  phase: 'P7-V2-R3B-PORT-R2',
  status,
  blockedStage: status === 'passed' ? '' : 'restart_probe',
  originalPort: selection.originalPort || 8080,
  originalPortClassification: selection.originalPortClassification || owner.classification || 'unknown',
  windowsOwner: owner.windows || {},
  wslOwner: owner.wsl || {},
  belongsToTradeMind: owner.belongsToTradeMind === true,
  action: selection.action || 'none',
  portMigrated: selection.action === 'port_migration',
  selectedPort: selection.selectedPort || 0,
  portProbe: { passedCycles: probe.passedCycles || 0, failedCycles: probe.failedCycles || 0 },
  runtimeFreeze: { status: 'not_started' },
  baseline: { runId: '', status: 'not_started', artifactStatus: 'not_started' },
  current: { runId: '', status: 'not_started', artifactStatus: 'not_started' },
  comparability: { status: 'not_started', differences: [] },
  regression: { status: 'not_started', failedMetrics: [] },
  newRunIdsRequired: true,
  rebaselineRequired: true,
  nextStep: status === 'passed' ? 'generate recovery3 runtime freeze' : 'repair restart probe and rerun with new recovery3 run IDs',
  productionResourcesAccessed: false,
  realProviderCalls: 0,
  realDouyinWrites: 0,
  tagCreated: false,
  productionReady: false,
  generatedAt: new Date().toISOString(),
};
writeJSON('docs/p7-v2-r3b-port-r2-final-report.json', report);
writeMarkdown('docs/P7_V2_R3B_PORT_R2_FINAL_REPORT.md', `# P7-V2-R3B-PORT-R2 Final Report\n\nStatus: **${status}**\n\n- Original port: ${report.originalPort}\n- Classification: \`${report.originalPortClassification}\`\n- Action: \`${report.action}\`\n- Selected port: ${report.selectedPort || 'unavailable'}\n- Restart probes: ${report.portProbe.passedCycles}/3\n- Baseline / Current / Comparability / Regression: not started\n\nPhase P7-V2-R3B-PORT-R2 Incomplete\n\nPhase P7-V2-R3B-REBASELINE2 Incomplete\n\nPhase P7-V2-R3B Execution Blocked\n\nPhase P7-V2 Incomplete\n\nPhase P7 Closure Verification Incomplete\n`);
console.log(JSON.stringify(report, null, 2));
process.exit(status === 'passed' ? 0 : 1);
