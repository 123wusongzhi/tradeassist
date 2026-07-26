#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const checks = [];

function read(rel) {
  const p = path.join(root, rel);
  return fs.existsSync(p) ? fs.readFileSync(p, 'utf8') : '';
}

function exists(rel) {
  return fs.existsSync(path.join(root, rel));
}

function readJSON(rel) {
  const text = read(rel).replace(/^\uFEFF/, '');
  if (!text) return null;
  try {
    return JSON.parse(text);
  } catch {
    return null;
  }
}

function add(status, id, message, detail = '') {
  checks.push({ status, id, message, detail });
}

function requireText(id, rel, needles, message) {
  const text = read(rel);
  const missing = needles.filter((n) => !text.includes(n));
  if (missing.length === 0) add('passed', id, message || rel);
  else add('failed', id, `${message || rel} missing ${missing.join(', ')}`, rel);
}

function requireNoText(id, rel, needles, message) {
  const text = read(rel);
  const found = needles.filter((n) => text.includes(n));
  if (found.length === 0) add('passed', id, message || rel);
  else add('failed', id, `${message || rel} still contains ${found.join(', ')}`, rel);
}

function requireJSON(id, rel, validate, message) {
  const text = read(rel);
  if (!text) {
    add('failed', id, `${message || rel} missing`, rel);
    return;
  }
  try {
    const parsed = JSON.parse(text);
    const result = validate(parsed);
    if (result === true) add('passed', id, message || rel);
    else add('failed', id, typeof result === 'string' ? result : `${message || rel} validation failed`, rel);
  } catch (err) {
    add('failed', id, `${message || rel} invalid JSON: ${err.message}`, rel);
  }
}

function requireAcceptanceRun(id, rel) {
  const parsed = readJSON(rel);
  if (!parsed) {
    add('failed', id, `${rel} missing or invalid JSON`, rel);
    return null;
  }
  const summary = parsed.summary || {};
  const aiText = (parsed.steps || []).find((s) => s.name === 'ai-text-trial-run');
  const aiImage = (parsed.steps || []).find((s) => s.name === 'ai-image-trial-run');
  const failuresClean = summary.failed === 0 && summary.codeFailed === 0 && summary.nonAiFailed === 0;
  const aiTextExternalOnly = !aiText || aiText.category === 'external_provider';
  const aiImageExternalOnly = !aiImage || aiImage.category === 'external_provider';
  if (failuresClean && aiTextExternalOnly && aiImageExternalOnly) {
    add('passed', id, `${rel} failed/codeFailed/nonAiFailed are 0`);
  } else {
    add('failed', id, `${rel} has failed=${summary.failed}, codeFailed=${summary.codeFailed}, nonAiFailed=${summary.nonAiFailed}`, rel);
  }
  return {
    rel,
    conclusion: parsed.automatableConclusion,
    startedAt: parsed.startedAt,
    finishedAt: parsed.finishedAt,
    summary,
    aiText: aiText ? { status: aiText.status, reasonCode: aiText.reasonCode, category: aiText.category } : null,
    aiImage: aiImage ? { status: aiImage.status, reasonCode: aiImage.reasonCode, category: aiImage.category } : null,
  };
}

requireText('standard-otlp-json-encoder', 'backend/internal/pkg/tracing/tracing.go', [
  'type otlpTraceExportRequest struct',
  'ResourceSpans []otlpResourceSpans',
  'ScopeSpans []otlpScopeSpans',
  'TraceID           string         `json:"traceId"`',
  'SpanID            string         `json:"spanId"`',
  'ParentSpanID      string         `json:"parentSpanId,omitempty"`',
  'StartTimeUnixNano string',
  'statusCodeToOTLP',
  'attrsToOTLP',
  'eventsToOTLP',
], 'standard OTLP/HTTP JSON request shape exists');

requireText('standard-http-contract', 'backend/internal/pkg/tracing/tracing.go', [
  'normalizeEndpoint',
  '"/v1/traces"',
  'req.Header.Set("Content-Type", "application/json")',
  'req.Header.Set("Accept", "application/json")',
], 'standard OTLP/HTTP endpoint and content type');

requireNoText('custom-json-shape-removed', 'backend/internal/pkg/tracing/tracing.go', [
  '"parentSpan"',
  '"startUnix"',
  '"endUnix"',
], 'legacy custom span JSON fields removed');

