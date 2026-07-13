import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');

function readJSON(rel) {
  return JSON.parse(fs.readFileSync(path.join(root, rel), 'utf8'));
}

function writeJSON(rel, value) {
  fs.writeFileSync(path.join(root, rel), `${JSON.stringify(value, null, 2)}\n`, 'utf8');
}

function writeText(rel, value) {
  fs.writeFileSync(path.join(root, rel), value, 'utf8');
}

function exists(rel) {
  return fs.existsSync(path.join(root, rel));
}

function reportStatus(rel) {
  if (!exists(rel)) return 'missing';
  const value = readJSON(rel);
  return value.status || value.summary?.status || 'unknown';
}

function packageStatus(pkg) {
  const clean = pkg.replace('./', 'backend/').replace('/...', '');
  return fs.existsSync(path.join(root, clean)) ? 'mapped' : 'missing';
}

const sourceAudit = readJSON('docs/p7-v-capability-completeness-audit.json');
const sourcePartials = sourceAudit.capabilities.filter((item) => item.status === 'partial');
const closureItems = sourceAudit.capabilities.map((item) => ({
  capabilityId: item.id,
  capabilityName: item.capability,
  mandatory: true,
  previousStatus: item.status,
  previousReason: item.gap || item.resultThisRound || '',
  codeLocation: item.codeLocation || '',
  testLocation: item.unitTestLocation || '',
  runtimeEvidence: item.runtimeVerification || '',
  actualGap:
    item.status === 'partial'
      ? 'P7-C runtime evidence and/or broader module adoption is still pending.'
      : 'No new P7-C code gap identified by the closure script.',
  implementationPlan:
    item.status === 'partial'
      ? 'Close with real code changes plus runtime report evidence; do not mark implemented from static scan alone.'
      : 'Keep existing evidence and rerun P7-C gate.',
  finalStatus: item.status,
  finalEvidence: item.status === 'partial' ? 'pending P7-C runtime evidence' : item.resultThisRound || 'carried from P7-V audit',
}));

const closureAudit = {
  phase: 'P7-C',
  status: sourcePartials.length === 0 ? 'passed' : 'incomplete',
  generatedAt: new Date().toISOString(),
  sourceAudit: 'docs/p7-v-capability-completeness-audit.json',
  summary: {
    total: closureItems.length,
    mandatoryImplemented: closureItems.filter((item) => item.finalStatus === 'implemented').length,
    mandatoryPartial: closureItems.filter((item) => item.finalStatus === 'partial').length,
    mandatoryMissing: closureItems.filter((item) => item.finalStatus === 'missing').length,
    notApplicable: closureItems.filter((item) => item.finalStatus === 'not_applicable').length,
  },
  capabilities: closureItems,
};

const racePackages = [
  ['pagination/cursor', './internal/pkg/pagination/...'],
  ['ratelimit', './internal/pkg/ratelimit/...'],
  ['cache/singleflight', './internal/pkg/cache/...'],
  ['httpclient/provider limiter', './internal/pkg/httpclient/...'],
  ['taskcenter/worker', './internal/modules/taskcenter/...'],
  ['webhook', './internal/modules/webhook/...'],
  ['inventory', './internal/modules/inventory/...'],
  ['auth', './internal/modules/admin/...'],
  ['export/files', './internal/modules/exportmod/...'],
  ['operationlog', './internal/modules/operationlog/...'],
  ['dataset generator runtime', './cmd/p7load/...'],
].map(([capability, pkg]) => {
  const status = packageStatus(pkg);
  return {
    capability,
    expectedPackage: pkg,
    actualPackage: status === 'mapped' ? pkg : '',
    status,
    reason: status === 'mapped' ? 'Existing package mapped for race execution.' : 'Expected package path does not exist.',
    mandatory: true,
  };
});

const cacheDecision = {
  phase: 'P7-C',
  status: packageStatus('./internal/pkg/cache/...') === 'mapped' ? 'cache_required_implemented' : 'cache_required_missing',
  decision: 'cache_required',
  evidence: [
    'CACHE_* configuration is present in .env.example, .env.production.example and backend/internal/config/p7_config.go.',
    'P7-V audit marks cache TTL and entry bound as implemented and several cache capabilities as partial.',
    'backend/internal/pkg/cache now provides TTL, max entries, invalidation, negative cache and singleflight primitives.',
  ],
  package: 'backend/internal/pkg/cache',
  testPackage: 'backend/internal/pkg/cache/...',
};

const runtime = {
  datasetResume: reportStatus('docs/p7-c-dataset-resume-drill-report.json'),
  pagination: reportStatus('docs/p7-c-pagination-runtime-report.json'),
  queryPlan: reportStatus('docs/p7-c-query-plan-report.json'),
  nPlusOne: reportStatus('docs/p7-c-nplusone-runtime-report.json'),
  race: reportStatus('docs/p7-c-race-test-report.json'),
};

