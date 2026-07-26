import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';

export const FORMAL_BINARY_PROVENANCE_VERSION = 2;
export const BINARY_PROVENANCE_MANIFEST_PATH = 'docs/p7-v2-r3b-formal-binary-provenance-manifest.json';
export const BINARY_STORE_ROOT = 'artifacts/p7-v2/formal-binaries';

const SHA256_RE = /^[a-f0-9]{64}$/;
const COMMIT_RE = /^[a-f0-9]{40}$/;
const SECRET_RE = /(^|_)(SECRET|PASSWORD|TOKEN|COOKIE|JWT|DATABASE_URL|DB_PASS|PROVIDER_KEY)$/i;
const RUNTIME_SOURCE_ROOTS = ['backend', 'backend/migrations', 'backend/go.mod', 'backend/go.sum'];
const RUNTIME_CONFIG_ROOTS = ['.env.example', '.env.docker.example', 'backend/internal/config'];
const IGNORED_DIRS = new Set(['.git', 'node_modules', 'dist', 'artifacts', 'docs', 'data', 'logs', 'tmp']);

export function repoRoot() {
  return process.cwd();
}

export function sha256Bytes(value) {
  return crypto.createHash('sha256').update(value).digest('hex');
}

export function stableJson(value) {
  if (Array.isArray(value)) return `[${value.map((entry) => stableJson(entry)).join(',')}]`;
  if (value && typeof value === 'object') {
    return `{${Object.keys(value).sort().map((key) => `${JSON.stringify(key)}:${stableJson(value[key])}`).join(',')}}`;
  }
  return JSON.stringify(value);
}

export function sha256Json(value) {
  return sha256Bytes(stableJson(value));
}

export function readJSON(relOrAbs) {
  try {
    const full = path.isAbsolute(relOrAbs) ? relOrAbs : path.join(repoRoot(), relOrAbs);
    return JSON.parse(fs.readFileSync(full, 'utf8'));
  } catch {
    return null;
  }
}

export function writeJSON(relOrAbs, data) {
  const full = path.isAbsolute(relOrAbs) ? relOrAbs : path.join(repoRoot(), relOrAbs);
  fs.mkdirSync(path.dirname(full), { recursive: true });
  fs.writeFileSync(full, `${JSON.stringify(data, null, 2)}\n`, 'utf8');
}

function run(command, args, opts = {}) {
  const res = spawnSync(command, args, {
    cwd: opts.cwd || repoRoot(),
    env: { ...process.env, ...(opts.env || {}) },
    encoding: 'utf8',
    timeout: opts.timeout ?? 120000,
    maxBuffer: opts.maxBuffer ?? 30 * 1024 * 1024,
  });
  return {
    command: `${command} ${args.join(' ')}`,
    status: res.status ?? 1,
    stdout: res.stdout || '',
    stderr: res.stderr || '',
  };
}

function repoRel(absPath, base) {
  return path.relative(base, absPath).replaceAll('\\', '/');
}

function walk(abs, base, out, predicate = () => true) {
  if (!fs.existsSync(abs)) return;
  const stat = fs.statSync(abs);
  if (stat.isFile()) {
    const rel = repoRel(abs, base);
    if (predicate(rel)) out.push({ path: rel, sha256: sha256Bytes(fs.readFileSync(abs)), sizeBytes: stat.size });
    return;
  }
  if (!stat.isDirectory()) return;
  for (const child of fs.readdirSync(abs).sort()) {
    if (IGNORED_DIRS.has(child)) continue;
    walk(path.join(abs, child), base, out, predicate);
  }
}

function hashRoots(runtimeWorktree, roots, predicate = () => true) {
  const files = [];
  for (const rel of roots) walk(path.join(runtimeWorktree, rel), runtimeWorktree, files, predicate);
  const includedFiles = files.sort((a, b) => a.path.localeCompare(b.path));
  return {
    algorithm: 'sha256',
    includedFiles,
    fileCount: includedFiles.length,
    hash: sha256Json({ includedFiles }),
  };
}

export function sourceTreeHash(runtimeWorktree) {
  return hashRoots(runtimeWorktree, RUNTIME_SOURCE_ROOTS, (rel) =>
    rel === 'backend/go.mod' ||
    rel === 'backend/go.sum' ||
    rel.startsWith('backend/migrations/') ||
    (rel.startsWith('backend/') && rel.endsWith('.go')),
  );
}

export function migrationSetHash(runtimeWorktree) {
  return hashRoots(runtimeWorktree, ['backend/migrations', 'backend/internal/database'], (rel) =>
    rel.startsWith('backend/migrations/') || rel === 'backend/internal/database/migrate_p7.go',
  );
}

