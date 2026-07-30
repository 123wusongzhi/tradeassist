import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';

export const P9_SCOPE_DISCOVERY_JSON = 'docs/p9-scope-discovery.json';
export const P9_SCOPE_DISCOVERY_MD = 'docs/P9_SCOPE_DISCOVERY.md';
export const P9_OWNER_SCOPE_DECISION_JSON = 'docs/p9-owner-scope-decision.json';
export const P9_OWNER_SCOPE_DECISION_MD = 'docs/P9_OWNER_SCOPE_DECISION.md';
export const P9_ENTRY_GATE_JSON = 'docs/p9-entry-gate-report.json';
export const P9_ENTRY_GATE_MD = 'docs/P9_ENTRY_GATE_REPORT.md';

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
  P9_SCOPE_DISCOVERY_MD,
  P9_SCOPE_DISCOVERY_JSON,
  P9_OWNER_SCOPE_DECISION_MD,
  P9_OWNER_SCOPE_DECISION_JSON,
  'docs/P8_DEVELOPMENT_CLOSURE.md',
  'docs/p8-development-closure.json',
  'docs/P8_TASK_BATCH_9_FINAL_GATE.md',
  'docs/p8-task-batch-9-final-gate.json',
];

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

function counts(references = []) {
  return references.reduce(
    (acc, ref) => {
      if (ref?.classification === 'current') acc.current += 1;
      if (ref?.classification === 'historical') acc.historical += 1;
      if (ref?.classification === 'completed') acc.completed += 1;
      if (ref?.classification === 'conflicting') acc.conflicting += 1;
      return acc;
    },
    { current: 0, historical: 0, completed: 0, conflicting: 0 },
  );
}

