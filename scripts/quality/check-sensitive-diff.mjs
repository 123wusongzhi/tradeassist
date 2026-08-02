#!/usr/bin/env node
import { existsSync, lstatSync } from 'node:fs';
import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execa } from 'execa';
import pc from 'picocolors';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, '..', '..');
const args = process.argv.slice(2);
const baseArg = args.find((arg) => arg.startsWith('--base='));
const headArg = args.find((arg) => arg.startsWith('--head='));
const base = baseArg?.slice('--base='.length) || process.env.QUALITY_BASE_SHA || process.env.TEST_AFFECTED_BASE;
const head = headArg?.slice('--head='.length) || process.env.QUALITY_HEAD_SHA || 'HEAD';

const patterns = [
  { name: 'private-key', regex: /-----BEGIN (?:RSA |EC |OPENSSH |DSA |)?PRIVATE KEY-----/i },
  { name: 'github-token', regex: /gh[opsur]_[A-Za-z0-9_]{36,}/ },
  { name: 'github-fine-grained-token', regex: /\bgithub_pat_[A-Za-z0-9_]{50,}\b/ },
  { name: 'gitlab-personal-access-token', regex: /\bglpat-[A-Za-z0-9_-]{20,}\b/ },
  { name: 'slack-token', regex: /\bxox(?:b|p|a|r|s)-[A-Za-z0-9-]{20,}\b/ },
  { name: 'aws-access-key', regex: /\b(?:AKIA|ASIA)[0-9A-Z]{16}\b/ },
  { name: 'aws-secret-access-key', regex: /\bAWS_SECRET_ACCESS_KEY\s*[:=]\s*['\"]?[A-Za-z0-9/+=]{32,}['\"]?/i },
  { name: 'openai-style-secret', regex: /\bsk-(?:proj-)?[A-Za-z0-9_-]{32,}\b/ },
  { name: 'stripe-live-secret', regex: /\b(?:sk|rk)_live_[A-Za-z0-9]{20,}\b/ },
  { name: 'jwt', regex: /\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b/ },
  { name: 'database-url-with-password', regex: /\b(?:postgres|postgresql|mysql|redis):\/\/[^\s:@/]+:[^\s:@/]{8,}@[^\s]+/i },
  { name: 'hardcoded-bearer', regex: /\bAuthorization\s*[:=]\s*['"]?Bearer\s+[A-Za-z0-9._~+/=-]{20,}/i },
  { name: 'hardcoded-cookie', regex: /\bCookie\s*[:=]\s*['"][^'"]*(?:session|token|auth|sid)=[^'"]{12,}/i },
  { name: 'production-password-assignment', regex: /\b(?:PROD|PRODUCTION)[A-Z0-9_]*PASSWORD\s*=\s*[^\s#'"`]{8,}/i },
];

const placeholderValues = new Set([
  'example', 'fake', 'dummy', 'placeholder', 'changeme', 'change-me', 'test', 'mock', 'redacted', 'your-token', 'xxx', '123456',
]);

function isStructuredPlaceholder(match) {
  const value = String(match || '').trim();
  if (!value) return true;
  if (/^(?:<|\[)?(?:redacted|placeholder|your[-_ ]?(?:token|key|secret))(?:>|\])?$/i.test(value)) return true;
  const assignment = value.match(/(?:token|secret|password|api[_-]?key)\s*[:=]\s*['"]?([^'"\s,;]+)/i);
  const bearer = value.match(/\bBearer\s+([A-Za-z0-9._~+/=-]+)/i);
  const candidate = assignment?.[1] || bearer?.[1] || value;
  const normalized = candidate.toLowerCase();
  return placeholderValues.has(normalized) || /^(?:(?:collector|demo)[_-])?(?:wrong|test|fake|mock|example|dummy)[_-](?:token|key|secret)$/i.test(candidate);
}

function mask(value) {
  if (value.length <= 12) return '<redacted>';
  return `${value.slice(0, 4)}…${value.slice(-4)}`;
}

async function git(args) {
  const result = await execa('git', args, { cwd: root, reject: false });
  if (result.exitCode !== 0) throw new Error(result.stderr || `git ${args.join(' ')} failed`);
  return result.stdout;
}

async function diffTexts() {
  if (base) {
    const range = head && head !== 'HEAD' ? [base, head] : [base, '--'];
    return [{ source: base, text: await git(['diff', '--unified=0', ...range]) }];
  }

  const entries = [
    { source: 'working-tree', text: await git(['diff', '--unified=0', '--']) },
    { source: 'staged', text: await git(['diff', '--cached', '--unified=0', '--']) },
  ];

  const untracked = (await git(['ls-files', '--others', '--exclude-standard']))
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
    .filter((file) => !file.startsWith('node_modules/') && !file.startsWith('.tmp/') && !file.includes('coverage') && !file.includes('playwright-report'));

  for (const file of untracked) {
    const absolute = path.join(root, file);
    if (!existsSync(absolute)) continue;
    if (lstatSync(absolute).isSymbolicLink()) {
      throw new Error(`refusing to scan untracked symbolic link: ${file}`);
    }
    const statLikeBinary = /\.(png|jpg|jpeg|gif|webp|zip|gz|pdf|mp4|webm)$/i.test(file);
    if (statLikeBinary) continue;
    // An unreadable file must fail the gate rather than silently looking clean.
    const content = await readFile(absolute, 'utf8');
    const text = [`+++ b/${file}`, ...content.split('\n').map((line) => `+${line}`)].join('\n');
    entries.push({ source: `untracked:${file}`, text });
  }

  return entries;
}

function scanDiff(source, text) {
  const findings = [];
  let file = '<unknown>';
  let newLine = 0;

  for (const line of text.split('\n')) {
    const fileMatch = line.match(/^\+\+\+ b\/(.+)$/);
    if (fileMatch) {
      file = fileMatch[1];
      newLine = 0;
      continue;
    }
    const hunk = line.match(/^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@/);
    if (hunk) {
      newLine = Number(hunk[1]) - 1;
      continue;
    }
    if (line.startsWith('+') && !line.startsWith('+++')) {
      newLine += 1;
      const value = line.slice(1);
      for (const pattern of patterns) {
        const match = value.match(pattern.regex);
        if (match && !isStructuredPlaceholder(match[0])) findings.push({ source, file, line: newLine, pattern: pattern.name, sample: mask(match[0]) });
      }
    } else if (!line.startsWith('-')) {
      continue;
    }
  }
  return findings;
}

if (args.includes('--self-test')) {
  const fixtureSecrets = [
    `sk-proj_${'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789'}${'abcdefghijk'}`,
    `ghs_${'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789'}${'abcdefghijk'}`,
    `github_pat_${'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_abcdefghijklmno'}`,
    `glpat-${'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789'}${'abcdefghijk'}`,
    `xoxb-${'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789'}${'abcdefghijk'}`,
    `ASIA${'ABCDEFGHIJKLMNOP'}`,
    `rk_live_${'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789'}${'abcdefghijk'}`,
  ];
  const fixture = [
    '+++ b/test.txt',
    `@@ -0,0 +1,${fixtureSecrets.length} @@`,
    ...fixtureSecrets.map((secret) => `+// fake fixture: ${secret}`),
  ].join('\n');
  const findings = scanDiff('self-test', fixture);
  if (findings.length !== fixtureSecrets.length || findings.some((finding) => finding.sample.includes('ABCDEFGHIJKLMNOPQRSTUVWXYZ'))) {
    console.error('sensitive scanner self-test failed');
    process.exit(1);
  }
  console.log(JSON.stringify(findings[0]));
  process.exit(0);
}

const entries = await diffTexts();
const findings = entries.flatMap((entry) => scanDiff(entry.source, entry.text));

console.log(pc.cyan('Sensitive diff scan'));
console.log(`Diff sources: ${entries.map((entry) => entry.source).join(', ') || 'none'}`);
console.log(`Findings: ${findings.length}`);

if (findings.length) {
  for (const finding of findings) {
    console.error(`${finding.file}:${finding.line} ${finding.pattern} ${finding.sample}`);
  }
  process.exit(1);
}

console.log(pc.green('No high-confidence sensitive values found in changed lines.'));