export function runtimeConfigSchemaHash(runtimeWorktree) {
  return hashRoots(runtimeWorktree, RUNTIME_CONFIG_ROOTS, (rel) =>
    rel === '.env.example' ||
    rel === '.env.docker.example' ||
    (rel.startsWith('backend/internal/config/') && rel.endsWith('.go')),
  );
}

export function runtimeWorktreeStatus(runtimeWorktree) {
  const status = run('git', ['status', '--porcelain=v1', '--untracked-files=all'], { cwd: runtimeWorktree });
  const rows = (status.stdout || '').split(/\r?\n/).filter(Boolean);
  const runtimeRows = rows.filter((row) => {
    const rel = row.slice(3).replaceAll('\\', '/');
    return rel.startsWith('backend/') || rel === 'backend/go.mod' || rel === 'backend/go.sum';
  });
  return {
    command: status.command,
    commandStatus: status.status,
    sourceWorkingTreeClean: status.status === 0 && runtimeRows.length === 0,
    allWorkingTreeClean: status.status === 0 && rows.length === 0,
    runtimeSourceDirtyPaths: runtimeRows.map((row) => row.slice(3).replaceAll('\\', '/')).sort(),
  };
}

export function resolveRuntime({ role, runtimeWorktree, runtimeCommit }) {
  if (!['baseline', 'current'].includes(role)) throw new Error('role must be baseline or current');
  const absRuntimeWorktree = path.resolve(runtimeWorktree || repoRoot());
  const head = run('git', ['rev-parse', 'HEAD'], { cwd: absRuntimeWorktree });
  const resolvedCommit = (head.stdout || '').trim();
  const commit = runtimeCommit || resolvedCommit;
  if (!COMMIT_RE.test(commit)) throw new Error('runtime commit must be a 40 character git SHA');
  if (resolvedCommit !== commit) throw new Error(`runtime worktree HEAD ${resolvedCommit} does not match ${commit}`);
  const exists = run('git', ['cat-file', '-e', `${commit}^{commit}`], { cwd: absRuntimeWorktree });
  if (exists.status !== 0) throw new Error(`runtime commit is not present: ${commit}`);
  const status = runtimeWorktreeStatus(absRuntimeWorktree);
  const source = sourceTreeHash(absRuntimeWorktree);
  const migrations = migrationSetHash(absRuntimeWorktree);
  const runtimeConfig = runtimeConfigSchemaHash(absRuntimeWorktree);
  return {
    formalBinaryProvenanceVersion: FORMAL_BINARY_PROVENANCE_VERSION,
    role,
    runtimeCommit: commit,
    runtimeWorktree: absRuntimeWorktree,
    sourceTreeHash: source.hash,
    sourceWorkingTreeClean: status.sourceWorkingTreeClean,
    allWorkingTreeClean: status.allWorkingTreeClean,
    runtimeSourceDirtyPaths: status.runtimeSourceDirtyPaths,
    migrationSetHash: migrations.hash,
    runtimeConfigSchemaHash: runtimeConfig.hash,
    sourceTreeFileCount: source.fileCount,
    migrationSetFileCount: migrations.fileCount,
    runtimeConfigSchemaFileCount: runtimeConfig.fileCount,
  };
}

export function receiptPathFromBinary(binaryPath) {
  return path.join(path.dirname(binaryPath), 'provenance.json');
}

export function sha256File(filePath) {
  return sha256Bytes(fs.readFileSync(filePath));
}

export function inspectBinary(binaryPath) {
  const file = run('file', [binaryPath]);
  const goVersionM = run('go', ['version', '-m', binaryPath]);
  const ldd = run('ldd', [binaryPath]);
  return {
    fileOutput: (file.stdout || file.stderr || '').trim(),
    goVersionM: (goVersionM.stdout || '').split(/\r?\n/).filter(Boolean),
    ldd: (ldd.stdout || ldd.stderr || '').split(/\r?\n/).filter(Boolean),
    fileStatus: file.status,
    goVersionMStatus: goVersionM.status,
    lddStatus: ldd.status,
  };
}

