import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';
import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

export const P9_OWNER_SCOPE_DECISION_JSON = 'docs/p9-owner-scope-decision.json';
export const P9_SCOPE_DISCOVERY_JSON = 'docs/p9-scope-discovery.json';
export const P9_EXECUTION_PLAN_JSON = 'docs/p9-execution-plan.json';
export const P9_BATCH_1_SCOPE_JSON = 'docs/p9-task-batch-1-scope.json';
export const P9_BATCH_1_SCOPE_MD = 'docs/P9_TASK_BATCH_1_SCOPE.md';
export const P9_BATCH_1_SCOPE_GATE_JSON = 'docs/p9-task-batch-1-scope-gate.json';
export const P9_BATCH_1_SCOPE_GATE_MD = 'docs/P9_TASK_BATCH_1_SCOPE_GATE.md';

const REQUIRED_FILES = [
  'docs/P9_OWNER_SCOPE_DECISION.md',
  P9_OWNER_SCOPE_DECISION_JSON,
  'docs/P9_SCOPE_DISCOVERY.md',
  P9_SCOPE_DISCOVERY_JSON,
  'docs/P9_EXECUTION_PLAN.md',
  P9_EXECUTION_PLAN_JSON,
  P9_BATCH_1_SCOPE_MD,
  P9_BATCH_1_SCOPE_JSON,
];

const EXPECTED_TASK_IDS = ['P9-501', 'P9-502', 'P9-503', 'P9-504', 'P9-505', 'P9-506'];

function git(args) {
  try {
    return execFileSync('git', args, { cwd: process.cwd(), encoding: 'utf8' }).trim();
  } catch {
    return '';
  }
}

function nonEmptyArray(value) {
  return Array.isArray(value) && value.length > 0;
}

function taskId(task = {}) {
  return String(task.taskId || task.id || '').trim();
}

function sameStringList(actual = [], expected = []) {
  const a = [...actual].map(String).sort();
  const b = [...expected].map(String).sort();
  return a.length === b.length && a.every((value, idx) => value === b[idx]);
}

function taskMetadataComplete(task = {}) {
  return [
    taskId(task),
    task.taskName || task.title || '',
    task.workstream || task.workstreamId || '',
    task.batch,
    task.dependencies,
    task.deliverables,
    task.acceptanceCriteriaIds,
    task.evidencePaths,
    task.gateIds,
    task.status,
  ].every((value) => {
    if (Array.isArray(value)) return value.length > 0;
    if (typeof value === 'boolean') return value;
    return String(value || '').trim().length > 0;
  });
}

function flattenPlanTasks(plan = {}) {
  return (Array.isArray(plan.workstreams) ? plan.workstreams : []).flatMap((ws) =>
    (Array.isArray(ws.tasks) ? ws.tasks : []).map((task) => ({
      ...task,
      workstreamId: ws.workstreamId || ws.id || '',
      workstreamType: ws.workstreamType || ws.kind || '',
      batch: task.batch ?? ws.batch ?? null,
      batchId: task.batchId || ws.batchId || '',
    })),
  );
}

