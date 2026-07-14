import {
  assertLoadHostSafe,
  root,
  writeJSON,
  writeMarkdown,
} from './p7-v2-lib.mjs';

const cases = [
  { id: 'localhost-allowed', url: 'http://localhost:8080', expectBlocked: false },
  { id: '127-allowed', url: 'http://127.0.0.1:8080', expectBlocked: false },
  { id: 'wsl-controlled-allowed', url: 'http://172.22.144.1:8080', expectBlocked: false },
  { id: 'production-domain-rejected', url: 'https://api.zhihengxiangyu.com', expectBlocked: true },
  { id: 'public-ip-rejected', url: 'http://8.8.8.8:8080', expectBlocked: true },
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

const failed = results.filter((r) => r.status !== 'passed').length;
const report = {
  phase: 'P7-V2',
  component: 'load-host-guard',
  status: failed === 0 ? 'passed' : 'failed',
  failed,
  passed: results.length - failed,
  cases: results,
  generatedAt: new Date().toISOString(),
};

writeJSON('docs/p7-v2-load-host-guard-report.json', report);
writeMarkdown(
  'docs/P7_V2_LOAD_HOST_GUARD_REPORT.md',
  `# P7-V2 Load Host Guard Report

Status: ${report.status}

${results.map((r) => `- [${r.status}] ${r.id}: ${r.url}`).join('\n')}
`,
);

console.log(JSON.stringify({ phase: 'P7-V2', status: report.status, failed, report: 'docs/p7-v2-load-host-guard-report.json' }, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
