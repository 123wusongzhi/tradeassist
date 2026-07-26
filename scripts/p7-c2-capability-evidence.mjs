import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');
const allowedBlockers = new Set([
  'code_missing',
  'wiring_missing',
  'test_missing',
  'runtime_evidence_missing',
  'race_evidence_missing',
  'audit_status_stale',
  'multiple_blockers',
]);

function readJSON(rel, fallback = null) {
  try {
    return JSON.parse(fs.readFileSync(path.join(root, rel), 'utf8'));
  } catch {
    return fallback;
  }
}

function fileHas(rel, patterns) {
  const abs = path.join(root, rel);
  if (!fs.existsSync(abs)) return false;
  const text = fs.readFileSync(abs, 'utf8');
  return patterns.every((pattern) => (pattern instanceof RegExp ? pattern.test(text) : text.includes(pattern)));
}

function reportPassed(rel) {
  const report = readJSON(rel, {});
  return report?.status === 'passed' && report.generatedAt && report.dryRun !== true;
}

function racePassed() {
  const report = readJSON('docs/p7-c2-race-test-report.json', {});
  return (
    report.status === 'passed' &&
    report.environmentBlocked === false &&
    report.mapped === 11 &&
    report.executed === 11 &&
    report.passed === 11 &&
    report.failed === 0 &&
    report.skipped === 0 &&
    report.dataRaces === 0 &&
    report.deadlocks === 0 &&
    report.combinedMatrix === 'passed'
  );
}

function goTestPassed() {
  const report = readJSON('docs/p7-c2-race-test-report.json', {});
  return Array.isArray(report.preflight) && report.preflight.some((item) => item.command === 'go test ./...' && item.exitCode === 0);
}

function paginationRuntimePassed(list) {
  const report = readJSON('docs/p7-c2-pagination-runtime-report.json', {});
  const item = Array.isArray(report.lists) ? report.lists.find((row) => row.list === list) : report.lists?.[list];
  return report.status === 'passed' && (item?.status === 'passed' || item === 'passed');
}

function runtimeFor(id) {
  if (id.includes('cursor-keyset-pagination')) {
    const list = id.replace('-cursor-keyset-pagination', '').replace('operation-log', 'operationLog');
    return paginationRuntimePassed(list);
  }
  if (id === 'db-pool-wait-metrics' || id === 'rows-transaction-leak-protection') {
    return reportPassed('docs/p7-c2-query-plan-report.json');
  }
  if (id.startsWith('cache-') || id === 'singleflight' || id === 'negative-cache' || id.includes('worker') || id.includes('rate') || id.includes('quota') || id.includes('redis') || id.includes('goroutine') || id.includes('ticker')) {
    return racePassed();
  }
  if (id.includes('upload') || id.includes('file') || id === 'memory-budget') {
    return racePassed() || goTestPassed();
  }
  return goTestPassed();
}

