import { DB_PREFIX, gitCommit, readJSON, runWSL, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';
import { classifyP7V2Database, summarizeCleanupClassifications } from './p7-v2-r3b-cleanup-classifier.mjs';

function psql(sql) {
  return runWSL(`psql -h /var/run/postgresql -U root -d postgres -At -v ON_ERROR_STOP=1 -c ${JSON.stringify(sql)}`);
}

const query = psql(`SELECT datname FROM pg_database WHERE datname LIKE '${DB_PREFIX}%' ORDER BY datname;`);
const databases = query.status === 0 ? (query.stdout || '').trim().split('\n').filter(Boolean) : [];
const classifications = databases.map((databaseName) => classifyP7V2Database(databaseName));
const summary = summarizeCleanupClassifications(classifications);
const cleanupGateSemanticsValid =
  query.status === 0 &&
  summary.unknownDatabaseCount === 0;
const cleanupCodeChangeRequired = false;
const report = {
  phase: 'P7-V2-R3B-DATASET-EXECUTE-RECOVERY',
  component: 'cleanup-preflight',
  status: cleanupGateSemanticsValid ? 'passed' : 'blocked',
  checkedAt: new Date().toISOString(),
  gitCommit: gitCommit(),
  databasePrefix: DB_PREFIX,
  cleanupGateSemanticsValid,
  ...summary,
  cleanupCodeChangeRequired,
  queryExecuted: query.status === 0,
  databases: classifications,
  issues: [
    ...(query.status === 0 ? [] : ['database inventory query failed']),
    ...(summary.unknownDatabaseCount > 0 ? ['unknown P7-V2 databases require audit before formal execution'] : []),
  ],
};

writeJSON('docs/p7-v2-r3b-cleanup-preflight.json', report);
writeMarkdown('docs/P7_V2_R3B_CLEANUP_PREFLIGHT.md', `# P7-V2 R3B Cleanup Preflight

Status: ${report.status}

- Cleanup gate semantics valid: ${cleanupGateSemanticsValid}
- Current formal residuals: ${summary.currentFormalResidualCount}
- Failed attempt residuals: ${summary.failedAttemptResidualCount}
- Historical evidence databases: ${summary.historicalEvidenceDatabaseCount}
- Superseded/diagnostic databases: ${summary.supersededRunDatabaseCount}
- Unknown databases: ${summary.unknownDatabaseCount}
- Cleanup code change required: ${cleanupCodeChangeRequired}
`);
console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
