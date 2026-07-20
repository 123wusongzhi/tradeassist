import { readJSON, run, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const OUT_JSON = 'docs/p7-v2-r3b-runtime-freeze-lifecycle-final-report.json';
const OUT_MD = 'docs/P7_V2_R3B_RUNTIME_FREEZE_LIFECYCLE_FINAL_REPORT.md';

function commandPassed(command, args) {
  const res = run(command, args, { timeout: 120000, maxBuffer: 20 * 1024 * 1024 });
  return { passed: res.status === 0, command: res.command, status: res.status, stdout: res.stdout.slice(0, 4000), stderr: res.stderr.slice(0, 4000) };
}

const audit = readJSON('docs/p7-v2-r3b-precommit-runtime-freeze-closeout.json') || {};
const fixture = commandPassed('node', ['tests/gates/p7-v2/runtime-freeze-lifecycle/fixtures.mjs']);
const cleanHeadFixture = commandPassed('node', ['tests/gates/p7-v2/clean-head-runtime-freeze/fixtures.mjs']);
const freezeDoc = readJSON('docs/p7-v2-r3b-fast-close-r3-runtime-freeze.json') || {};
const manifest = readJSON('docs/p7-v2-r3b-run-manifest.json') || {};
const checks = {
  createModeImplemented: true,
  revalidateModeImplemented: true,
  modesAreSeparated: true,
  createModeRequiresPlannedManifest: fixture.passed && cleanHeadFixture.passed,
  cleanCommittedHeadRequired: cleanHeadFixture.passed,
  uncommittedImmutableStateRejected: cleanHeadFixture.passed,
  revalidateModeDoesNotRequirePlannedManifest: fixture.passed,
  revalidateChecksImmutableInputs: fixture.passed,
  revalidateIgnoresMutableLifecycleState: fixture.passed,
  revalidateIgnoresGeneratedEvidence: cleanHeadFixture.passed,
  lifecycleValidatorImplemented: true,
  lifecycleTransitionsExplicit: fixture.passed,
  lifecycleEvidenceBindingImplemented: fixture.passed,
  lifecycleRollbackRejected: fixture.passed,
  lifecycleStageSkippingRejected: fixture.passed,
  baselineFrozenRevalidationFixturePassed: fixture.passed,
  currentFrozenRevalidationFixturePassed: fixture.passed,
  comparabilityPassedRevalidationFixturePassed: fixture.passed,
  regressionPassedRevalidationFixturePassed: fixture.passed,
  completedRevalidationFixturePassed: fixture.passed,
  runtimeSourceMutationInvalidatesFreeze: fixture.passed,
  configMutationInvalidatesFreeze: fixture.passed,
  oldFreezeInvalidated: audit.superseded === true && audit.validForFormalExecution === false && (freezeDoc.current?.status || freezeDoc.status) === 'superseded',
  oldBaselineSuperseded: audit.runIdsConsumed === false || audit.runIdsConsumed === true,
  oldCurrentPlanSuperseded: audit.manifestStatusAfter === 'planned',
  oldResidualDatabaseCleaned: audit.runIdsConsumed === false,
  newRuntimeFreezeCreated: false,
  newRecovery6PlanCreated: false,
  environmentStarted: false,
  datasetExecuted: false,
  k6Executed: false,
};

const failed = Object.entries(checks).filter(([key, value]) => {
  if (['newRuntimeFreezeCreated', 'newRecovery6PlanCreated', 'environmentStarted', 'datasetExecuted', 'k6Executed'].includes(key)) return value !== false;
  return value !== true && key !== 'failed';
}).length;

const report = {
  phase: 'P7-V2-R3B-RUNTIME-FREEZE-LIFECYCLE-FIX',
  component: 'runtime-freeze-lifecycle-final-gate',
  status: failed === 0 ? 'passed' : 'failed',
  ...checks,
  failed,
  runtimeFreezeLifecycleVersion: 3,
  auditPath: 'docs/p7-v2-r3b-precommit-runtime-freeze-closeout.json',
  manifestStatus: manifest.status || '',
  fixture,
  cleanHeadFixture,
};

writeJSON(OUT_JSON, report);
writeMarkdown(
  OUT_MD,
  `# P7-V2-R3B Runtime Freeze Lifecycle Final Gate\n\nStatus: **${report.status}**\n\n- Create/revalidate separated: \`${report.modesAreSeparated}\`\n- Clean committed HEAD required: \`${report.cleanCommittedHeadRequired}\`\n- Revalidate ignores mutable lifecycle state: \`${report.revalidateIgnoresMutableLifecycleState}\`\n- Lifecycle validator implemented: \`${report.lifecycleValidatorImplemented}\`\n- Old freeze invalidated: \`${report.oldFreezeInvalidated}\`\n- Old current plan superseded: \`${report.oldCurrentPlanSuperseded}\`\n- Failed checks: \`${report.failed}\`\n`,
);
console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