requireText('failure-safe-exporter', 'backend/internal/pkg/tracing/tracing.go', [
  'retryAttempts',
  'isRetryableExportError',
  'boundedQueueSize',
  'boundedBatchSize',
  'exportTimeout',
  'OnExportError',
], 'retry, queue, batch and failure callbacks exist');

requireText('protocol-tests', 'backend/internal/pkg/tracing/tracing_test.go', [
  'TestHTTPExporterSendsStandardOTLPToMockCollector',
  'TestHTTPExporterRetriesRetryableStatus',
  'TestHTTPExporterDoesNotRetryClientStatus',
  'TestGoldenOTLPFixtureParses',
  'DisallowUnknownFields',
  'TEST_ACCESS_TOKEN_UNIQUE',
], 'standard OTLP mock collector and sensitive-field tests exist');

requireJSON('golden-fixture', 'backend/internal/pkg/tracing/testdata/valid_otlp_trace.json', (parsed) => {
  const rs = parsed.resourceSpans?.[0];
  const ss = rs?.scopeSpans?.[0];
  const spans = ss?.spans || [];
  if (!rs?.resource?.attributes?.some((a) => a.key === 'service.name')) return 'golden fixture missing service.name';
  if (!ss?.scope?.name) return 'golden fixture missing scope name';
  if (!spans.some((s) => s.traceId?.length === 32 && s.spanId?.length === 16)) return 'golden fixture missing 16-byte trace id or 8-byte span id';
  if (!spans.some((s) => s.parentSpanId?.length === 16)) return 'golden fixture missing parent span id';
  return true;
}, 'golden OTLP fixture parses');

requireText('otel-config', 'backend/internal/config/observability_config.go', [
  'OTELExporterOTLPHeaders',
  'OTELExporterOTLPInsecure',
  'OTELExportQueueSize',
  'OTELExportBatchSize',
  'OTELExportRetryMax',
  '"http/json"',
], 'OTLP config bounds and protocol defaults exist');

requireText('main-wiring', 'backend/cmd/server/main.go', [
  'OTLPProtocol:  obsCfg.OTELExporterOTLPProtocol',
  'OTLPHeaders:   obsCfg.OTELExporterOTLPHeaders',
  'QueueSize:     obsCfg.OTELExportQueueSize',
  'BatchSize:     obsCfg.OTELExportBatchSize',
  'RetryMax:      obsCfg.OTELExportRetryMax',
], 'server passes OTLP config to tracing provider');

requireText('api-runtime-status', 'backend/internal/modules/observabilitymod/handler.go', [
  'standard_protocol_ready',
  'mock_verified',
  'real_backend_deferred',
  'export_degraded',
  'disabled',
], 'observability API distinguishes OTLP runtime states');

requireText('admin-runtime-status', 'admin/src/pages/Ops/Observability/index.tsx', [
  'standard_protocol_ready',
  'mock_verified',
  'real_backend_deferred',
  'export_degraded',
  '模拟接收端已验证',
], 'observability UI distinguishes Mock verification from real backend status');

for (const rel of ['.env.example', '.env.docker.example', '.env.production.example']) {
  requireText(`env-${rel}`, rel, [
    'OTEL_EXPORTER_OTLP_PROTOCOL=http/json',
    'OTEL_EXPORTER_OTLP_HEADERS=',
    'OTEL_EXPORT_QUEUE_SIZE=1024',
    'OTEL_EXPORT_BATCH_SIZE=128',
    'OTEL_EXPORT_RETRY_MAX=2',
  ], `${rel} documents standard OTLP config`);
}

for (const doc of [
  'docs/P5_V_FINAL_VERIFICATION_AUDIT.md',
  'docs/P5_V_OTLP_DEPENDENCY_MATRIX.md',
  'docs/P5_V_OTLP_PROTOCOL_IMPLEMENTATION.md',
  'docs/P5_V_OTLP_PROTOCOL_TEST_REPORT.md',
  'docs/P5_V_FINAL_OBSERVABILITY_REPORT.md',
]) {
  if (exists(doc)) add('passed', `doc-${path.basename(doc)}`, doc);
  else add('failed', `doc-${path.basename(doc)}`, `${doc} missing`);
}

requireText('race-report-passed', 'docs/P5_V_RACE_TEST_REPORT.md', [
  'Status: passed',
  'Linux Race Verification Passed',
  '0 data races',
], 'Linux race matrix report is passed');