export function validateP9Batch1ScopeBundle({
  ownerDecision = {},
  discovery = {},
  plan = {},
  scope = {},
  currentBranch,
  currentHead,
  headDetached,
  stagedFileCount,
} = {}) {
  const branch = currentBranch ?? git(['branch', '--show-current']);
  const head = currentHead ?? git(['rev-parse', 'HEAD']);
  const detached = headDetached ?? git(['rev-parse', '--abbrev-ref', 'HEAD']) === 'HEAD';
  const staged = stagedFileCount ?? (git(['diff', '--cached', '--name-only']) === '' ? 0 : git(['diff', '--cached', '--name-only']).split('\n').length);
  const missingFiles = REQUIRED_FILES.filter((rel) => !fs.existsSync(path.join(process.cwd(), rel)));
  const planTasks = flattenPlanTasks(plan);
  const planBatch1Tasks = planTasks.filter((task) => Number(task.batch) === 1);
  const scopeTasks = Array.isArray(scope.tasks) ? scope.tasks : [];
  const scopeTaskIdsField = Array.isArray(scope.taskIds) ? scope.taskIds.map((value) => String(value).trim()) : scopeTasks.map(taskId);
  const ownerDecisionApproved =
    ownerDecision.decisionStatus === 'approved' &&
    ownerDecision.approvedByRole === 'Owner' &&
    ownerDecision.approvalSource === 'explicit_owner_instruction' &&
    ownerDecision.canonicalScopeResolved === true &&
    ownerDecision.scopeConfidence === 'high';
  const scopeTaskIds = scopeTasks.map(taskId);
  const taskMetadataCoverage = scopeTasks.every(taskMetadataComplete);
  const taskStatusCoverage = scopeTasks.every((task) => task.status === 'planned' && task.implementationStarted === false);
  const scopeTaskIdsMatchPlan = sameStringList(scopeTaskIdsField, EXPECTED_TASK_IDS) && sameStringList(scopeTaskIdsField, planBatch1Tasks.map(taskId));
  const requiredFlags = [
    ['requiredFilesPresent', missingFiles.length === 0],
    ['ownerDecisionApproved', ownerDecisionApproved],
    ['canonicalScopeResolved', discovery.canonicalScopeResolved === true && scope.canonicalScopeResolved === true],
    ['scopeConfidence', discovery.scopeConfidence === 'high' && scope.scopeConfidence === 'high'],
    ['batchId', scope.batchId === 'P9-TASK-BATCH-1'],
    ['batchName', typeof scope.batchName === 'string' && scope.batchName.length > 0],
    ['canonicalProductScope', typeof scope.canonicalProductScope === 'string' && scope.canonicalProductScope.length > 0],
    ['taskIds', scopeTaskIdsMatchPlan],
    ['exactTaskCount', scopeTaskIdsField.length === 6],
    ['domainModelsIncluded', scope.domainModelsIncluded === true],
    ['migrationsIncluded', scope.migrationsIncluded === true],
    ['repositoriesIncluded', scope.repositoriesIncluded === true],
    ['constraintsIncluded', scope.constraintsIncluded === true],
    ['persistenceTestsIncluded', scope.persistenceTestsIncluded === true],
    ['calibrationServiceExcluded', scope.calibrationServiceExcluded === true],
    ['syncOrchestratorExcluded', scope.syncOrchestratorExcluded === true],
    ['workerExcluded', scope.workerExcluded === true],
    ['apiExcluded', scope.apiExcluded === true],
    ['adminUiExcluded', scope.adminUiExcluded === true],
    ['realProviderExcluded', scope.realProviderExcluded === true],
    ['realOAuthExcluded', scope.realOAuthExcluded === true],
    ['realCredentialsExcluded', scope.realCredentialsExcluded === true],
    ['realInventoryReadExcluded', scope.realInventoryReadExcluded === true],
    ['realInventoryWriteExcluded', scope.realInventoryWriteExcluded === true],
    ['batch1ScopeReady', scope.scopeReady === true],
    ['implementationStarted', scope.implementationStarted === false],
    ['productCodeChanged', scope.productCodeChanged === false],
    ['productionReady', scope.productionReady === false],
    ['p10BoundaryPreserved', scope.p10BoundaryPreserved === true],
    ['allowedAreas', nonEmptyArray(scope.allowedAreas)],
    ['forbiddenAreas', nonEmptyArray(scope.forbiddenAreas)],
    ['testingRequirements', nonEmptyArray(scope.testingRequirements)],
    ['gateRequirements', nonEmptyArray(scope.gateRequirements)],
    ['evidenceRequirements', nonEmptyArray(scope.evidenceRequirements)],
    ['notDoing', nonEmptyArray(scope.notDoing)],
    ['p10ReservedBoundary', nonEmptyArray(scope.p10ReservedBoundary)],
    ['gitOperationConstraints', nonEmptyArray(scope.gitOperationConstraints)],
    ['taskMetadataCoverage', taskMetadataCoverage],
    ['taskStatusCoverage', taskStatusCoverage],
    ['planBatch1Aligned', sameStringList(planBatch1Tasks.map(taskId), EXPECTED_TASK_IDS)],
    ['planBatch1Count', planBatch1Tasks.length === 6],
    ['currentBranch', branch === 'dev'],
    ['headDetached', detached === false],
    ['stagedFileCount', staged === 0],
  ];

  const failed = requiredFlags.filter(([, ok]) => !ok).map(([id]) => id);
  return {
    status: failed.length ? 'failed' : 'passed',
    failed,
    failedCount: failed.length,
    currentBranch: branch,
    currentHead: head,
    headDetached: detached,
    stagedFileCount: staged,
    canonicalScopeResolved: discovery.canonicalScopeResolved === true && scope.canonicalScopeResolved === true,
    scopeConfidence: scope.scopeConfidence || discovery.scopeConfidence || '',
    batchId: scope.batchId || '',
    batchName: scope.batchName || '',
    taskIds: scopeTaskIdsField,
    taskCount: scopeTaskIdsField.length,
    batch1TaskIdsMatchPlan: scopeTaskIdsMatchPlan,
    batch1TaskCount: planBatch1Tasks.length,
    domainModelsIncluded: scope.domainModelsIncluded === true,
    migrationsIncluded: scope.migrationsIncluded === true,
    repositoriesIncluded: scope.repositoriesIncluded === true,
    constraintsIncluded: scope.constraintsIncluded === true,
    persistenceTestsIncluded: scope.persistenceTestsIncluded === true,
    calibrationServiceExcluded: scope.calibrationServiceExcluded === true,
    syncOrchestratorExcluded: scope.syncOrchestratorExcluded === true,
    workerExcluded: scope.workerExcluded === true,
    apiExcluded: scope.apiExcluded === true,
    adminUiExcluded: scope.adminUiExcluded === true,
    realProviderExcluded: scope.realProviderExcluded === true,
    realOAuthExcluded: scope.realOAuthExcluded === true,
    realCredentialsExcluded: scope.realCredentialsExcluded === true,
    realInventoryReadExcluded: scope.realInventoryReadExcluded === true,
    realInventoryWriteExcluded: scope.realInventoryWriteExcluded === true,
    batch1ScopeReady: scope.scopeReady === true,
    implementationStarted: scope.implementationStarted === true,
    productCodeChanged: scope.productCodeChanged === true,
    p10BoundaryPreserved: scope.p10BoundaryPreserved === true,
    productionReady: scope.productionReady === true,
    failedCountValue: failed.length,
    checks: requiredFlags.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  };
}