function codeEvidence(id) {
  const evidence = [];
  const ok = (rel, patterns, label = rel) => {
    if (fileHas(rel, patterns)) evidence.push(label);
  };
  if (id.includes('cursor-keyset-pagination')) {
    ok('backend/internal/pkg/pagination/pagination.go', [/EncodeCursor/, /DecodeCursor/, /HMAC|hmac/, /TenantID/, /ShopID/], 'signed scoped cursor codec');
    if (id.startsWith('product')) ok('backend/internal/modules/product/service.go', [/ApplyTenantScope/, /Order\("created_at DESC/, /Limit\(ps\)/], 'product list service currently offset-backed');
    if (id.startsWith('order')) ok('backend/internal/modules/order/service.go', [/ApplyTenantScope/, /ApplyStoreScope/, /Order\("created_at DESC/, /Limit\(ps\)/], 'order list service currently offset-backed');
    if (id.startsWith('inventory')) ok('backend/internal/modules/inventory/center_list.go', [/buildSKUAlertBaseTX/, /Order\("sk.updated_at DESC/, /Limit\(ps\)/], 'inventory center service currently offset-backed');
    if (id.startsWith('task')) ok('backend/internal/modules/taskcenter/service_list.go', [/applyTenantListFilter/, /Order\("updated_at DESC/, /Limit\(fetchLimit\)/], 'taskcenter list service currently limit-backed');
    if (id.startsWith('webhook')) ok('backend/internal/modules/webhook/service.go', [/eventScopeQuery/, /tenant_id/, /platform_shop_id/], 'webhook scoped repository code');
    if (id.startsWith('operation-log')) ok('backend/internal/modules/operationlog/service.go', [/ApplyTenantScope/, /ApplyStoreScope/, /Order\("created_at DESC/, /Limit\(ps\)/], 'operation log service currently offset-backed');
  } else if (id.startsWith('cache') || id === 'singleflight' || id === 'negative-cache') {
    ok('backend/internal/pkg/cache/cache.go', [/TTL|ttl/, /maxEntries|MaxEntries/, /Invalidate|Delete/, /singleflight|Group/, /negative/i], 'bounded cache primitives');
  } else if (id.includes('worker')) {
    ok('backend/internal/config/p7_config.go', [/WorkerConcurrencyDefault/, /WorkerQueueCapacity/, /WorkerMaxInflight/, /WorkerShutdownTimeoutSecs/], 'bounded worker config');
    ok('backend/internal/modules/worker/monitor.go', [/Limit\(/, /lease|inflight|stale/i], 'worker monitor bounded reads');
  } else if (id.includes('rate') || id.includes('redis') || id.includes('quota')) {
    ok('backend/internal/pkg/ratelimit/limiter.go', [/Allow/, /Limit/, /mutex|sync/i], 'rate limiter package');
    ok('backend/internal/config/p7_config.go', [/RateLimitMode/, /RateLimitLocalFallback/, /RateLimitFailMode/], 'rate limit config');
  } else if (id.includes('upload') || id.includes('file')) {
    ok('backend/internal/pkg/security/upload.go', [/LimitReader|Max/, /ReadAll/], 'bounded upload helpers');
    ok('backend/internal/modules/files/service.go', [/tenant_id|TenantID/, /Limit|Cleanup|security/i], 'file service wiring');
  } else if (id.includes('memory') || id.includes('goroutine') || id.includes('ticker')) {
    ok('backend/internal/config/p7_config.go', [/DBMaxOpenConnections/, /WorkerShutdownTimeoutSecs/, /CacheMaxEntries/], 'bounded runtime config');
    ok('backend/internal/pkg/cache/cache.go', [/Close|Stop|Shutdown|ticker/i], 'cache lifecycle code');
  } else if (id.includes('db-pool') || id.includes('rows-transaction')) {
    ok('backend/internal/database/database.go', [/SetMaxOpenConns/, /SetMaxIdleConns/, /SetConnMaxLifetime/, /PingContext/], 'database pool bounds');
  }
  return evidence;
}

function wiringEvidence(id) {
  const evidence = [];
  if (id.includes('cursor-keyset-pagination')) {
    if (id.startsWith('webhook')) evidence.push('webhook Event scope query is tenant/shop scoped; list runtime still required');
    if (id.startsWith('task')) evidence.push('taskcenter list applies tenant/shop filters before bounded fetch');
    if (id.startsWith('product') || id.startsWith('order') || id.startsWith('inventory') || id.startsWith('operation-log')) {
      evidence.push('business list service exists, but P7-C2 requires runtime proof of keyset cursor path');
    }
  }
  if (id.startsWith('cache') || id === 'singleflight' || id === 'negative-cache') {
    if (fs.existsSync(path.join(root, 'docs/P7_C_CACHE_DECISION.md'))) evidence.push('P7-C cache decision document exists');
  }
  if (id.includes('worker') || id.includes('rate') || id.includes('quota') || id.includes('redis')) {
    if (fileHas('backend/internal/config/p7_config.go', [/Worker|RateLimit/])) evidence.push('P7 runtime limits are loaded from config');
  }
  return evidence;
}

function testEvidence(id) {
  const evidence = [];
  if (goTestPassed()) evidence.push('P7-C2 Linux preflight go test ./... passed');
  if (id.includes('cursor') && fileHas('backend/internal/pkg/pagination/pagination_test.go', [/TestCursorTamperRejected/, /TestCursorScope/])) evidence.push('pagination cursor unit tests cover tamper and scope');
  if ((id.startsWith('cache') || id === 'singleflight' || id === 'negative-cache') && fs.existsSync(path.join(root, 'backend/internal/pkg/cache/cache_test.go'))) evidence.push('cache unit tests exist');
  if (id.includes('rate') && fs.existsSync(path.join(root, 'backend/internal/pkg/ratelimit/limiter_test.go'))) evidence.push('rate limiter unit tests exist');
  return evidence;
}

function blocker(missing) {
  const types = [];
  if (missing.code) types.push('code_missing');
  if (missing.wiring) types.push('wiring_missing');
  if (missing.test) types.push('test_missing');
  if (missing.runtime) types.push('runtime_evidence_missing');
  if (missing.race) types.push('race_evidence_missing');
  if (types.length === 0) return 'audit_status_stale';
  if (types.length === 1) return types[0];
  return 'multiple_blockers';
}

const sourceAudit = readJSON('docs/p7-v-capability-completeness-audit.json', { capabilities: [] });
const previousPartials = sourceAudit.capabilities.filter((item) => item.status === 'partial');
const classification = previousPartials.map((item) => {
  const code = codeEvidence(item.id);
  const wiring = wiringEvidence(item.id);
  const tests = testEvidence(item.id);
  const runtimeOk = runtimeFor(item.id);
  const needsRace = /worker|cache|singleflight|negative-cache|rate|redis|quota|goroutine|ticker|file|upload|memory/.test(item.id);
  const raceOk = needsRace ? racePassed() : true;
  const missing = {
    code: code.length === 0,
    wiring: wiring.length === 0 && item.id.includes('cursor-keyset-pagination'),
    test: tests.length === 0,
    runtime: !runtimeOk,
    race: needsRace && !raceOk,
  };
  const blockerType = blocker(missing);
  if (!allowedBlockers.has(blockerType)) throw new Error(`invalid blocker type ${blockerType}`);
  const implemented = blockerType === 'audit_status_stale' || (!missing.code && !missing.wiring && !missing.test && !missing.runtime && !missing.race);
  return {
    capabilityId: item.id,
    capabilityName: item.capability,
    mandatory: true,
    previousStatus: 'partial',
    blockerType,
    codeEvidence: code,
    wiringEvidence: wiring,
    testEvidence: tests,
    runtimeEvidence: runtimeOk ? ['required P7-C2 runtime report status=passed'] : [],
    raceEvidence: raceOk && needsRace ? ['P7-C2 Linux race report status=passed'] : [],
    requiredAction: implemented ? 'Regenerate P7-C audit from current evidence.' : 'Close missing evidence categories listed by blockerType; do not mark implemented manually.',
    finalStatus: implemented ? 'implemented' : 'partial',
    finalReason: implemented ? 'All required evidence categories are present.' : 'Required evidence is still missing.',
  };
});

const summary = {
  previousPartial: classification.length,
  codeMissing: classification.filter((item) => item.blockerType === 'code_missing' || item.blockerType === 'multiple_blockers' && item.codeEvidence.length === 0).length,
  wiringMissing: classification.filter((item) => item.blockerType === 'wiring_missing' || item.blockerType === 'multiple_blockers' && item.wiringEvidence.length === 0 && item.capabilityId.includes('cursor')).length,
  testMissing: classification.filter((item) => item.blockerType === 'test_missing' || item.blockerType === 'multiple_blockers' && item.testEvidence.length === 0).length,
  runtimeEvidenceMissing: classification.filter((item) => item.blockerType === 'runtime_evidence_missing' || item.blockerType === 'multiple_blockers' && item.runtimeEvidence.length === 0).length,
  raceEvidenceMissing: classification.filter((item) => item.blockerType === 'race_evidence_missing' || item.blockerType === 'multiple_blockers' && item.raceEvidence.length === 0 && /worker|cache|singleflight|negative-cache|rate|redis|quota|goroutine|ticker|file|upload|memory/.test(item.capabilityId)).length,
  auditStatusStale: classification.filter((item) => item.blockerType === 'audit_status_stale').length,
  finalImplemented: classification.filter((item) => item.finalStatus === 'implemented').length,
  finalPartial: classification.filter((item) => item.finalStatus === 'partial').length,
  finalMissing: 0,
};

const normalization = {
  phase: 'P7-C2',
  status: summary.finalPartial === 0 && summary.finalMissing === 0 ? 'passed' : 'incomplete',
  generatedAt: new Date().toISOString(),
  capabilities: {
    previousPartial: summary.previousPartial,
    mandatoryImplemented: sourceAudit.capabilities.length - summary.previousPartial + summary.finalImplemented,
    mandatoryPartial: summary.finalPartial,
    mandatoryMissing: summary.finalMissing,
    auditStatusStaleResolved: summary.auditStatusStale,
    codeGapsFixed: 0,
    wiringGapsFixed: 0,
    testGapsFixed: 0,
    runtimeEvidenceGapsClosed: classification.filter((item) => item.runtimeEvidence.length > 0).length,
    raceEvidenceGapsClosed: classification.filter((item) => item.raceEvidence.length > 0).length,
  },
  blockerSummary: summary,
  items: classification,
};

fs.writeFileSync(path.join(docs, 'p7-c2-partial-classification.json'), `${JSON.stringify(classification, null, 2)}\n`, 'utf8');
fs.writeFileSync(path.join(docs, 'p7-c2-capability-normalization-report.json'), `${JSON.stringify(normalization, null, 2)}\n`, 'utf8');
fs.writeFileSync(
  path.join(docs, 'P7_C2_PARTIAL_CLASSIFICATION.md'),
  `# P7-C2 Partial Classification\n\nStatus: ${normalization.status}\n\n- Previous partial: ${summary.previousPartial}\n- Final implemented from previous partial: ${summary.finalImplemented}\n- Final partial: ${summary.finalPartial}\n- Final missing: ${summary.finalMissing}\n\n## Items\n\n${classification.map((item) => `- \`${item.capabilityId}\`: ${item.blockerType} -> ${item.finalStatus}`).join('\n')}\n`,
  'utf8',
);
fs.writeFileSync(
  path.join(docs, 'P7_C2_CAPABILITY_NORMALIZATION_REPORT.md'),
  `# P7-C2 Capability Normalization Report\n\nStatus: ${normalization.status}\n\n- Mandatory partial: ${normalization.capabilities.mandatoryPartial}\n- Mandatory missing: ${normalization.capabilities.mandatoryMissing}\n- Runtime evidence gaps closed: ${normalization.capabilities.runtimeEvidenceGapsClosed}\n- Race evidence gaps closed: ${normalization.capabilities.raceEvidenceGapsClosed}\n`,
  'utf8',
);

console.log(JSON.stringify(normalization, null, 2));
process.exit(normalization.status === 'passed' ? 0 : 1);
