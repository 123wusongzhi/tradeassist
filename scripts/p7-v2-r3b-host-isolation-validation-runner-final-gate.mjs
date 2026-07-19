import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { readJSON, root, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';
import {
  EXPECTED,
  FORBIDDEN_ARGS,
  HOST_ISOLATION_VALIDATION_RUNNER_VERSION,
  RUN_ORDER,
  assertFixedRunPlan,
  assertNoFifthRun,
  buildSelfTestFixtureMatrix,
  runSelfTest,
  validateMatrixEvidenceSchema,
} from './p7-v2-r3b-host-isolation-validation-runner.mjs';
import { FORMAL_HOST_ISOLATION_VERSION } from './p7-v2-r3b-formal-host-isolation.mjs';

const OUT_JSON = 'docs/p7-v2-r3b-host-isolation-validation-runner-final-gate.json';
const OUT_MD = 'docs/P7_V2_R3B_HOST_ISOLATION_VALIDATION_RUNNER_FINAL_GATE.md';
const RUNNER_PATH = 'scripts/p7-v2-r3b-host-isolation-validation-runner.mjs';
const FIXTURE_PATH = 'tests/gates/p7-v2/host-isolation-validation-runner.mjs';

function hasPkgScript(pkg, name, command) {
  return pkg.scripts?.[name] === command;
}

function sourceContainsAll(source, needles) {
  return needles.filter((needle) => !source.includes(needle));
}

function assertThrows(fn) {
  try {
    fn();
    return false;
  } catch {
    return true;
  }
}

export function evaluateHostIsolationValidationRunnerFinalGate() {
  const pkg = readJSON('package.json') || {};
  const runnerSource = fs.readFileSync(path.join(root, RUNNER_PATH), 'utf8');
  const fixtureExists = fs.existsSync(path.join(root, FIXTURE_PATH));
  const fixtureMatrix = buildSelfTestFixtureMatrix();
  const schema = validateMatrixEvidenceSchema(fixtureMatrix);
  const selfTest = runSelfTest();
  const forbiddenCoverage = FORBIDDEN_ARGS.every((arg) => runnerSource.includes(arg));
  const missingIntegrationNeedles = sourceContainsAll(runnerSource, [
    'validateFormalHostIsolationContract',
    'datasetBarrier',
    'deterministicWarmup',
    'applicationCooldown',
    'waitForHostQuietWindow',
    'hostReadinessFingerprint',
    'startP7V2Server',
    'runDataset',
    'scripts/p7-v2-load.mjs',
    'formalPlanCreated: false',
    'runtimeFreezeCreated: false',
    'formalPairStarted: false',
    'baselineBinaryRebuilt: false',
    'currentBinaryRebuilt: false',
    'processRows',
    'countOtherDiagnosticRunners',
    '[g]o (build|test)',
    '[p]g_dump',
  ]);
  const missingStateNeedles = sourceContainsAll(runnerSource, [
    'resource_precheck',
    'database_prepare',
    'dataset_build',
    'dataset_barrier',
    'application_start',
    'warmup',
    'cooldown',
    'quiet_window',
    'measurement',
    'application_stop',
    'connection_drain',
  ]);
  const completedFlags = ['B1Completed', 'C1Completed', 'C2Completed', 'B2Completed'].every((key) => key in fixtureMatrix);
  const checks = {
    packageRunnerScriptRegistered: hasPkgScript(pkg, 'p7-v2:r3b:host-isolation-validation', 'node scripts/p7-v2-r3b-host-isolation-validation-runner.mjs'),
    packageRunnerGateScriptRegistered: hasPkgScript(pkg, 'p7-v2:r3b:host-isolation-validation-runner-gate', 'node scripts/p7-v2-r3b-host-isolation-validation-runner-final-gate.mjs'),
    packageRunnerFixtureScriptRegistered: hasPkgScript(pkg, 'test:p7-v2-host-isolation-validation-runner', 'node tests/gates/p7-v2/host-isolation-validation-runner.mjs'),
    runnerVersionOne: HOST_ISOLATION_VALIDATION_RUNNER_VERSION === 1,
    formalHostIsolationVersionTwo: FORMAL_HOST_ISOLATION_VERSION === 2,
    fixedRunOrder: RUN_ORDER.map((entry) => entry.kind).join('-') === 'B-C-C-B' && assertFixedRunPlan(),
    fixedRunCount: RUN_ORDER.length === 4,
    fifthRunRejected: assertThrows(() => assertNoFifthRun(5)),
    forbiddenOverrideCoverage: forbiddenCoverage,
    runnerFixtureExists: fixtureExists,
    selfTestPassed: selfTest.status === 'passed',
    fixtureSchemaValid: schema.status === 'passed',
    fixtureValidForFormalPlanFromAnalysis: fixtureMatrix.validForFormalPlan === true,
    fixtureBusinessRuntimeUnchanged: fixtureMatrix.businessRuntimeChanged === false,
    fixtureLoadContractUnchanged: fixtureMatrix.loadContractChanged === false,
    fixtureNoFormalArtifactsCreated: fixtureMatrix.formalPlanCreated === false && fixtureMatrix.runtimeFreezeCreated === false && fixtureMatrix.formalPairStarted === false,
    fixtureMatrixCompletedFlagsPresent: completedFlags,
    binaryShaPinsHardcoded: runnerSource.includes(EXPECTED.baselineBinarySha256) && runnerSource.includes(EXPECTED.currentBinarySha256),
    realContractIntegrationPresent: missingIntegrationNeedles.length === 0,
    runnerStateMachinePresent: missingStateNeedles.length === 0,
  };
  const failedChecks = Object.entries(checks).filter(([, passed]) => passed !== true).map(([id]) => id);
  return {
    phase: 'P7-V2-R3B-HOST-ISOLATION-VALIDATION-RUNNER-FINAL-GATE',
    status: failedChecks.length ? 'failed' : 'passed',
    hostIsolationValidationRunnerVersion: HOST_ISOLATION_VALIDATION_RUNNER_VERSION,
    formalHostIsolationVersion: FORMAL_HOST_ISOLATION_VERSION,
    runOrder: RUN_ORDER.map((entry) => entry.kind).join('-'),
    runCount: RUN_ORDER.length,
    forbiddenOverrideCount: FORBIDDEN_ARGS.length,
    missingIntegrationNeedles,
    missingStateNeedles,
    selfTestStatus: selfTest.status,
    schemaStatus: schema.status,
    checks: Object.entries(checks).map(([id, passed]) => ({ id, status: passed ? 'passed' : 'failed' })),
    failedChecks,
    failed: failedChecks,
    failedCount: failedChecks.length,
    generatedAt: new Date().toISOString(),
  };
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const gate = evaluateHostIsolationValidationRunnerFinalGate();
  writeJSON(OUT_JSON, gate);
  writeMarkdown(
    OUT_MD,
    `# P7-V2-R3B Host Isolation Validation Runner Final Gate

Status: **${gate.status}**

- Runner version: \`${gate.hostIsolationValidationRunnerVersion}\`
- Host isolation version: \`${gate.formalHostIsolationVersion}\`
- Run order: \`${gate.runOrder}\`
- Run count: \`${gate.runCount}\`
- Forbidden override args covered: \`${gate.forbiddenOverrideCount}\`
- Self-test status: \`${gate.selfTestStatus}\`
- Schema status: \`${gate.schemaStatus}\`
- Failed checks: ${gate.failedCount ? gate.failedChecks.join(', ') : 'none'}
`,
  );
  console.log(JSON.stringify(gate, null, 2));
  process.exit(gate.status === 'passed' ? 0 : 1);
}