export function buildP9Batch1ScopeGateReport(bundle = {}) {
  const ownerDecision = bundle.ownerDecision ?? readJSON(P9_OWNER_SCOPE_DECISION_JSON) ?? {};
  const discovery = bundle.discovery ?? readJSON(P9_SCOPE_DISCOVERY_JSON) ?? {};
  const plan = bundle.plan ?? readJSON(P9_EXECUTION_PLAN_JSON) ?? {};
  const scope = bundle.scope ?? readJSON(P9_BATCH_1_SCOPE_JSON) ?? {};
  const validation = validateP9Batch1ScopeBundle({
    ownerDecision,
    discovery,
    plan,
    scope,
    currentBranch: bundle.currentBranch,
    currentHead: bundle.currentHead,
    headDetached: bundle.headDetached,
    stagedFileCount: bundle.stagedFileCount,
  });

  return {
    phase: 'P9',
    gate: 'P9-TASK-BATCH-1-SCOPE',
    status: validation.status,
    checkedAt: new Date().toISOString(),
    ownerDecisionId: ownerDecision.decisionId || '',
    batchId: validation.batchId,
    batchName: validation.batchName,
    currentBranch: validation.currentBranch,
    currentHead: validation.currentHead,
    headDetached: validation.headDetached,
    stagedFileCount: validation.stagedFileCount,
    canonicalScopeResolved: validation.canonicalScopeResolved,
    scopeConfidence: validation.scopeConfidence,
    taskIds: validation.taskIds,
    taskCount: validation.taskCount,
    batch1TaskIdsMatchPlan: validation.batch1TaskIdsMatchPlan,
    batch1TaskCount: validation.batch1TaskCount,
    domainModelsIncluded: validation.domainModelsIncluded,
    migrationsIncluded: validation.migrationsIncluded,
    repositoriesIncluded: validation.repositoriesIncluded,
    constraintsIncluded: validation.constraintsIncluded,
    persistenceTestsIncluded: validation.persistenceTestsIncluded,
    calibrationServiceExcluded: validation.calibrationServiceExcluded,
    syncOrchestratorExcluded: validation.syncOrchestratorExcluded,
    workerExcluded: validation.workerExcluded,
    apiExcluded: validation.apiExcluded,
    adminUiExcluded: validation.adminUiExcluded,
    realProviderExcluded: validation.realProviderExcluded,
    realOAuthExcluded: validation.realOAuthExcluded,
    realCredentialsExcluded: validation.realCredentialsExcluded,
    realInventoryReadExcluded: validation.realInventoryReadExcluded,
    realInventoryWriteExcluded: validation.realInventoryWriteExcluded,
    batch1ScopeReady: validation.batch1ScopeReady,
    implementationStarted: validation.implementationStarted,
    productCodeChanged: validation.productCodeChanged,
    p10BoundaryPreserved: validation.p10BoundaryPreserved,
    productionReady: validation.productionReady,
    failedCount: validation.failedCount,
    failed: validation.failed,
    checks: validation.checks,
  };
}