export function verifyBinaryReceipt(receiptOrPath, options = {}) {
  const receipt = typeof receiptOrPath === 'string' ? readJSON(receiptOrPath) : receiptOrPath;
  const issues = [];
  if (!receipt) issues.push('receipt_missing');
  if (receipt && receipt.formalBinaryProvenanceVersion !== FORMAL_BINARY_PROVENANCE_VERSION) issues.push('invalid_binary_provenance_version');
  if (options.role && receipt?.role !== options.role) issues.push('role_mismatch');
  if (options.runtimeCommit && receipt?.runtimeCommit !== options.runtimeCommit) issues.push('runtime_commit_mismatch');
  if (receipt && !SHA256_RE.test(receipt.binarySha256 || '')) issues.push('invalid_binary_sha256');
  if (receipt && !SHA256_RE.test(receipt.sourceTreeHash || '')) issues.push('invalid_source_tree_hash');
  if (receipt && !SHA256_RE.test(receipt.migrationSetHash || '')) issues.push('invalid_migration_set_hash');
  if (receipt && !SHA256_RE.test(receipt.runtimeConfigSchemaHash || '')) issues.push('invalid_runtime_config_schema_hash');
  if (receipt?.sourceWorkingTreeClean !== true) issues.push('runtime_source_worktree_dirty');
  if (receipt?.immutable !== true) issues.push('receipt_not_immutable');
  const binaryPath = receipt?.binaryPath ? path.resolve(repoRoot(), receipt.binaryPath) : '';
  if (!binaryPath || !fs.existsSync(binaryPath)) {
    issues.push('binary_missing');
  } else {
    const actualSha = sha256File(binaryPath);
    if (actualSha !== receipt.binarySha256) issues.push('binary_sha256_mismatch');
    const inspection = inspectBinary(binaryPath);
    if (!/ELF 64-bit/.test(inspection.fileOutput) || !/(x86-64|amd64)/i.test(inspection.fileOutput)) issues.push('linux_amd64_elf_required');
    if (/PE32|MS Windows/i.test(inspection.fileOutput)) issues.push('windows_binary_rejected');
    if (inspection.goVersionMStatus !== 0 || inspection.goVersionM.length === 0) issues.push('go_build_info_missing');
  }
  const secretLikeEntries = [];
  const walkEvidence = (value, trail = []) => {
    if (Array.isArray(value)) {
      value.forEach((entry, index) => walkEvidence(entry, [...trail, String(index)]));
      return;
    }
    if (value && typeof value === 'object') {
      for (const [key, entry] of Object.entries(value)) {
        if (SECRET_RE.test(key)) secretLikeEntries.push([...trail, key].join('.'));
        walkEvidence(entry, [...trail, key]);
      }
      return;
    }
    if (typeof value === 'string' && trail[0] !== 'goVersionM' && /(DATABASE_URL|JWT|BEGIN PRIVATE KEY)/i.test(value)) {
      secretLikeEntries.push(trail.join('.'));
    }
  };
  walkEvidence(receipt || {});
  if (secretLikeEntries.length) issues.push(`secret_like_field_in_receipt:${secretLikeEntries.join('|')}`);
  return {
    status: issues.length ? 'failed' : 'passed',
    valid: issues.length === 0,
    issues,
    receipt,
    binaryPath,
  };
}

export function resolveBinaryForRunId(runId, manifest = readJSON(BINARY_PROVENANCE_MANIFEST_PATH) || readJSON('docs/p7-v2-r3b-run-manifest.json') || {}) {
  const role = String(runId || '').includes('-baseline-') ? 'baseline' : String(runId || '').includes('-current-') ? 'current' : '';
  if (!role) return null;
  const binding = manifest.binaryProvenance?.[role] || manifest[`${role}BinaryProvenance`] || {};
  const binaryPath = binding.binaryPath || manifest[`${role}BinaryPath`] || '';
  const binarySha256 = binding.binarySha256 || manifest[`${role}BinarySha256`] || '';
  const runtimeCommit = binding.runtimeCommit || manifest[`${role}RuntimeCommit`] || '';
  const receiptPath = binding.receiptPath || manifest[`${role}BinaryReceiptPath`] || '';
  if (!binaryPath || !binarySha256 || !runtimeCommit) return null;
  return { role, binaryPath, binarySha256, runtimeCommit, receiptPath, sourceTreeHash: binding.sourceTreeHash || '' };
}