export function validateP9EntryBundle({
  discovery = {},
  ownerDecision = {},
  p8Closure = {},
  p8TaskBatch9Gate = {},
  currentBranch,
  currentHead,
  headDetached,
  stagedFileCount,
  discoveryHeadIsAncestor,
} = {}) {
  const refs = Array.isArray(discovery.references) ? discovery.references : [];
  const refCounts = counts(refs);
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
  const p8ClosurePassed =
    p8Closure.status === 'Development Complete' &&
    p8Closure.productionReady === false &&
    p8Closure.finalProductionAcceptanceDeferredToP10 === true &&
    p8Closure.p10ProductionBoundaryPreserved === true;
  const p8FinalGatePassed =
    p8TaskBatch9Gate.status === 'passed' && p8TaskBatch9Gate.productionReady === false;
  const productImplementationStarted = discovery.implementationStarted === true || discovery.productImplementationStarted === true;
  const implementationStartedCompatible = productImplementationStarted ? discovery.implementationStarted === true && discovery.productImplementationStarted === true : discovery.implementationStarted === false && discovery.productImplementationStarted !== true;
  const recordedDiscoveryHead = String(discovery.currentHead || discovery.p9DiscoveryBaseHead || '').trim();
  const ancestryCompatible = discoveryHeadIsAncestor ?? (recordedDiscoveryHead !== '' && gitIsAncestor(recordedDiscoveryHead, head));
  const discoveryHeadMatch = productImplementationStarted
    ? recordedDiscoveryHead !== '' && String(discovery.p9DiscoveryBaseHead || '').length > 0 && ancestryCompatible
    : discovery.p9DiscoveryBaseHead === head;
  const productImplementationFileCountCompatible = productImplementationStarted ? Number(discovery.p9ProductImplementationFileCount || 0) > 0 : Number(discovery.p9ProductImplementationFileCount || 0) === 0;

  const checks = [
    ['requiredFilesPresent', missingFiles.length === 0],
    ['p8DevelopmentClosurePassed', p8ClosurePassed],
    ['p8FinalGatePassed', p8FinalGatePassed],
    ['currentBranch', branch === 'dev'],
    ['headDetached', detached === false],
    ['stagedFileCount', staged === 0],
    ['discoveryStatus', discovery.discoveryStatus === 'completed' || discovery.status === 'scope_discovery_resolved'],
    ['discoveryHeadMatch', discoveryHeadMatch],
    ['ownerScopeDecisionCreated', discovery.ownerScopeDecisionCreated === true],
    ['ownerDecisionApproved', ownerDecisionApproved],
    ['canonicalScopeResolved', discovery.canonicalScopeResolved === true],
    ['scopeConfidence', discovery.scopeConfidence === 'high'],
    ['ownerDecisionRequired', discovery.ownerDecisionRequired === false],
    ['historicalPhase9Reused', discovery.historicalPhase9Reused === false],
    ['conflictingP9ReferenceCount', discovery.conflictingP9ReferenceCount === 0 && refCounts.conflicting === 0],
    ['referenceClassifications', refCounts.current > 0 && refCounts.historical > 0 && refCounts.completed > 0],
    ['planningFoundationCompleted', discovery.planningFoundationCompleted === true],
    ['fullImplementationPlanCompleted', discovery.fullImplementationPlanCompleted === true],
    ['p9ProductImplementationFileCount', productImplementationFileCountCompatible],
    ['implementationStarted', implementationStartedCompatible],
    ['p10BoundaryPreserved', discovery.p10BoundaryPreserved === true],
    ['productionReady', discovery.productionReady === false],
  ];

  const failed = checks.filter(([, ok]) => !ok).map(([id]) => id);
  return {
    status: failed.length ? 'blocked' : 'allowed',
    failed,
    failedCount: failed.length,
    currentBranch: branch,
    currentHead: head,
    headDetached: detached,
    stagedFileCount: staged,
    workingTreeDirty: discovery.workingTreeDirty === true,
    p8DevelopmentClosurePassed: p8ClosurePassed,
    p8FinalGatePassed,
    discoveryHeadMatch,
    ownerScopeDecisionCreated: discovery.ownerScopeDecisionCreated === true,
    ownerDecisionApproved,
    planningFoundationCompleted: discovery.planningFoundationCompleted === true,
    fullImplementationPlanCompleted: discovery.fullImplementationPlanCompleted === true,
    canonicalScopeResolved: discovery.canonicalScopeResolved === true,
    scopeConfidence: discovery.scopeConfidence || '',
    ownerDecisionRequired: discovery.ownerDecisionRequired === true,
    historicalPhase9Reused: discovery.historicalPhase9Reused === true,
    historicalP9ReferenceCount: discovery.historicalP9ReferenceCount || refs.length,
    activeP9ReferenceCount: discovery.activeP9ReferenceCount || refCounts.current,
    completedHistoricalP9ReferenceCount: discovery.completedHistoricalP9ReferenceCount || refCounts.completed,
    supersededP9ReferenceCount: discovery.supersededP9ReferenceCount || 0,
    conflictingP9ReferenceCount: discovery.conflictingP9ReferenceCount || refCounts.conflicting,
    p9ProductImplementationFileCount: discovery.p9ProductImplementationFileCount || 0,
    implementationStarted: discovery.implementationStarted === true,
    p10BoundaryPreserved: discovery.p10BoundaryPreserved === true,
    productionReady: discovery.productionReady === true,
    checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  };
}