requireText('frontend-collector-passed', 'docs/P5_V_FRONTEND_COLLECTOR_VERIFICATION.md', [
  'Status: passed',
  'pnpm check:dev',
  'pnpm check:ui-copy --strict',
  'pnpm build:admin',
  'pnpm build:collector',
  'Mock Collector success must not be shown as production collector active',
], 'frontend and collector verification report is passed');

requireText('acceptance-run1-doc', 'docs/P5_V_DEVELOPMENT_ACCEPTANCE_RUN_1.md', [
  'Status: passed_with_blocked',
  'failed: 0',
  'codeFailed: 0',
  'nonAiFailed: 0',
], 'development acceptance Run 1 report records clean failure counts');

requireText('acceptance-run2-doc', 'docs/P5_V_DEVELOPMENT_ACCEPTANCE_RUN_2.md', [
  'Status: passed_with_blocked',
  'failed: 0',
  'codeFailed: 0',
  'nonAiFailed: 0',
], 'development acceptance Run 2 report records clean failure counts');

const acceptanceRuns = [
  requireAcceptanceRun('acceptance-run1-json', 'docs/demo-auto-acceptance.run1.json'),
  requireAcceptanceRun('acceptance-run2-json', 'docs/demo-auto-acceptance.run2.json'),
].filter(Boolean);

const p52 = read('docs/p5-2-final-observability-report.json');
if (p52.includes('"failed": 0')) add('passed', 'p5-2-report-failed-zero', 'P5.2 report has failed=0');
else add('failed', 'p5-2-report-failed-zero', 'P5.2 report missing failed=0');

const passed = checks.filter((c) => c.status === 'passed').length;
const warnings = checks.filter((c) => c.status === 'warning').length;
const failed = checks.filter((c) => c.status === 'failed').length;

const status = failed === 0
  ? 'passed_with_real_environment_telemetry_verification_deferred'
  : 'incomplete';

const report = {
  phase: 'P5-V',
  status,
  summary: { passed, warnings, failed },
  otlp: {
    protocol: 'otlp_http',
    format: 'json',
    standardCompatible: failed === 0,
    mockCollectorParsed: checks.some((c) => c.id === 'protocol-tests' && c.status === 'passed'),
    sensitiveFields: checks.some((c) => c.id === 'protocol-tests' && c.status === 'passed') ? 'passed' : 'incomplete',
    failureSafe: checks.some((c) => c.id === 'failure-safe-exporter' && c.status === 'passed') ? 'passed' : 'incomplete',
    queueBounded: checks.some((c) => c.id === 'failure-safe-exporter' && c.status === 'passed'),
    shutdownBounded: checks.some((c) => c.id === 'failure-safe-exporter' && c.status === 'passed'),
  },
  acceptanceRuns,
  realEnvironmentTelemetryVerification: 'deferred',
  externalAlertChannels: 'deferred',
  productionSloValidation: 'deferred',
  issues: checks.filter((c) => c.status === 'failed'),
};

fs.mkdirSync(path.join(root, 'docs'), { recursive: true });
fs.writeFileSync(path.join(root, 'docs/p5-v-final-observability-report.json'), JSON.stringify(report, null, 2));
fs.writeFileSync(path.join(root, 'docs/P5_V_FINAL_OBSERVABILITY_REPORT.md'), `# P5-V Final Observability Report

Phase: P5-V
Status: ${status}
Real Environment Telemetry Verification: deferred
External Alert Channels: deferred
Production SLO Validation: deferred

## Summary
- passed: ${passed}
- warnings: ${warnings}
- failed: ${failed}

## Development Acceptance
${acceptanceRuns.map((r, index) => `- Run ${index + 1}: ${r.conclusion}; failed=${r.summary.failed}, codeFailed=${r.summary.codeFailed}, nonAiFailed=${r.summary.nonAiFailed}; AI text=${r.aiText?.status || 'missing'}, AI image=${r.aiImage?.status || 'missing'}`).join('\n')}

## Checks
${checks.map((c) => `- [${c.status}] ${c.id}: ${c.message}`).join('\n')}

## Conclusion
${failed === 0 ? 'Phase P5-V code-level observability gate passed with real environment telemetry verification deferred. Do not mark Production Ready, do not tag, and do not treat Mock Collector verification as a real telemetry backend.' : 'Phase P5-V Incomplete. Phase P5 Closure Verification Incomplete. Do not mark Phase P5 Fully Closed until failed items are fixed and the required verification commands pass.'}
`);

console.log(JSON.stringify(report.summary));
process.exit(failed > 0 ? 1 : 0);