const checks = [
  {
    id: 'mandatory-partial-zero',
    status: closureAudit.summary.mandatoryPartial === 0 ? 'passed' : 'failed',
    detail: `mandatoryPartial=${closureAudit.summary.mandatoryPartial}`,
  },
  {
    id: 'mandatory-missing-zero',
    status: closureAudit.summary.mandatoryMissing === 0 ? 'passed' : 'failed',
    detail: `mandatoryMissing=${closureAudit.summary.mandatoryMissing}`,
  },
  {
    id: 'dataset-resume-report-passed',
    status: runtime.datasetResume === 'passed' ? 'passed' : 'failed',
    detail: `dataset resume status=${runtime.datasetResume}`,
  },
  {
    id: 'pagination-runtime-passed',
    status: runtime.pagination === 'passed' ? 'passed' : 'failed',
    detail: `pagination status=${runtime.pagination}`,
  },
  {
    id: 'query-plan-passed',
    status: runtime.queryPlan === 'passed' ? 'passed' : 'failed',
    detail: `query plan status=${runtime.queryPlan}`,
  },
  {
    id: 'nplusone-passed',
    status: runtime.nPlusOne === 'passed' ? 'passed' : 'failed',
    detail: `n+1 status=${runtime.nPlusOne}`,
  },
  {
    id: 'cache-package-mapped',
    status: cacheDecision.status === 'cache_required_implemented' ? 'passed' : 'failed',
    detail: cacheDecision.package,
  },
  {
    id: 'race-package-mapping-complete',
    status: racePackages.every((item) => item.status === 'mapped') ? 'passed' : 'failed',
    detail: `${racePackages.filter((item) => item.status === 'mapped').length}/${racePackages.length} mapped`,
  },
  {
    id: 'linux-race-passed',
    status: runtime.race === 'passed' ? 'passed' : 'failed',
    detail: `race status=${runtime.race}`,
  },
];

const failed = checks.filter((item) => item.status !== 'passed');
const closureReport = {
  phase: 'P7-C',
  status: failed.length === 0 ? 'passed_ready_for_p7_v2' : 'incomplete',
  generatedAt: new Date().toISOString(),
  failed: failed.length,
  passed: checks.length - failed.length,
  checks,
  capabilityAudit: closureAudit.summary,
  runtime,
  cache: cacheDecision,
  raceMapping: {
    mandatoryCapabilitiesMapped: racePackages.every((item) => item.status === 'mapped'),
    packages: racePackages,
  },
  loadSoakFinalVerification: 'pending_p7_v2',
  realProductionPerformanceVerification: 'deferred',
  realProductionCapacityVerification: 'deferred',
  realProductionPeakLoadVerification: 'deferred',
  douyinRealCredentialVerification: 'deferred',
  tag: 'deferred',
  productionReady: false,
};

writeJSON('docs/p7-c-capability-closure-audit.json', closureAudit);
writeJSON('docs/p7-c-race-package-mapping.json', { phase: 'P7-C', status: racePackages.every((item) => item.status === 'mapped') ? 'mapped' : 'incomplete', packages: racePackages });
writeJSON('docs/p7-c-capability-closure-report.json', closureReport);

writeText(
  'docs/P7_C_CAPABILITY_CLOSURE_AUDIT.md',
  `# P7-C Capability Closure Audit\n\nStatus: ${closureAudit.status}\n\nSource: \`docs/p7-v-capability-completeness-audit.json\`\n\n- Mandatory implemented: ${closureAudit.summary.mandatoryImplemented}\n- Mandatory partial: ${closureAudit.summary.mandatoryPartial}\n- Mandatory missing: ${closureAudit.summary.mandatoryMissing}\n- Not applicable: ${closureAudit.summary.notApplicable}\n\n## Partial Items\n\n${sourcePartials.map((item) => `- \`${item.id}\`: ${item.capability} — ${item.gap}`).join('\n')}\n`,
);

writeText(
  'docs/P7_C_CACHE_DECISION.md',
  `# P7-C Cache Decision\n\nStatus: ${cacheDecision.status}\n\nDecision: \`${cacheDecision.decision}\`\n\nEvidence:\n\n${cacheDecision.evidence.map((item) => `- ${item}`).join('\n')}\n\nPackage: \`${cacheDecision.package}\`\n\nTest package: \`${cacheDecision.testPackage}\`\n`,
);

writeText(
  'docs/P7_C_RACE_PACKAGE_MAPPING.md',
  `# P7-C Race Package Mapping\n\nStatus: ${closureReport.raceMapping.mandatoryCapabilitiesMapped ? 'mapped' : 'incomplete'}\n\n${racePackages.map((item) => `- ${item.capability}: ${item.status} (${item.actualPackage || item.expectedPackage})`).join('\n')}\n`,
);

writeText(
  'docs/P7_C_CAPABILITY_CLOSURE_REPORT.md',
  `# P7-C Capability Closure Report\n\nStatus: ${closureReport.status}\n\nPassed: ${closureReport.passed}\nFailed: ${closureReport.failed}\n\n## Checks\n\n${checks.map((item) => `- ${item.status}: ${item.id} — ${item.detail}`).join('\n')}\n\nLoad/soak final verification remains pending for P7-V2. Production performance verification is deferred. Production Ready remains false.\n`,
);

console.log(JSON.stringify(closureReport, null, 2));
process.exit(failed.length === 0 ? 0 : 1);
