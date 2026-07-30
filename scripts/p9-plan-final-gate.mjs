import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';

export const P9_OWNER_SCOPE_DECISION_JSON = 'docs/p9-owner-scope-decision.json';
export const P9_OWNER_SCOPE_DECISION_MD = 'docs/P9_OWNER_SCOPE_DECISION.md';
export const P9_SCOPE_DISCOVERY_JSON = 'docs/p9-scope-discovery.json';
export const P9_SCOPE_DISCOVERY_MD = 'docs/P9_SCOPE_DISCOVERY.md';
export const P9_EXECUTION_PLAN_JSON = 'docs/p9-execution-plan.json';
export const P9_EXECUTION_PLAN_MD = 'docs/P9_EXECUTION_PLAN.md';
export const P9_BATCH_1_SCOPE_JSON = 'docs/p9-task-batch-1-scope.json';
export const P9_BATCH_1_SCOPE_MD = 'docs/P9_TASK_BATCH_1_SCOPE.md';
export const P9_BATCH_1_SCOPE_GATE_JSON = 'docs/p9-task-batch-1-scope-gate.json';
export const P9_BATCH_1_SCOPE_GATE_MD = 'docs/P9_TASK_BATCH_1_SCOPE_GATE.md';
export const P9_PLAN_GATE_JSON = 'docs/p9-plan-final-gate.json';
export const P9_PLAN_GATE_MD = 'docs/P9_PLAN_FINAL_GATE.md';

const REPO_ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

function readJSON(rel) {
  try {
    return JSON.parse(fs.readFileSync(path.join(REPO_ROOT, rel), 'utf8'));
  } catch {
    return null;
  }
}

function writeJSON(rel, data) {
  const full = path.join(REPO_ROOT, rel);
  fs.mkdirSync(path.dirname(full), { recursive: true });
  fs.writeFileSync(full, `${JSON.stringify(data, null, 2)}\n`, 'utf8');
}

function writeMarkdown(rel, body) {
  const full = path.join(REPO_ROOT, rel);
  fs.mkdirSync(path.dirname(full), { recursive: true });
  fs.writeFileSync(full, body, 'utf8');
}

const REQUIRED_FILES = [
  P9_OWNER_SCOPE_DECISION_MD,
  P9_OWNER_SCOPE_DECISION_JSON,
  P9_SCOPE_DISCOVERY_MD,
  P9_SCOPE_DISCOVERY_JSON,
  P9_EXECUTION_PLAN_MD,
  P9_EXECUTION_PLAN_JSON,
  P9_BATCH_1_SCOPE_MD,
  P9_BATCH_1_SCOPE_JSON,
  P9_BATCH_1_SCOPE_GATE_MD,
  P9_BATCH_1_SCOPE_GATE_JSON,
  'docs/PROGRESS.md',
  'docs/README.md',
];

const PLANNING_TASK_IDS = ['P9-101', 'P9-102', 'P9-201', 'P9-202', 'P9-301', 'P9-302', 'P9-401', 'P9-402'];
const PRODUCT_BATCH_SPECS = [
  { workstreamId: 'WS-05', batch: 1, taskIds: ['P9-501', 'P9-502', 'P9-503', 'P9-504', 'P9-505', 'P9-506'] },
  { workstreamId: 'WS-06', batch: 2, taskIds: ['P9-601', 'P9-602', 'P9-603', 'P9-604', 'P9-605', 'P9-606'] },
  { workstreamId: 'WS-07', batch: 3, taskIds: ['P9-701', 'P9-702', 'P9-703', 'P9-704', 'P9-705', 'P9-706'] },
  { workstreamId: 'WS-08', batch: 4, taskIds: ['P9-801', 'P9-802', 'P9-803', 'P9-804'] },
  { workstreamId: 'WS-09', batch: 5, taskIds: ['P9-901', 'P9-902', 'P9-903', 'P9-904', 'P9-905'] },
  { workstreamId: 'WS-10', batch: 6, taskIds: ['P9-1001', 'P9-1002', 'P9-1003', 'P9-1004', 'P9-1005', 'P9-1006'] },
  { workstreamId: 'WS-11', batch: 7, taskIds: ['P9-1101', 'P9-1102', 'P9-1103', 'P9-1104', 'P9-1105'] },
];
const PRODUCT_TASK_IDS = PRODUCT_BATCH_SPECS.flatMap((batch) => batch.taskIds);
const PRODUCT_WORKSTREAM_IDS = PRODUCT_BATCH_SPECS.map((batch) => batch.workstreamId);
const PRODUCT_BATCH_IDS = PRODUCT_BATCH_SPECS.map((batch) => batch.batch);

