import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';
import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

export const P8_TASK_BATCH_5_EVIDENCE_JSON = 'docs/p8-task-batch-5-platform-draft-adapters.json';
export const P8_TASK_BATCH_5_GATE_JSON = 'docs/p8-task-batch-5-final-gate.json';
export const P8_TASK_BATCH_5_GATE_MD = 'docs/P8_TASK_BATCH_5_FINAL_GATE.md';

const requiredFiles = [
  'backend/internal/modules/operationtask/execution_services.go',
  'backend/internal/modules/operationtask/platform_draft_adapters.go',
  'backend/internal/modules/operationtask/platform_draft_adapters_test.go',
  'docs/P8_TASK_BATCH_5_PLATFORM_DRAFT_ADAPTERS.md',
  'docs/p8-task-batch-5-platform-draft-adapters.json',
  'scripts/p8-task-batch-5-final-gate.mjs',
  'tests/gates/p8/task-batch-5.mjs',
];

function rootPath(rel) {
  return path.join(process.cwd(), rel);
}

function text(rel) {
  try {
    return fs.readFileSync(rootPath(rel), 'utf8');
  } catch {
    return '';
  }
}

function hasAll(value, needles) {
  return needles.every((needle) => value.includes(needle));
}

function git(args) {
  try {
    return execFileSync('git', args, { cwd: process.cwd(), encoding: 'utf8' }).trim();
  } catch {
    return '';
  }
}