export function writeP9Batch1ScopeGateReport(report) {
  writeJSON(P9_BATCH_1_SCOPE_GATE_JSON, report);
  writeMarkdown(
    P9_BATCH_1_SCOPE_GATE_MD,
    `# P9 Batch 1 Scope Gate

Status: **${report.status}**

- Batch id: ${report.batchId}
- Batch name: ${report.batchName}
- Current branch: ${report.currentBranch}
- Current head: ${report.currentHead}
- Head detached: ${report.headDetached}
- Staged files: ${report.stagedFileCount}
- Canonical scope resolved: ${report.canonicalScopeResolved}
- Scope confidence: ${report.scopeConfidence}
- Task ids: ${report.taskIds.join(', ')}
- Task count: ${report.taskCount}
- Batch 1 task ids match plan: ${report.batch1TaskIdsMatchPlan}
- Batch 1 task count: ${report.batch1TaskCount}
- Domain models included: ${report.domainModelsIncluded}
- Migrations included: ${report.migrationsIncluded}
- Repositories included: ${report.repositoriesIncluded}
- Constraints included: ${report.constraintsIncluded}
- Persistence tests included: ${report.persistenceTestsIncluded}
- Calibration service excluded: ${report.calibrationServiceExcluded}
- Sync orchestrator excluded: ${report.syncOrchestratorExcluded}
- Worker excluded: ${report.workerExcluded}
- API excluded: ${report.apiExcluded}
- Admin UI excluded: ${report.adminUiExcluded}
- Real provider excluded: ${report.realProviderExcluded}
- Real OAuth excluded: ${report.realOAuthExcluded}
- Real credentials excluded: ${report.realCredentialsExcluded}
- Real inventory read excluded: ${report.realInventoryReadExcluded}
- Real inventory write excluded: ${report.realInventoryWriteExcluded}
- Batch 1 scope ready: ${report.batch1ScopeReady}
- Implementation started: ${report.implementationStarted}
- Product code changed: ${report.productCodeChanged}
- P10 boundary preserved: ${report.p10BoundaryPreserved}
- Production Ready: ${report.productionReady}
- Failed checks: ${report.failedCount ? report.failed.join(', ') : 'none'}

This gate validates the Batch 1 scope artifact only. It does not authorize domain model, migration, repository, calibration, sync orchestration, API, or Admin UI implementation.
`,
  );
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = buildP9Batch1ScopeGateReport();
  writeP9Batch1ScopeGateReport(report);
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}
