import {
  assertLoadHostSafe,
  DB_PREFIX,
  resolveP7V2PortConfig,
  runWSL,
  writeJSON,
  writeMarkdown,
} from './p7-v2-lib.mjs';
import { classifyP7V2Database, summarizeCleanupClassifications } from './p7-v2-r3b-cleanup-classifier.mjs';

const portConfig = resolveP7V2PortConfig();
const cases = [
  { id: 'localhost-allowed', url: `http://localhost:${portConfig.port}`, expectBlocked: false },
  { id: '127-allowed', url: portConfig.baseUrl, expectBlocked: false },
  { id: 'wsl-nonloopback-rejected', url: `http://172.22.144.1:${portConfig.port}`, expectBlocked: true },
  { id: 'production-domain-rejected', url: 'https://api.zhihengxiangyu.com', expectBlocked: true },
  { id: 'public-ip-rejected', url: `http://8.8.8.8:${portConfig.port}`, expectBlocked: true },
  { id: 'empty-host-rejected', url: '', expectBlocked: true },
];

const results = cases.map((item) => {
  const issues = assertLoadHostSafe(item.url, 'performance');
  const blocked = issues.length > 0;
  return {
    id: item.id,
    url: item.url || '<empty>',
    blocked,
    issues,
    status: blocked === item.expectBlocked ? 'passed' : 'failed',
  };
});

function splitRows(stdout) {
  return String(stdout || '').trim().split('\n').filter(Boolean);
}

function psql(sql) {
  const oneLineSql = String(sql).replace(/\s+/g, ' ').trim();
  return runWSL(`psql -h /var/run/postgresql -U root -d postgres -At -v ON_ERROR_STOP=1 -c ${JSON.stringify(oneLineSql)}`, {
    timeout: 30000,
    maxBuffer: 10 * 1024 * 1024,
  });
}

const listenerQuery = runWSL(
  `ss -ltnp 2>/dev/null | awk '$4 ~ /127\\.0\\.0\\.1:${portConfig.port}$/ {print}' || true`,
  { timeout: 15000 },
);
const listenerRows = splitRows(listenerQuery.stdout);
const listener18080Count = listenerRows.length;
const unknownPortOwner = listenerRows.some((line) => !line.includes('artifacts/p7-v2/server') && !line.includes('pid='));
const bindProbe = runWSL(
  `python3 - <<'PY'
import socket
s = socket.socket()
try:
    s.bind(('127.0.0.1', ${portConfig.port}))
    print('bind-ok')
finally:
    s.close()
PY`,
  { timeout: 15000 },
);
const bindProbePassed = (bindProbe.stdout || '').includes('bind-ok');
const probeReleased = bindProbePassed;

const dbQuery = psql(`
SELECT
  d.datname,
  pg_get_userbyid(d.datdba) AS owner,
  pg_database_size(d.datname) AS size_bytes,
  COALESCE(a.active_connections, 0) AS active_connections
FROM pg_database d
LEFT JOIN (
  SELECT datname, count(*) AS active_connections
  FROM pg_stat_activity
  WHERE datname LIKE '${DB_PREFIX}%'
  GROUP BY datname
) a ON a.datname = d.datname
WHERE d.datname LIKE '${DB_PREFIX}%'
ORDER BY d.datname;`);
const databases = dbQuery.status === 0
  ? splitRows(dbQuery.stdout).map((line) => {
      const [databaseName, databaseOwner, databaseSizeBytes, activeConnectionCount] = line.split('|');
      return classifyP7V2Database(databaseName, {
        databaseOwner,
        databaseSizeBytes: Number(databaseSizeBytes || 0),
        activeConnectionCount: Number(activeConnectionCount || 0),
      });
    })
  : [];
const classificationSummary = summarizeCleanupClassifications(databases);
const procQuery = runWSL("pgrep -af 'p7v2|p7load|p7verify|k6.*p7v2|artifacts/p7-v2/server' 2>/dev/null || true", { timeout: 15000 });
const processRows = splitRows(procQuery.stdout).filter((line) => !line.includes('pgrep -af'));
const unknownProcessCount = processRows.filter((line) => !line.includes('artifacts/p7-v2/server') && !line.includes('p7-v2')).length;

const failed = results.filter((r) => r.status !== 'passed').length;
const resourceFailed = [
  listener18080Count === 0,
  !unknownPortOwner,
  bindProbePassed,
  probeReleased,
  dbQuery.status === 0,
  classificationSummary.currentFormalResidualCount === 0,
  classificationSummary.unknownDatabaseCount === 0,
  unknownProcessCount === 0,
].filter((ok) => !ok).length;
const report = {
  phase: 'P7-V2',
  component: 'load-host-guard',
  status: failed === 0 && resourceFailed === 0 ? 'passed' : 'failed',
  failed: failed + resourceFailed,
  passed: results.length - failed,
  cases: results,
  port18080Available: listener18080Count === 0,
  listener18080Count,
  unknownPortOwner,
  bindProbePassed,
  probeReleased,
  currentFormalResidualCount: classificationSummary.currentFormalResidualCount,
  unknownDatabaseCount: classificationSummary.unknownDatabaseCount,
  unknownProcessCount,
  knownRetainedDiagnosticDatabaseCount: classificationSummary.knownRetainedDiagnosticDatabaseCount,
  databaseQueryExecuted: dbQuery.status === 0,
  databases,
  listenerRows,
  processRows,
  generatedAt: new Date().toISOString(),
};

writeJSON('docs/p7-v2-load-host-guard-report.json', report);
writeMarkdown(
  'docs/P7_V2_LOAD_HOST_GUARD_REPORT.md',
  `# P7-V2 Load Host Guard Report

Status: ${report.status}

${results.map((r) => `- [${r.status}] ${r.id}: ${r.url}`).join('\n')}

| Field | Value |
| --- | --- |
| Port ${portConfig.port} available | ${report.port18080Available} |
| Listener ${portConfig.port} count | ${listener18080Count} |
| Unknown DBs | ${classificationSummary.unknownDatabaseCount} |
| Current formal residual DBs | ${classificationSummary.currentFormalResidualCount} |
| Unknown processes | ${unknownProcessCount} |
| Bind probe passed | ${bindProbePassed} |
`,
);

console.log(JSON.stringify({ phase: 'P7-V2', status: report.status, failed, report: 'docs/p7-v2-load-host-guard-report.json' }, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