export function validateP8TaskBatch5Bundle({ evidence = {}, sources = {} } = {}) {
  const executionText = sources.executionText ?? text('backend/internal/modules/operationtask/execution_services.go');
  const adapterText = sources.adapterText ?? text('backend/internal/modules/operationtask/platform_draft_adapters.go');
  const testsText = sources.testsText ?? text('backend/internal/modules/operationtask/platform_draft_adapters_test.go');
  const packageText = sources.packageText ?? text('package.json');
  const docsText = sources.docsText ?? text('docs/P8_TASK_BATCH_5_PLATFORM_DRAFT_ADAPTERS.md');
  const combined = `${executionText}\n${adapterText}\n${testsText}\n${docsText}`;

  const checks = [
    ['batchId', evidence.batchId === 'P8-TASK-BATCH-5'],
    ['P8-301 status', evidence.tasks?.['P8-301']?.status === 'completed'],
    ['P8-302 status', evidence.tasks?.['P8-302']?.status === 'completed'],
    ['P8-303 status', evidence.tasks?.['P8-303']?.status === 'completed'],
    ['P8-304 status', evidence.tasks?.['P8-304']?.status === 'completed'],
    ['P8-305 status', evidence.tasks?.['P8-305']?.status === 'completed'],
    ['draftExecutionPortReused', evidence.draftExecutionPortReused === true && executionText.includes('type DraftExecutionPort interface') && !adapterText.includes('type PlatformDraftAdapter interface')],
    ['parallelAdapterInterfaceAbsent', evidence.parallelAdapterInterfaceAbsent === true && !combined.includes('type PlatformDraftAdapter interface')],
    ['platformDraftAdapterRegistryPresent', evidence.platformDraftAdapterRegistryPresent === true && hasAll(adapterText, ['type PlatformDraftAdapterRegistry struct', 'func (r *PlatformDraftAdapterRegistry) ExecuteDraft'])],
    ['draftAdapterCapabilitiesPresent', evidence.draftAdapterCapabilitiesPresent === true && hasAll(adapterText, ['type DraftAdapterCapabilities struct', 'DraftCreation', 'Publish', 'Listing', 'NetworkAccess', 'RealCredentials', 'AutomaticExecution'])],
    ['localDraftAdapterPresent', evidence.localDraftAdapterPresent === true && hasAll(adapterText, ['type LocalDraftAdapter struct', 'func NewLocalDraftAdapter', 'local_draft'])],
    ['douyinFixtureAdapterPresent', evidence.douyinFixtureAdapterPresent === true && hasAll(adapterText, ['type DouyinDraftFixtureAdapter struct', 'func NewDouyinDraftFixtureAdapter', 'mock:douyin', 'sandbox:douyin'])],
    ['unsupportedPlatformGuardPresent', evidence.unsupportedPlatformGuardPresent === true && adapterText.includes('func UnsupportedPlatformGuard')],
    ['automaticPublishGuardPresent', evidence.automaticPublishGuardPresent === true && adapterText.includes('func AutomaticPublishGuard')],
    ['credentialAbsenceGuardPresent', evidence.credentialAbsenceGuardPresent === true && adapterText.includes('func CredentialAbsenceGuard')],
    ['capabilitiesSafe', evidence.draftCreationCapability === true && evidence.publishCapability === false && evidence.listingCapability === false && evidence.networkAccessCapability === false && evidence.realCredentialsCapability === false && evidence.automaticExecutionCapability === false],
    ['adapterContractTestsPassed', evidence.adapterContractTestsPassed === true && testsText.includes('TestPlatformDraftAdapterContractCapabilitiesAndReferences')],
    ['localAdapterTestsPassed', evidence.localAdapterTestsPassed === true && testsText.includes('TestLocalDraftAdapterIdempotencyAndModeSafety')],
    ['douyinMockSandboxTestsPassed', evidence.douyinMockSandboxTestsPassed === true && testsText.includes('TestDouyinDraftFixtureAdapterScenariosAndValidation')],
    ['unsupportedPlatformGuardTestsPassed', evidence.unsupportedPlatformGuardTestsPassed === true && testsText.includes('TestUnsupportedPlatformGuardAndRegistryResolution')],
    ['automaticPublishGuardTestsPassed', evidence.automaticPublishGuardTestsPassed === true && testsText.includes('TestAutomaticPublishGuardBlocksDangerousConfigAndPayloadBeforeAdapter')],
    ['networkIsolationTestsPassed', evidence.networkIsolationTestsPassed === true && testsText.includes('TestPlatformDraftAdaptersSourceHasNoNetworkClientDependency') && !adapterText.includes('net/http') && !adapterText.includes('http.Client') && !adapterText.includes('http.NewRequest')],
    ['idempotencyTestsPassed', evidence.idempotencyTestsPassed === true && testsText.includes('TestLocalDraftAdapterIdempotencyAndModeSafety') && testsText.includes('ErrCodeIdemPayloadConflict')],
    ['concurrencyTestsPassed', evidence.concurrencyTestsPassed === true && testsText.includes('TestPlatformDraftAdapterRegistryConcurrentCallsAreStable')],
    ['orchestratorIntegrationTestsPassed', evidence.orchestratorIntegrationTestsPassed === true && testsText.includes('TestExecutionOrchestratorWithSafeRegistryWritesDraftAndRejectsGuardFailure')],
    ['racePassed', evidence.racePassed === true && evidence.dataRaces === 0],
    ['draftWrittenNotPublished', evidence.draftWrittenNotPublished === true && docsText.includes('draft_written != published') && docsText.includes('draft_written != listed')],
    ['realDouyinApiImplemented', evidence.realDouyinApiImplemented === false],
    ['oauthImplemented', evidence.oauthImplemented === false],
    ['networkAccessEnabled', evidence.networkAccessEnabled === false],
    ['realCredentialsEnabled', evidence.realCredentialsEnabled === false],
    ['realPlatformWriteImplemented', evidence.realPlatformWriteImplemented === false],
    ['automaticPublishImplemented', evidence.automaticPublishImplemented === false && !combined.includes('PublishProduct') && !combined.includes('PublishDraft') && !combined.includes('AutoPublish')],
    ['automaticListingImplemented', evidence.automaticListingImplemented === false && !combined.includes('ListProduct') && !combined.includes('AutoList')],
    ['apiImplemented', evidence.apiImplemented === false],
    ['adminUiImplemented', evidence.adminUiImplemented === false],
    ['backgroundWorkerImplemented', evidence.backgroundWorkerImplemented === false],
    ['productionPlatformAdapterImplemented', evidence.productionPlatformAdapterImplemented === false && !combined.includes('ProductionAdapter')],
    ['p7DeferredPerformancePreserved', evidence.p7DeferredPerformancePreserved === true],
    ['p10ProductionBoundaryPreserved', evidence.p10ProductionBoundaryPreserved === true],
    ['productionReady', evidence.productionReady === false],
    ['packageScriptsRegistered', hasAll(packageText, ['test:p8-task-batch-5', 'p8:task-batch-5-gate'])],
  ];

  const failed = checks.filter(([, ok]) => !ok).map(([id]) => id);
  return {
    status: failed.length ? 'failed' : 'passed',
    failed,
    failedCount: failed.length,
    checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  };
}