function git(args) {
  try {
    return execFileSync('git', args, { cwd: REPO_ROOT, encoding: 'utf8' }).trim();
  } catch {
    return '';
  }
}

function gitIsAncestor(ancestor, descendant) {
  try {
    execFileSync('git', ['merge-base', '--is-ancestor', ancestor, descendant], { cwd: REPO_ROOT, stdio: 'ignore' });
    return true;
  } catch {
    return false;
  }
}

function nonEmptyArray(value) {
  return Array.isArray(value) && value.length > 0;
}

function taskId(task = {}) {
  return String(task.taskId || task.id || '').trim();
}

function taskStatus(task = {}) {
  return String(task.status || '').trim();
}

function taskBatch(task = {}) {
  return task.batch;
}

function taskWorkstreamType(task = {}) {
  return String(task.taskCategory || task.workstreamType || '').trim();
}

function unique(values) {
  return [...new Set(values)];
}

function sameStringList(actual = [], expected = []) {
  const a = [...actual].map(String).sort();
  const b = [...expected].map(String).sort();
  return a.length === b.length && a.every((value, idx) => value === b[idx]);
}

function flattenTasks(workstreams = []) {
  return (Array.isArray(workstreams) ? workstreams : []).flatMap((ws) =>
    (Array.isArray(ws.tasks) ? ws.tasks : []).map((task) => ({
      ...task,
      workstreamId: ws.workstreamId || ws.id || '',
      workstreamName: ws.workstreamName || ws.title || '',
      workstreamType: ws.workstreamType || ws.kind || task.workstreamType || task.taskCategory || '',
      batch: task.batch ?? ws.batch ?? null,
      batchId: task.batchId || ws.batchId || '',
    })),
  );
}

function classifyTasks(tasks = []) {
  return tasks.reduce(
    (acc, task) => {
      const category = taskWorkstreamType(task);
      if (category === 'planning_governance' || task.planningFoundation === true) {
        acc.planning.push(task);
      } else if (category === 'product_implementation' || task.planningFoundation === false) {
        acc.product.push(task);
      }
      return acc;
    },
    { planning: [], product: [] },
  );
}

function batchTaskIds(tasks = [], batchNumber) {
  return tasks.filter((task) => Number(taskBatch(task)) === Number(batchNumber)).map(taskId);
}

function compareArrayField(tasks = [], field) {
  return tasks.every((task) => nonEmptyArray(task[field]));
}

