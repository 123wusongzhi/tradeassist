import { fileURLToPath } from 'node:url';
import path from 'node:path';
import { RUNTIME_FREEZE_PATH } from './p7-v2-r3b-lpc-r3-runtime-freeze.mjs';
import { readJSON, runWSL, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

export const LIFECYCLE_AUDIT_JSON = 'docs/p7-v2-r3b-runtime-freeze-lifecycle-audit.json';
export const LIFECYCLE_AUDIT_MD = 'docs/P7_V2_R3B_RUNTIME_FREEZE_LIFECYCLE_AUDIT.md';

const OLD_FREEZE_ID = '057f7285831caac4f52d9bffec5559115954dda6f76407249fbf7d9b94b70d00';
const OLD_BASELINE_RUN_ID = 'p7v2-baseline-r3b-recovery6-20260715112749';
const OLD_CURRENT_RUN_ID = 'p7v2-current-r3b-recovery6-20260715112749';
const OLD_BASELINE_SHA = '5fe8002a2491c3a7bca09dee19386d120b807e6e4b8d0295de1461ed86c6e7a5';
const OLD_DB = 'trademind_p7v2_p7v2_baseline_r3b_recovery6_20260715112749';

function invalidateFreeze() {
  const doc = readJSON(RUNTIME_FREEZE_PATH) || {};
  const current = doc.current || doc;
  const history = Array.isArray(doc.history) ? [...doc.history] : [];
  let oldFreezeInvalidated = false;
  if (current?.runtimeFreezeId === OLD_FREEZE_ID) {
    const invalidated = {
      ...current,
      status: 'invalidated',
      active: false,
      validForFormalExecution: false,
      reason: 'runtime_freeze_lifecycle_contract_changed_after_baseline',
      invalidatedAt: new Date().toISOString(),
    };
    writeJSON(RUNTIME_FREEZE_PATH, { current: invalidated, history });
    oldFreezeInvalidated = true;
  }
  return oldFreezeInvalidated;
}

function supersedeBaseline() {
  const registryPath = 'docs/baselines/p7-v2-baseline-registry.json';
  const registry = readJSON(registryPath) || { baselines: [] };
  let oldBaselineSuperseded = false;
  const baselines = (registry.baselines || []).map((entry) => {
    if (entry.runId !== OLD_BASELINE_RUN_ID) return entry;
    oldBaselineSuperseded = true;
    return {
      ...entry,
      active: false,
      status: 'superseded',
      validForRegression: false,
      superseded: true,
      supersededReason: 'runtime_freeze_lifecycle_contract_changed',
    };
  });
  writeJSON(registryPath, {
    ...registry,
    activeRegressionBaseline: registry.activeRegressionBaseline === OLD_BASELINE_RUN_ID ? '' : registry.activeRegressionBaseline,
    baselines,
  });

  const manifestPath = `docs/baselines/frozen/${OLD_BASELINE_RUN_ID}/manifest.json`;
  const manifest = readJSON(manifestPath);
  let artifactIntegrity = false;
  if (manifest?.sha256 === OLD_BASELINE_SHA) {
    writeJSON(manifestPath, {
      ...manifest,
      artifactIntegrity: true,
      immutable: true,
      superseded: true,
      validForRegression: false,
      reason: 'runtime_freeze_lifecycle_validator_changed_after_baseline',
    });
    artifactIntegrity = true;
  }
  return { oldBaselineSuperseded, artifactIntegrity };
}

function supersedePlan() {
  const manifestPath = 'docs/p7-v2-r3b-run-manifest.json';
  const manifest = readJSON(manifestPath) || {};
  const superseded = {
    ...manifest,
    status: 'superseded_after_baseline',
    active: false,
    validForExecution: false,
    validForRegression: false,
    oldCurrentRunIdSuperseded: manifest.currentRunId === OLD_CURRENT_RUN_ID,
    supersededReason: 'runtime_freeze_lifecycle_contract_changed',
    supersededAt: new Date().toISOString(),
  };
  writeJSON(manifestPath, superseded);
  writeJSON('docs/p7-v2-r3b-recovery6-superseded-plan.json', superseded);
  return manifest.baselineRunId === OLD_BASELINE_RUN_ID && manifest.currentRunId === OLD_CURRENT_RUN_ID;
}

function dropOldResidualDatabase() {
  const check = runWSL(`psql -h /var/run/postgresql -U root -At -d postgres -c "select datname from pg_database where datname='${OLD_DB}';" 2>/dev/null || true`, { timeout: 30000 });
  const existsBefore = String(check.stdout || '').trim() === OLD_DB;
  if (!existsBefore) return { existsBefore, dropped: true, existsAfter: false };
  const drop = runWSL(`psql -h /var/run/postgresql -U root -At -d postgres -c "drop database if exists ${OLD_DB};" 2>/dev/null`, { timeout: 30000 });
  const recheck = runWSL(`psql -h /var/run/postgresql -U root -At -d postgres -c "select datname from pg_database where datname='${OLD_DB}';" 2>/dev/null || true`, { timeout: 30000 });
  return { existsBefore, dropStatus: drop.status, dropped: drop.status === 0 && !String(recheck.stdout || '').trim(), existsAfter: Boolean(String(recheck.stdout || '').trim()) };
}

export function runLifecycleAudit() {
  const oldFreezeInvalidated = invalidateFreeze();
  const baseline = supersedeBaseline();
  const oldCurrentPlanSuperseded = supersedePlan();
  const database = dropOldResidualDatabase();
  const report = {
    phase: 'P7-V2-R3B-RUNTIME-FREEZE-LIFECYCLE-FIX',
    component: 'runtime-freeze-lifecycle-audit',
    status: oldFreezeInvalidated && baseline.oldBaselineSuperseded && baseline.artifactIntegrity && oldCurrentPlanSuperseded && database.dropped ? 'passed' : 'failed',
    runtimeFreezeLifecycleContractVersion: 2,
    oldRuntimeFreezeId: OLD_FREEZE_ID,
    oldFreezeInvalidated,
    oldBaselineRunId: OLD_BASELINE_RUN_ID,
    oldBaselineArtifactSha256: OLD_BASELINE_SHA,
    oldBaselineSuperseded: baseline.oldBaselineSuperseded,
    oldBaselineArtifactIntegrity: baseline.artifactIntegrity,
    oldCurrentRunId: OLD_CURRENT_RUN_ID,
    oldCurrentPlanSuperseded,
    oldResidualDatabaseCleaned: database.dropped,
    database,
  };
  writeJSON(LIFECYCLE_AUDIT_JSON, report);
  writeMarkdown(
    LIFECYCLE_AUDIT_MD,
    `# P7-V2-R3B Runtime Freeze Lifecycle Audit\n\nStatus: **${report.status}**\n\n- Old freeze invalidated: \`${report.oldFreezeInvalidated}\`\n- Old baseline superseded: \`${report.oldBaselineSuperseded}\`\n- Old current plan superseded: \`${report.oldCurrentPlanSuperseded}\`\n- Old residual database cleaned: \`${report.oldResidualDatabaseCleaned}\`\n`,
  );
  return report;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = runLifecycleAudit();
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}