export function buildP8TaskBatch5GateReport(bundle = {}) {
  const evidence = bundle.evidence ?? readJSON(P8_TASK_BATCH_5_EVIDENCE_JSON) ?? {};
  const validation = validateP8TaskBatch5Bundle({ evidence, sources: bundle.sources });
  const missingFiles = requiredFiles.filter((rel) => !fs.existsSync(rootPath(rel)));
  const stagedFiles = git(['diff', '--cached', '--name-only']);
  const currentBranch = git(['branch', '--show-current']);
  const failed = [
    ...validation.failed,
    ...missingFiles.map((rel) => `missing:${rel}`),
  ];
  if (currentBranch !== 'dev') failed.push('currentBranch');
  if (stagedFiles !== '') failed.push('stagedFileCount');
  return {
    phase: 'P8',
    gate: 'P8-TASK-BATCH-5',
    status: failed.length ? 'failed' : 'passed',
    checkedAt: '2026-07-26T00:00:00.000Z',
    batchId: 'P8-TASK-BATCH-5',
    tasks: ['P8-301', 'P8-302', 'P8-303', 'P8-304', 'P8-305'],
    currentBranch,
    stagedFileCount: stagedFiles === '' ? 0 : stagedFiles.split('\n').length,
    platformDraftAdapterRegistryPresent: evidence.platformDraftAdapterRegistryPresent === true,
    localDraftAdapterPresent: evidence.localDraftAdapterPresent === true,
    douyinFixtureAdapterPresent: evidence.douyinFixtureAdapterPresent === true,
    unsupportedPlatformGuardPresent: evidence.unsupportedPlatformGuardPresent === true,
    automaticPublishGuardPresent: evidence.automaticPublishGuardPresent === true,
    credentialAbsenceGuardPresent: evidence.credentialAbsenceGuardPresent === true,
    networkAccessEnabled: evidence.networkAccessEnabled === true,
    realCredentialsEnabled: evidence.realCredentialsEnabled === true,
    realDouyinApiImplemented: evidence.realDouyinApiImplemented === true,
    oauthImplemented: evidence.oauthImplemented === true,
    realPlatformWriteImplemented: evidence.realPlatformWriteImplemented === true,
    automaticPublishImplemented: evidence.automaticPublishImplemented === true,
    automaticListingImplemented: evidence.automaticListingImplemented === true,
    apiImplemented: evidence.apiImplemented === true,
    adminUiImplemented: evidence.adminUiImplemented === true,
    backgroundWorkerImplemented: evidence.backgroundWorkerImplemented === true,
    productionReady: evidence.productionReady === true,
    racePassed: evidence.racePassed === true,
    dataRaces: evidence.dataRaces ?? null,
    failedCount: failed.length,
    failed,
    checks: validation.checks,
  };
}

export function writeP8TaskBatch5GateReport(report) {
  writeJSON(P8_TASK_BATCH_5_GATE_JSON, report);
  writeMarkdown(
    P8_TASK_BATCH_5_GATE_MD,
    `# P8 Task Batch 5 Final Gate\n\nStatus: **${report.status}**\n\n- Batch: ${report.batchId}\n- Tasks: ${report.tasks.join(', ')}\n- Current branch: ${report.currentBranch}\n- Staged files: ${report.stagedFileCount}\n- Registry present: ${report.platformDraftAdapterRegistryPresent}\n- Local adapter present: ${report.localDraftAdapterPresent}\n- Douyin fixture adapter present: ${report.douyinFixtureAdapterPresent}\n- Unsupported platform guard present: ${report.unsupportedPlatformGuardPresent}\n- Automatic publish guard present: ${report.automaticPublishGuardPresent}\n- Credential absence guard present: ${report.credentialAbsenceGuardPresent}\n- Race passed: ${report.racePassed}\n- Data races: ${report.dataRaces}\n- Network access enabled: ${report.networkAccessEnabled}\n- Real credentials enabled: ${report.realCredentialsEnabled}\n- Real Douyin API implemented: ${report.realDouyinApiImplemented}\n- OAuth implemented: ${report.oauthImplemented}\n- Real platform write implemented: ${report.realPlatformWriteImplemented}\n- Automatic publish implemented: ${report.automaticPublishImplemented}\n- Automatic listing implemented: ${report.automaticListingImplemented}\n- API implemented: ${report.apiImplemented}\n- Admin UI implemented: ${report.adminUiImplemented}\n- Background worker implemented: ${report.backgroundWorkerImplemented}\n- Production Ready: ${report.productionReady}\n- Failed checks: ${report.failedCount ? report.failed.join(', ') : 'none'}\n\nThis gate validates only P8 Batch 5 safe platform draft adapter boundaries. It does not authorize real Douyin calls, OAuth, real credentials, network access, real platform writes, automatic publish, automatic listing, API, Admin UI, background workers, production tag, production release, or Production Ready.\n`,
  );
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = buildP8TaskBatch5GateReport();
  writeP8TaskBatch5GateReport(report);
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}
