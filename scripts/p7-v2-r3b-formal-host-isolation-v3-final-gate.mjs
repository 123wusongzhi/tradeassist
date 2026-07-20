import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';
import {
  FORMAL_HOST_ISOLATION_VERSION,
  HOST_ISOLATION_CONTRACT_PATH,
  HOST_ISOLATION_REPORT_PATH,
  validateFormalHostIsolationContract,
} from './p7-v2-r3b-formal-host-isolation.mjs';
import { HOST_ISOLATION_VALIDATION_RUNNER_VERSION } from './p7-v2-r3b-host-isolation-validation-runner.mjs';

const AUDIT_PATH = 'docs/p7-v2-r3b-host-isolation-v3-current-self-variance-audit.json';
const OUT_JSON = 'docs/p7-v2-r3b-formal-host-isolation-v3-final-gate.json';
const OUT_MD = 'docs/P7_V2_R3B_FORMAL_HOST_ISOLATION_V3_FINAL_GATE.md';

export function evaluateFormalHostIsolationV3FinalGate({
  audit = readJSON(AUDIT_PATH) || {},
  contract = readJSON(HOST_ISOLATION_CONTRACT_PATH) || {},
  repair = readJSON(HOST_ISOLATION_REPORT_PATH) || {},
} = {}) {
  const contractValidation = validateFormalHostIsolationContract(contract);
  const checks = {
    formalHostIsolationVersion: contract.formalHostIsolationVersion === 3 && FORMAL_HOST_ISOLATION_VERSION === 3,
    failedMatrixEvidencePreserved: audit.failedValidationMatrixId === 'p7v2-diag-host-isolation-validation-20260719061648' && audit.formalPlanAllowed === false,
    primaryRootCausePresent: typeof audit.primaryRootCause === 'string' && audit.primaryRootCause.startsWith('V3_'),
    repairMatchesRootCause: audit.primaryRootCause === 'V3_E_quiet_window_not_predictive_of_measurement_stability' && audit.repairPath === 'predictive_host_stability_barrier',
    businessRuntimeChanged: audit.businessRuntimeChangeRequired === false && repair.businessRuntimeChanged === false && contract.businessRuntimeChanged === false,
    binaryChanged: audit.binaryBindingPassed === true,
    inputChanged: audit.inputBindingPassed === true,
    loadContractChanged: repair.loadContractChanged === false && contract.loadContractChanged === false,
    selectedRepairImplemented: contract.predictiveHostStabilityBarrier?.predictiveHostStabilityBarrierVersion === 1,
    selectedRepairFixturesPassed: contractValidation.status === 'passed',
    runnerUsesV3Contract: HOST_ISOLATION_VALIDATION_RUNNER_VERSION === 2,
    validationGateUsesV3Contract: FORMAL_HOST_ISOLATION_VERSION === 3,
    formalPlanCreated: audit.formalPlanAllowed === false && repair.formalPairStarted === false,
    formalRerunStarted: repair.validationMatrixStarted === false,
  };
  const failedChecks = [
    ...Object.entries(checks).filter(([, passed]) => passed !== true).map(([id]) => id),
    ...contractValidation.issues,
  ];
  return {
    phase: 'P7-V2-R3B-HOST-ISOLATION-V3-FINAL-GATE',
    status: failedChecks.length ? 'failed' : 'passed',
    formalHostIsolationVersion: contract.formalHostIsolationVersion ?? null,
    hostIsolationValidationRunnerVersion: HOST_ISOLATION_VALIDATION_RUNNER_VERSION,
    failedMatrixEvidencePreserved: checks.failedMatrixEvidencePreserved,
    primaryRootCause: audit.primaryRootCause || '',
    repairPath: audit.repairPath || '',
    businessRuntimeChanged: repair.businessRuntimeChanged === true || contract.businessRuntimeChanged === true,
    binaryChanged: audit.binaryBindingPassed !== true,
    inputChanged: audit.inputBindingPassed !== true,
    loadContractChanged: repair.loadContractChanged === true || contract.loadContractChanged === true,
    failedChecks,
    failed: failedChecks,
    failedCount: failedChecks.length,
    checks: Object.entries(checks).map(([id, passed]) => ({ id, status: passed ? 'passed' : 'failed' })),
    generatedAt: new Date().toISOString(),
  };
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const gate = evaluateFormalHostIsolationV3FinalGate();
  writeJSON(OUT_JSON, gate);
  writeMarkdown(
    OUT_MD,
    `# P7-V2-R3B Host Isolation V3 Final Gate

Status: **${gate.status}**

- Formal host isolation version: \`${gate.formalHostIsolationVersion ?? 'missing'}\`
- Primary root cause: \`${gate.primaryRootCause || 'missing'}\`
- Repair path: \`${gate.repairPath || 'missing'}\`
- Failed checks: ${gate.failedCount ? gate.failedChecks.join(', ') : 'none'}

This gate validates only the V3 bounded repair. It does not create a formal plan, runtime freeze, formal pair, soak, demo, tag, release, or production readiness claim.
`,
  );
  console.log(JSON.stringify(gate, null, 2));
  process.exit(gate.status === 'passed' ? 0 : 1);
}
