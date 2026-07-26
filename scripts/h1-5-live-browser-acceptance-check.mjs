/**
 * Phase H1.5.1 — validates live browser acceptance artifacts (screenshots, reports, doc sync).
 * Does NOT fabricate manual browser results; reads outputs from h1-5-live-browser-acceptance.mjs.
 */
import fs from 'node:fs';
import path from 'node:path';

const MIN_PNG_BYTES = 5000;
const FORBIDDEN = [
  'accessToken', 'refreshToken', 'appSecret', 'apiKey', 'buyerName', 'customerPhone',
  'customerEmail', 'prompt', 'platformRaw', 'providerRaw',
];

const REQUIRED_SCREENSHOTS = {
  '1366x768': [
    '01-dashboard.png', '02-product-drafts.png', '03-publish-batches.png', '04-collect-tasks.png',
    '05-orders.png', '06-order-sync-tasks.png', '07-inventory-sync-tasks.png',
    '08-customer-conversations.png', '09-ai-image-batches.png', '10-task-center.png', '11-config-status.png',
  ],
  '1024x768': [
    '01-dashboard.png', '02-product-drafts.png', '03-publish-batches.png', '04-orders.png',
    '05-inventory-sync-tasks.png', '06-customer-conversations.png', '07-ai-image-batches.png', '08-task-center.png',
  ],
};

function readJson(file) {
  if (!fs.existsSync(file)) return null;
  return JSON.parse(fs.readFileSync(file, 'utf8'));
}

function scanPngDir(dir, expected) {
  const issues = [];
  const found = [];
  for (const name of expected) {
    const full = path.join(dir, name);
    if (!fs.existsSync(full)) {
      issues.push({ level: 'P1', message: `Missing screenshot: ${full}` });
      continue;
    }
    const stat = fs.statSync(full);
    if (stat.size < MIN_PNG_BYTES) {
      issues.push({ level: 'P1', message: `Screenshot too small (${stat.size} B): ${full}` });
    }
    found.push({ file: name, bytes: stat.size });
  }
  return { issues, found };
}

function scanTextForSecrets(text, label) {
  const hits = [];
  for (const token of FORBIDDEN) {
    if (text.toLowerCase().includes(token.toLowerCase())) {
      hits.push(`${label}:contains:${token}`);
    }
  }
  return hits;
}

function checkDocStatusSync() {
  const issues = [];
  const files = ['README.md', 'README.en.md', 'docs/PROGRESS.md', 'CHANGELOG.md'];
  const needles = ['Phase H1.5.1', 'H1.5.1', 'MVP Demo Ready', 'Tag deferred', '非 Production Ready'];
  for (const f of files) {
    if (!fs.existsSync(f)) {
      issues.push({ level: 'P1', message: `Missing doc: ${f}` });
      continue;
    }
    const text = fs.readFileSync(f, 'utf8');
    if (!text.includes('MVP Demo Ready')) {
      issues.push({ level: 'P2', message: `${f} missing MVP Demo Ready` });
    }
    const prIdx = text.indexOf('Production Ready');
    if (prIdx >= 0) {
      const hasNegation =
        text.includes('非 Production Ready') ||
        /not Production Ready/i.test(text) ||
        /remain blocked/i.test(text) ||
        /blocked by/i.test(text);
      if (!hasNegation) {
        issues.push({ level: 'P1', message: `${f} may mark Production Ready` });
      }
    }
    if (f === 'docs/PROGRESS.md' && !text.includes('H1.5.1')) {
      issues.push({ level: 'P2', message: 'PROGRESS.md missing H1.5.1 entry' });
    }
  }
  return issues;
}