export function buildP9EntryGateReport(bundle = {}) {
  const discovery = bundle.discovery ?? readJSON(P9_SCOPE_DISCOVERY_JSON) ?? {};
  const ownerDecision = bundle.ownerDecision ?? readJSON(P9_OWNER_SCOPE_DECISION_JSON) ?? {};
  const p8Closure = bundle.p8Closure ?? readJSON('docs/p8-development-closure.json') ?? {};
  const p8TaskBatch9Gate = bundle.p8TaskBatch9Gate ?? readJSON('docs/p8-task-batch-9-final-gate.json') ?? {};
  const validation = validateP9EntryBundle({
    discovery,
    ownerDecision,
    p8Closure,
    p8TaskBatch9Gate,
    currentBranch: bundle.currentBranch,
    currentHead: bundle.currentHead,
    headDetached: bundle.headDetached,
    stagedFileCount: bundle.stagedFileCount,
    discoveryHeadIsAncestor: bundle.discoveryHeadIsAncestor,
  });

  return {
    phase: 'P9',
    gate: 'P9-ENTRY',
    status: validation.status,
    checkedAt: new Date().toISOString(),
    discoveryBaseHead: discovery.p9DiscoveryBaseHead || '',
    ownerDecisionId: ownerDecision.decisionId || '',
    currentBranch: validation.currentBranch,
    currentHead: validation.currentHead,
    headDetached: validation.headDetached,
    stagedFileCount: validation.stagedFileCount,
    workingTreeDirty: validation.workingTreeDirty,
    discoveryHeadMatch: validation.discoveryHeadMatch,
    p8DevelopmentClosurePassed: validation.p8DevelopmentClosurePassed,
    p8FinalGatePassed: validation.p8FinalGatePassed,
    canonicalScopeResolved: validation.canonicalScopeResolved,
    scopeConfidence: validation.scopeConfidence,
    planningFoundationCompleted: validation.planningFoundationCompleted,
    ownerScopeDecisionCreated: validation.ownerScopeDecisionCreated,
    ownerDecisionApproved: validation.ownerDecisionApproved,
    ownerDecisionRequired: validation.ownerDecisionRequired,
    historicalP9ReferenceCount: validation.historicalP9ReferenceCount,
    activeP9ReferenceCount: validation.activeP9ReferenceCount,
    completedHistoricalP9ReferenceCount: validation.completedHistoricalP9ReferenceCount,
    supersededP9ReferenceCount: validation.supersededP9ReferenceCount,
    conflictingP9ReferenceCount: validation.conflictingP9ReferenceCount,
    p9ProductImplementationFileCount: validation.p9ProductImplementationFileCount,
    implementationStarted: validation.implementationStarted,
    p10BoundaryPreserved: validation.p10BoundaryPreserved,
    productionReady: validation.productionReady,
    readyForPhaseP9Plan: validation.status === 'allowed',
    failedCount: validation.failedCount,
    failed: validation.failed,
    checks: validation.checks,
  };
}

export function writeP9EntryGateReport(report) {
  writeJSON(P9_ENTRY_GATE_JSON, report);
  writeMarkdown(
    P9_ENTRY_GATE_MD,
    `# P9 Entry Gate

Status: **${report.status}**

- Discovery base head: ${report.discoveryBaseHead}
- Current branch: ${report.currentBranch}
- Current head: ${report.currentHead}
- Head detached: ${report.headDetached}
- Staged files: ${report.stagedFileCount}
- Working tree dirty: ${report.workingTreeDirty}
- P8 development closure passed: ${report.p8DevelopmentClosurePassed}
- P8 final gate passed: ${report.p8FinalGatePassed}
- Discovery head matches live HEAD: ${report.discoveryHeadMatch}
- Owner scope decision created: ${report.ownerScopeDecisionCreated}
- Owner decision approved: ${report.ownerDecisionApproved}
- Canonical scope resolved: ${report.canonicalScopeResolved}
- Scope confidence: ${report.scopeConfidence}
- Owner decision required: ${report.ownerDecisionRequired}
- Historical P9 references: ${report.historicalP9ReferenceCount}
- Active P9 references: ${report.activeP9ReferenceCount}
- Completed historical P9 references: ${report.completedHistoricalP9ReferenceCount}
- Superseded P9 references: ${report.supersededP9ReferenceCount}
- Conflicting P9 references: ${report.conflictingP9ReferenceCount}
- Planning foundation completed: ${report.planningFoundationCompleted}
- Full implementation plan completed: ${report.fullImplementationPlanCompleted}
- P9 product implementation files: ${report.p9ProductImplementationFileCount}
- Implementation started: ${report.implementationStarted}
- P10 boundary preserved: ${report.p10BoundaryPreserved}
- Production Ready: ${report.productionReady}
- Failed checks: ${report.failedCount ? report.failed.join(', ') : 'none'}

This gate only validates discovery and boundary conditions. It does not authorize P9 product code.
`,
  );
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = buildP9EntryGateReport();
  writeP9EntryGateReport(report);
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'allowed' ? 0 : 1);
}