function hasTaskCoreFields(task = {}) {
  return [
    taskId(task),
    task.taskName || task.title || '',
    task.workstreamId || task.workstream || '',
    task.batch !== undefined && task.batch !== null,
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

export function validateP9PlanBundle({
  ownerDecision = {},
  discovery = {},
  plan = {},
  batch1Scope = {},
  batch1ScopeGate = {},
  currentBranch,
  currentHead,
  headDetached,
  stagedFileCount,
  discoveryHeadIsAncestor,
} = {}) {
  const workstreams = Array.isArray(plan.workstreams) ? plan.workstreams : [];
  const allTasks = flattenTasks(workstreams);
  const { planning, product } = classifyTasks(allTasks);
  const branch = currentBranch ?? git(['branch', '--show-current']);
  const head = currentHead ?? git(['rev-parse', 'HEAD']);
  const detached = headDetached ?? git(['rev-parse', '--abbrev-ref', 'HEAD']) === 'HEAD';
  const staged = stagedFileCount ?? (git(['diff', '--cached', '--name-only']) === '' ? 0 : git(['diff', '--cached', '--name-only']).split('\n').length);
  const missingFiles = REQUIRED_FILES.filter((rel) => !fs.existsSync(path.join(REPO_ROOT, rel)));
  const ownerDecisionApproved =
    ownerDecision.decisionStatus === 'approved' &&
    ownerDecision.approvedByRole === 'Owner' &&
    ownerDecision.approvalSource === 'explicit_owner_instruction' &&
    ownerDecision.canonicalScopeResolved === true &&
    ownerDecision.scopeConfidence === 'high';
  const planningTaskIdsPreserved = sameStringList(planning.map(taskId), PLANNING_TASK_IDS);
  const productTaskIdsPreserved = sameStringList(product.map(taskId), PRODUCT_TASK_IDS);
  const planningWorkstreams = workstreams.filter((ws) => ws.workstreamType === 'planning_governance');
  const productWorkstreams = workstreams.filter((ws) => ws.workstreamType === 'product_implementation');
  const batchCounts = unique(product.map((task) => Number(taskBatch(task))));
  const planningCompleted = planning.every((task) => taskStatus(task) === 'completed');
  const planningEvidenceCoverage = planning.every((task) => nonEmptyArray(task.evidencePaths));
  const planningGateCoverage = planning.every((task) => nonEmptyArray(task.gateIds));
  const productPlanned = product.every((task) => taskStatus(task) === 'planned');
  const productDependenciesCovered = compareArrayField(product, 'dependencies');
  const productDeliverablesCovered = compareArrayField(product, 'deliverables');
  const productAcceptanceCovered = compareArrayField(product, 'acceptanceCriteriaIds');
  const productEvidenceCovered = compareArrayField(product, 'evidencePaths');
  const productGateCoverage = compareArrayField(product, 'gateIds');
  const productBatchCoverage = product.every((task) => Number(taskBatch(task)) >= 1 && Number(taskBatch(task)) <= 7 && String(task.batchId || '').length > 0);
  const productImplementationStarted = plan.productImplementationStarted === true || product.some((task) => task.implementationStarted === true);
  const productCompletedTaskCount = product.filter((task) => taskStatus(task) === 'completed').length;
  const productStartedPlanCompatible = productImplementationStarted ? product.some((task) => task.implementationStarted === true || taskStatus(task) === 'completed') : true;
  const productStatusCompatible = productImplementationStarted ? product.every((task) => ['planned', 'in_progress', 'completed'].includes(taskStatus(task))) : productPlanned;
  const productCompletedTaskCountCompatible = productImplementationStarted ? productCompletedTaskCount > 0 : productCompletedTaskCount === 0;
  const productImplementationFileCountCompatible = productImplementationStarted ? Number(plan.p9ProductImplementationFileCount || 0) > 0 : Number(plan.p9ProductImplementationFileCount || 0) === 0;
  const recordedPlanHead = String(plan.currentHead || plan.p9DiscoveryBaseHead || '').trim();
  const ancestryCompatible = discoveryHeadIsAncestor ?? (recordedPlanHead !== '' && gitIsAncestor(recordedPlanHead, head));
  const discoveryHeadCompatible = productImplementationStarted
    ? recordedPlanHead !== '' && String(plan.p9DiscoveryBaseHead || '').length > 0 && ancestryCompatible
    : plan.p9DiscoveryBaseHead === head;
  const duplicateTaskIdCount = allTasks.length - unique(allTasks.map(taskId)).length;
  const planningProductOverlap = product.map(taskId).filter((id) => PLANNING_TASK_IDS.includes(id)).length;
  const batch1Ids = batchTaskIds(product, 1);
  const batch1ScopeIds = Array.isArray(batch1Scope.taskIds) ? batch1Scope.taskIds.map(String) : [];
  const batch1TaskIdsExactlyMatch = sameStringList(batch1Ids, batch1ScopeIds) && sameStringList(batch1Ids, PRODUCT_BATCH_SPECS[0].taskIds);
  const batch1TaskCount = batch1Ids.length;
  const batch1ScopeCreated = batch1Scope.scopeReady === true || plan.batch1ScopeReady === true;
  const batch1ScopeGatePassed = batch1ScopeGate.status === 'passed';
  const batch1ScopeBatchMatch = batch1Scope.batchId === 'P9-TASK-BATCH-1' && Number(batch1Scope.batch) === 1;
  const batch1ScopeTaskCount = Number(batch1Scope.taskCount || 0);
  const allTasksHaveCoreFields = allTasks.every(hasTaskCoreFields);
  const planningFoundationPreserved =
    plan.planningFoundationPreserved === true &&
    plan.historicalPlanningTaskIdsPreserved === true &&
    planningTaskIdsPreserved &&
    planningCompleted &&
    planningEvidenceCoverage &&
    planningGateCoverage;
  const failedCountChecks = [
    ['requiredFilesPresent', missingFiles.length === 0],
    ['ownerDecisionPresent', ownerDecisionApproved],
    ['canonicalScopeResolved', discovery.canonicalScopeResolved === true],
    ['scopeConfidence', discovery.scopeConfidence === 'high'],
    ['ownerScopeDecisionCreated', discovery.ownerScopeDecisionCreated === true],
    ['planningFoundationCompleted', discovery.planningFoundationCompleted === true],
    ['fullImplementationPlanCompleted', discovery.fullImplementationPlanCompleted === true],
    ['planningFoundationPreserved', planningFoundationPreserved],
    ['historicalPlanningTaskIdsPreserved', plan.historicalPlanningTaskIdsPreserved === true],
    ['legacyPlanningTaskIdsPreserved', plan.legacyPlanningTaskIdsPreserved === true && plan.historicalPlanningTaskIdsPreserved === true],
    ['planningTaskIdsPreserved', planningTaskIdsPreserved],
    ['productTaskIdsPreserved', productTaskIdsPreserved],
    ['planningWorkstreamCount', planningWorkstreams.length === 4],
    ['productImplementationWorkstreamCount', productWorkstreams.length === 7],
    ['productImplementationBatchCount', batchCounts.length === 7],
    ['productTaskCount', product.length > 0],
    ['planningFoundationTaskCount', planning.length === 8],
    ['batch1TaskCount', batch1TaskCount === 6],
    ['batch1TaskIdsExactlyMatch', batch1TaskIdsExactlyMatch],
    ['batch1ScopeCreated', batch1ScopeCreated],
    ['batch1ScopeGatePassed', batch1ScopeGatePassed],
    ['batch1ScopeBatchMatch', batch1ScopeBatchMatch],
    ['batch1ScopeTaskCount', batch1ScopeTaskCount === 6],
    ['allTasksHaveDependencies', productDependenciesCovered],
    ['allTasksHaveDeliverables', productDeliverablesCovered],
    ['allTasksHaveAcceptanceMapping', productAcceptanceCovered],
    ['allTasksHaveEvidenceMapping', productEvidenceCovered],
    ['allTasksHaveGateMapping', productGateCoverage],
    ['allTasksHaveCoreFields', allTasksHaveCoreFields],
    ['allTasksPlannedOrCompletedAsExpected', planningCompleted && productStatusCompatible],
    ['allProductTasksHaveBatchMapping', productBatchCoverage],
    ['productImplementationStarted', productStartedPlanCompatible],
    ['productCompletedTaskCount', productCompletedTaskCountCompatible],
    ['duplicateTaskIdCount', duplicateTaskIdCount === 0],
    ['productTaskIdCollisionCount', planningProductOverlap === 0],
    ['ownerDecisionRequired', discovery.ownerDecisionRequired === false],
    ['p9ProductImplementationFileCount', productImplementationFileCountCompatible],
    ['p10BoundaryPreserved', plan.p10BoundaryPreserved === true],
    ['productionReady', plan.productionReady === false],
    ['currentBranch', branch === 'dev'],
    ['headDetached', detached === false],
    ['stagedFileCount', staged === 0],
    ['discoveryHeadMatch', discoveryHeadCompatible],
  ];

  const failed = failedCountChecks.filter(([, ok]) => !ok).map(([id]) => id);
  return {
    status: failed.length ? 'failed' : 'passed',
    failed,
    failedCount: failed.length,
    currentBranch: branch,
    currentHead: head,
    headDetached: detached,
    stagedFileCount: staged,
    ownerScopeDecisionPresent: ownerDecisionApproved,
    ownerScopeDecisionApproved: ownerDecisionApproved,
    canonicalScopeResolved: discovery.canonicalScopeResolved === true,
    scopeConfidence: discovery.scopeConfidence || '',
    ownerDecisionRequired: discovery.ownerDecisionRequired === true,
    planningFoundationPreserved,
    historicalPlanningTaskIdsPreserved: plan.historicalPlanningTaskIdsPreserved === true,
    legacyPlanningTaskIdsPreserved: plan.legacyPlanningTaskIdsPreserved === true && plan.historicalPlanningTaskIdsPreserved === true,
    planningFoundationTaskIdsPreserved: planningTaskIdsPreserved,
    productTaskIdsPreserved,
    planningFoundationTaskCount: planning.length,
    planningFoundationCompletedTaskCount: planning.filter((task) => taskStatus(task) === 'completed').length,
    planningWorkstreamCount: planningWorkstreams.length,
    productImplementationWorkstreamCount: productWorkstreams.length,
    productImplementationBatchCount: batchCounts.length,
    productTaskCount: product.length,
    productCompletedTaskCount,
    batch1TaskIds: batch1Ids,
    batch1TaskCount,
    batch1TaskIdsExactlyMatch,
    batch1ScopeCreated,
    batch1ScopeGatePassed,
    batch1ScopeBatchMatch,
    duplicateTaskIdCount,
    productTaskIdCollisionCount: planningProductOverlap,
    allTasksHaveDependencies: productDependenciesCovered,
    allTasksHaveDeliverables: productDeliverablesCovered,
    allTasksHaveAcceptanceMapping: productAcceptanceCovered,
    allTasksHaveEvidenceMapping: productEvidenceCovered,
    allTasksHaveGateMapping: productGateCoverage,
    allTasksHaveCoreFields,
    allTasksPlannedOrCompletedAsExpected: planningCompleted && productStatusCompatible,
    allProductTasksHaveBatchMapping: productBatchCoverage,
    productImplementationStarted,
    p9ProductImplementationFileCount: plan.p9ProductImplementationFileCount || 0,
    p10BoundaryPreserved: plan.p10BoundaryPreserved === true,
    productionReady: plan.productionReady === true,
    discoveryHeadMatch: discoveryHeadCompatible,
    checks: failedCountChecks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  };
}

export function buildP9PlanGateReport(bundle = {}) {
  const ownerDecision = bundle.ownerDecision ?? readJSON(P9_OWNER_SCOPE_DECISION_JSON) ?? {};
  const discovery = bundle.discovery ?? readJSON(P9_SCOPE_DISCOVERY_JSON) ?? {};
  const plan = bundle.plan ?? readJSON(P9_EXECUTION_PLAN_JSON) ?? {};
  const batch1Scope = bundle.batch1Scope ?? readJSON(P9_BATCH_1_SCOPE_JSON) ?? {};
  const batch1ScopeGate = bundle.batch1ScopeGate ?? readJSON(P9_BATCH_1_SCOPE_GATE_JSON) ?? {};
  const validation = validateP9PlanBundle({
    ownerDecision,
    discovery,
    plan,
    batch1Scope,
    batch1ScopeGate,
    currentBranch: bundle.currentBranch,
    currentHead: bundle.currentHead,
    headDetached: bundle.headDetached,
    stagedFileCount: bundle.stagedFileCount,
    discoveryHeadIsAncestor: bundle.discoveryHeadIsAncestor,
  });

  return {
    phase: 'P9',
    gate: 'P9-PLAN',
    status: validation.status,
    checkedAt: new Date().toISOString(),
    ownerDecisionId: ownerDecision.decisionId || '',
    discoveryBaseHead: plan.p9DiscoveryBaseHead || discovery.p9DiscoveryBaseHead || '',
    canonicalPhaseName: plan.canonicalPhaseName || '',
    currentBranch: validation.currentBranch,
    currentHead: validation.currentHead,
    headDetached: validation.headDetached,
    stagedFileCount: validation.stagedFileCount,
    canonicalScopeResolved: validation.canonicalScopeResolved,
    scopeConfidence: validation.scopeConfidence,
    ownerScopeDecisionPresent: validation.ownerScopeDecisionPresent,
    ownerScopeDecisionApproved: validation.ownerScopeDecisionApproved,
    ownerDecisionRequired: validation.ownerDecisionRequired,
    planningFoundationPreserved: validation.planningFoundationPreserved,
    historicalPlanningTaskIdsPreserved: validation.historicalPlanningTaskIdsPreserved,
    legacyPlanningTaskIdsPreserved: validation.legacyPlanningTaskIdsPreserved,
    planningFoundationTaskIdsPreserved: validation.planningFoundationTaskIdsPreserved,
    productTaskIdsPreserved: validation.productTaskIdsPreserved,
    planningFoundationTaskCount: validation.planningFoundationTaskCount,
    planningFoundationCompletedTaskCount: validation.planningFoundationCompletedTaskCount,
    planningWorkstreamCount: validation.planningWorkstreamCount,
    productImplementationWorkstreamCount: validation.productImplementationWorkstreamCount,
    productImplementationBatchCount: validation.productImplementationBatchCount,
    productTaskCount: validation.productTaskCount,
    productCompletedTaskCount: validation.productCompletedTaskCount,
    batch1TaskIds: validation.batch1TaskIds,
    batch1TaskCount: validation.batch1TaskCount,
    batch1TaskIdsExactlyMatch: validation.batch1TaskIdsExactlyMatch,
    batch1ScopeCreated: validation.batch1ScopeCreated,
    batch1ScopeGatePassed: validation.batch1ScopeGatePassed,
    batch1ScopeBatchMatch: validation.batch1ScopeBatchMatch,
    duplicateTaskIdCount: validation.duplicateTaskIdCount,
    productTaskIdCollisionCount: validation.productTaskIdCollisionCount,
    allTasksHaveDependencies: validation.allTasksHaveDependencies,
    allTasksHaveDeliverables: validation.allTasksHaveDeliverables,
    allTasksHaveAcceptanceMapping: validation.allTasksHaveAcceptanceMapping,
    allTasksHaveEvidenceMapping: validation.allTasksHaveEvidenceMapping,
    allTasksHaveGateMapping: validation.allTasksHaveGateMapping,
    allTasksHaveCoreFields: validation.allTasksHaveCoreFields,
    allTasksPlannedOrCompletedAsExpected: validation.allTasksPlannedOrCompletedAsExpected,
    allProductTasksHaveBatchMapping: validation.allProductTasksHaveBatchMapping,
    productImplementationStarted: validation.productImplementationStarted,
    p9ProductImplementationFileCount: validation.p9ProductImplementationFileCount,
    p10BoundaryPreserved: validation.p10BoundaryPreserved,
    productionReady: validation.productionReady,
    failedCount: validation.failedCount,
    failed: validation.failed,
    checks: validation.checks,
  };
}

export function writeP9PlanGateReport(report) {
  writeJSON(P9_PLAN_GATE_JSON, report);
  writeMarkdown(
    P9_PLAN_GATE_MD,
    `# P9 Plan Final Gate

Status: **${report.status}**

- Owner decision id: ${report.ownerDecisionId}
- Canonical phase name: ${report.canonicalPhaseName}
- Discovery base head: ${report.discoveryBaseHead}
- Current branch: ${report.currentBranch}
- Current head: ${report.currentHead}
- Head detached: ${report.headDetached}
- Staged files: ${report.stagedFileCount}
- Canonical scope resolved: ${report.canonicalScopeResolved}
- Scope confidence: ${report.scopeConfidence}
- Owner scope decision present: ${report.ownerScopeDecisionPresent}
- Owner scope decision approved: ${report.ownerScopeDecisionApproved}
- Owner decision required: ${report.ownerDecisionRequired}
- Planning foundation preserved: ${report.planningFoundationPreserved}
- Historical planning task ids preserved: ${report.historicalPlanningTaskIdsPreserved}
- Legacy planning task ids preserved: ${report.legacyPlanningTaskIdsPreserved}
- Planning foundation task ids preserved: ${report.planningFoundationTaskIdsPreserved}
- Product task ids preserved: ${report.productTaskIdsPreserved}
- Planning foundation task count: ${report.planningFoundationTaskCount}
- Planning foundation completed task count: ${report.planningFoundationCompletedTaskCount}
- Planning workstreams: ${report.planningWorkstreamCount}
- Product implementation workstreams: ${report.productImplementationWorkstreamCount}
- Product implementation batches: ${report.productImplementationBatchCount}
- Product tasks: ${report.productTaskCount}
- Product completed task count: ${report.productCompletedTaskCount}
- Batch 1 task ids: ${report.batch1TaskIds.join(', ')}
- Batch 1 task count: ${report.batch1TaskCount}
- Batch 1 task ids exactly match: ${report.batch1TaskIdsExactlyMatch}
- Batch 1 scope created: ${report.batch1ScopeCreated}
- Batch 1 scope gate passed: ${report.batch1ScopeGatePassed}
- Duplicate task id count: ${report.duplicateTaskIdCount}
- Product task id collision count: ${report.productTaskIdCollisionCount}
- All tasks have dependencies: ${report.allTasksHaveDependencies}
- All tasks have deliverables: ${report.allTasksHaveDeliverables}
- All tasks have acceptance mapping: ${report.allTasksHaveAcceptanceMapping}
- All tasks have evidence mapping: ${report.allTasksHaveEvidenceMapping}
- All tasks have gate mapping: ${report.allTasksHaveGateMapping}
- All tasks have core fields: ${report.allTasksHaveCoreFields}
- All product tasks have batch mapping: ${report.allProductTasksHaveBatchMapping}
- Product implementation started: ${report.productImplementationStarted}
- P9 product implementation files: ${report.p9ProductImplementationFileCount}
- P10 boundary preserved: ${report.p10BoundaryPreserved}
- Production Ready: ${report.productionReady}
- Failed checks: ${report.failedCount ? report.failed.join(', ') : 'none'}

This gate validates the planning foundation and the full P9 implementation plan while keeping Batch 1 scoped and the P10 boundary intact.
`,
  );
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = buildP9PlanGateReport();
  writeP9PlanGateReport(report);
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}