function main() {
  const issues = [];
  let passed = 0;
  let warning = 0;
  let failed = 0;

  const liveJson = readJson('docs/h1-5-live-browser-acceptance.json');
  const aiJson = readJson('docs/h1-5-ai-image-baseline-confirmation.json');
  const requiredDocs = [
    'docs/H1_5_LIVE_BROWSER_ACCEPTANCE.md',
    'docs/H1_5_AI_IMAGE_BASELINE_CONFIRMATION.md',
    'docs/h1-5-live-browser-acceptance.json',
    'docs/h1-5-ai-image-baseline-confirmation.json',
  ];

  for (const d of requiredDocs) {
    if (!fs.existsSync(d)) {
      failed += 1;
      issues.push({ id: 'H151-doc', level: 'P1', message: `Missing required doc: ${d}`, status: 'failed' });
    } else {
      passed += 1;
    }
  }

  if (!liveJson?.phase || liveJson.phase !== 'H1.5.1') {
    failed += 1;
    issues.push({ id: 'H151-live-json', level: 'P1', message: 'Invalid live browser JSON phase', status: 'failed' });
  } else {
    passed += 1;
    if (!liveJson.environment?.adminUrl || !liveJson.environment?.chromeVersion) {
      warning += 1;
      issues.push({ id: 'H151-env', level: 'P2', message: 'Live JSON missing environment details', status: 'warning' });
    }
    if (liveJson.status === 'failed') {
      failed += 1;
      issues.push({ id: 'H151-live-status', level: 'P1', message: 'Live browser status failed', status: 'failed' });
    } else {
      passed += 1;
    }
  }

  if (!aiJson?.baseline) {
    failed += 1;
    issues.push({ id: 'H151-ai-json', level: 'P1', message: 'Missing AI image baseline JSON', status: 'failed' });
  } else {
    passed += 1;
  }

  for (const [res, names] of Object.entries(REQUIRED_SCREENSHOTS)) {
    const dir = path.join('docs', 'screenshots', 'h1-5', res);
    const { issues: shotIssues, found } = scanPngDir(dir, names);
    if (shotIssues.length) {
      const allMissing = shotIssues.every((i) => i.message.startsWith('Missing screenshot:'));
      if (allMissing && found.length === 0) {
        warning += 1;
        issues.push({
          id: `H151-shot-${res}`,
          level: 'P2',
          message: `Screenshots not present locally (optional; run scripts/h1-5-live-browser-acceptance.mjs)`,
          status: 'warning',
        });
      } else {
        failed += shotIssues.length;
        issues.push(...shotIssues.map((i) => ({ ...i, id: `H151-shot-${res}`, status: 'failed' })));
      }
    } else {
      passed += found.length;
    }
  }

  const secretHits = [];
  for (const res of ['1366x768', '1024x768']) {
    const dir = path.join('docs', 'screenshots', 'h1-5', res);
    if (!fs.existsSync(dir)) continue;
    for (const f of fs.readdirSync(dir)) {
      if (!f.endsWith('.png')) continue;
      secretHits.push(...scanTextForSecrets(f, f));
    }
  }
  if (secretHits.length) {
    failed += 1;
    issues.push({ id: 'H151-secrets', level: 'P1', message: `Forbidden tokens in screenshot names: ${secretHits.join(', ')}`, status: 'failed' });
  } else {
    passed += 1;
  }

  const docIssues = checkDocStatusSync();
  for (const i of docIssues) {
    if (i.level === 'P1') failed += 1;
    else warning += 1;
    issues.push({ ...i, id: 'H151-doc-sync', status: i.level === 'P1' ? 'failed' : 'warning' });
  }
  if (!docIssues.length) passed += 1;

  const overall = failed > 0 ? 'failed' : warning > 0 ? 'passed_with_warning' : 'passed';
  const report = {
    phase: 'H1.5.1',
    status: overall,
    checkedAt: new Date().toISOString(),
    summary: { passed, warning, failed, total: passed + warning + failed },
    liveBrowserStatus: liveJson?.status ?? 'missing',
    aiImageBaseline: aiJson?.baseline?.result ?? 'missing',
    issues,
    finalConclusion:
      overall === 'passed'
        ? 'H1.5.1 live browser acceptance artifacts validated.'
        : 'H1.5.1 check completed with issues — see issues array.',
  };

  fs.writeFileSync('docs/h1-5-live-browser-acceptance-check.json', `${JSON.stringify(report, null, 2)}\n`);
  console.log(`H1.5.1 live browser check — ${overall} (failed=${failed}, warning=${warning})`);
  process.exit(failed > 0 ? 1 : 0);
}

main();