export function buildBinaryProvenanceReceipt({ role, runtimeWorktree, runtimeCommit, builtAt = new Date().toISOString() }) {
  const resolved = resolveRuntime({ role, runtimeWorktree, runtimeCommit });
  if (!resolved.sourceWorkingTreeClean) {
    throw new Error(`runtime source worktree is dirty: ${resolved.runtimeSourceDirtyPaths.join(', ')}`);
  }
  const goVersion = run('go', ['version']);
  const goEnv = {
    GOOS: 'linux',
    GOARCH: 'amd64',
    CGO_ENABLED: process.env.CGO_ENABLED || '1',
    CC: process.env.CC || 'gcc',
  };
  const tempDir = path.join(repoRoot(), BINARY_STORE_ROOT, '.build', `${role}-${process.pid}-${Date.now()}`);
  fs.mkdirSync(tempDir, { recursive: true });
  const tempBinary = path.join(tempDir, 'server');
  const buildCommand = ['go', 'build', '-trimpath', '-o', tempBinary, './cmd/server'];
  const build = run(buildCommand[0], buildCommand.slice(1), {
    cwd: path.join(resolved.runtimeWorktree, 'backend'),
    env: goEnv,
    timeout: 10 * 60 * 1000,
  });
  if (build.status !== 0) throw new Error(`go build failed: ${build.stderr.slice(0, 800)}`);
  const binarySha256 = sha256File(tempBinary);
  const storeDir = path.join(repoRoot(), BINARY_STORE_ROOT, role, binarySha256);
  fs.mkdirSync(storeDir, { recursive: true });
  const binaryPathAbs = path.join(storeDir, 'server');
  if (!fs.existsSync(binaryPathAbs)) fs.copyFileSync(tempBinary, binaryPathAbs, fs.constants.COPYFILE_EXCL);
  const inspection = inspectBinary(binaryPathAbs);
  const receipt = {
    formalBinaryProvenanceVersion: FORMAL_BINARY_PROVENANCE_VERSION,
    role,
    runtimeCommit: resolved.runtimeCommit,
    sourceTreeHash: resolved.sourceTreeHash,
    sourceWorkingTreeClean: resolved.sourceWorkingTreeClean,
    buildCommand,
    buildWorkingDirectory: `${resolved.runtimeWorktree}/backend`,
    goVersion: (goVersion.stdout || '').trim(),
    goEnv,
    buildTags: [],
    ldflags: [],
    binaryPath: path.relative(repoRoot(), binaryPathAbs).replaceAll('\\', '/'),
    binarySha256,
    binarySizeBytes: fs.statSync(binaryPathAbs).size,
    goVersionM: inspection.goVersionM,
    fileOutput: inspection.fileOutput,
    dynamicLibraryDependencies: inspection.ldd,
    migrationSetHash: resolved.migrationSetHash,
    runtimeConfigSchemaHash: resolved.runtimeConfigSchemaHash,
    builtAt,
    verifiedAt: new Date().toISOString(),
    immutable: true,
  };
  const receiptPathAbs = receiptPathFromBinary(binaryPathAbs);
  writeJSON(receiptPathAbs, receipt);
  try {
    fs.chmodSync(binaryPathAbs, 0o555);
    fs.chmodSync(receiptPathAbs, 0o444);
  } catch {
    // Windows-mounted worktrees may not support POSIX chmod; verification relies on SHA and receipt.
  }
  fs.rmSync(tempDir, { recursive: true, force: true });
  const verification = verifyBinaryReceipt(receipt);
  if (!verification.valid) throw new Error(`binary provenance verification failed: ${verification.issues.join(', ')}`);
  return {
    ...receipt,
    receiptPath: path.relative(repoRoot(), receiptPathAbs).replaceAll('\\', '/'),
    binaryProvenanceHash: sha256Json(receipt),
  };
}

export function freezeBinaryProvenance({ baselineReceiptPath, currentReceiptPath, outputPath = BINARY_PROVENANCE_MANIFEST_PATH } = {}) {
  const baseline = readJSON(baselineReceiptPath || '');
  const current = readJSON(currentReceiptPath || '');
  const baselineCheck = verifyBinaryReceipt(baseline, { role: 'baseline' });
  const currentCheck = verifyBinaryReceipt(current, { role: 'current' });
  const issues = [...baselineCheck.issues.map((issue) => `baseline:${issue}`), ...currentCheck.issues.map((issue) => `current:${issue}`)];
  if (baseline?.binarySha256 && current?.binarySha256 && baseline.binarySha256 === current.binarySha256) issues.push('baseline_current_binary_sha256_must_differ');
  const manifest = {
    phase: 'P7-V2-R3B-FORMAL-BINARY-PROVENANCE-V2',
    status: issues.length ? 'failed' : 'passed',
    formalBinaryProvenanceVersion: FORMAL_BINARY_PROVENANCE_VERSION,
    baselineRuntimeCommit: baseline?.runtimeCommit || '',
    currentRuntimeCommit: current?.runtimeCommit || '',
    baselineBinarySha256: baseline?.binarySha256 || '',
    currentBinarySha256: current?.binarySha256 || '',
    baselineBinaryPath: baseline?.binaryPath || '',
    currentBinaryPath: current?.binaryPath || '',
    baselineBinaryReceiptPath: baselineReceiptPath || '',
    currentBinaryReceiptPath: currentReceiptPath || '',
    baselineBinaryProvenanceHash: baseline ? sha256Json(baseline) : '',
    currentBinaryProvenanceHash: current ? sha256Json(current) : '',
    binaryProvenance: {
      baseline: baseline ? { ...baseline, receiptPath: baselineReceiptPath || '' } : null,
      current: current ? { ...current, receiptPath: currentReceiptPath || '' } : null,
    },
    issues,
    generatedAt: new Date().toISOString(),
  };
  writeJSON(outputPath, manifest);
  return manifest;
}
