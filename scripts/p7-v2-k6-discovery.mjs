import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { root, run, runWSL, valueOf } from './p7-v2-lib.mjs';

export const K6_VERSION = process.env.K6_VERSION || 'v0.57.0';
export const K6_DOCKER_IMAGE = `grafana/k6:${K6_VERSION.replace(/^v/, '')}`;

const COMMON_BINARY_PATHS = [
  () => process.env.P7_K6_BIN,
  () => path.join(root, 'tools', 'k6', 'k6'),
  () => path.join(root, 'tools', 'k6', 'k6.exe'),
  () => '/usr/bin/k6',
  () => '/usr/local/bin/k6',
  () => '/opt/k6/k6',
  () => '/root/go/bin/k6',
  () => '/mnt/c/Program Files/k6/k6.exe',
  () => '/mnt/c/tools/k6/k6.exe',
];

function sha256File(filePath) {
  try {
    const data = fs.readFileSync(filePath);
    return crypto.createHash('sha256').update(data).digest('hex');
  } catch {
    return '';
  }
}

function sha256Wsl(filePath) {
  const res = runWSL(`sha256sum ${JSON.stringify(filePath)} 2>/dev/null | awk '{print $1}'`, { timeout: 30000 });
  return res.status === 0 ? (res.stdout || '').trim() : '';
}

function probeBinaryWsl(binPath) {
  if (!binPath) return null;
  const wslPath = binPath.replace(/\\/g, '/').replace(/^([A-Za-z]):\//, (_, d) => `/mnt/${d.toLowerCase()}/`);
  const res = runWSL(`${JSON.stringify(wslPath)} version 2>&1`, { timeout: 20000 });
  const out = `${res.stdout || ''}${res.stderr || ''}`;
  if (res.status !== 0 || !out.includes('k6')) return null;
  const version = out.split('\n').find((line) => line.includes('k6'))?.trim() || '';
  const sha256 = wslPath.startsWith('/mnt/') ? sha256File(binPath.replace(/\\/g, '/').replace(/^\/mnt\/([a-z])\//, (_, d) => `${d.toUpperCase()}:/`)) : sha256Wsl(wslPath);
  return {
    mode: 'binary',
    path: wslPath,
    version,
    platform: 'linux',
    architecture: out.includes('amd64') ? 'amd64' : out.includes('arm64') ? 'arm64' : 'unknown',
    sha256: sha256 || sha256Wsl(wslPath),
    dockerImage: '',
    dockerDigest: '',
    source: binPath === process.env.P7_K6_BIN ? 'env:P7_K6_BIN' : 'filesystem',
    executable: true,
  };
}

function probeBinaryNative(binPath) {
  if (!binPath || !fs.existsSync(binPath)) return null;
  const res = run(binPath, ['version'], { timeout: 20000 });
  const out = `${res.stdout || ''}${res.stderr || ''}`;
  if (res.status !== 0 || !out.includes('k6')) return null;
  return {
    mode: 'binary',
    path: binPath,
    version: out.split('\n').find((line) => line.includes('k6'))?.trim() || '',
    platform: process.platform,
    architecture: out.includes('amd64') ? 'amd64' : 'unknown',
    sha256: sha256File(binPath),
    dockerImage: '',
    dockerDigest: '',
    source: 'filesystem',
    executable: true,
  };
}

function probePathK6() {
  const res = runWSL('command -v k6 >/dev/null 2>&1 && k6 version 2>&1 || true', { timeout: 20000 });
  const out = `${res.stdout || ''}${res.stderr || ''}`;
  if (res.status !== 0 || !out.includes('k6')) return null;
  const which = runWSL('command -v k6 2>/dev/null || true', { timeout: 10000 });
  const binPath = (which.stdout || '').trim();
  return {
    mode: 'binary',
    path: binPath || 'k6',
    version: out.split('\n').find((line) => line.includes('k6'))?.trim() || '',
    platform: 'linux',
    architecture: out.includes('amd64') ? 'amd64' : 'unknown',
    sha256: binPath ? sha256Wsl(binPath) : '',
    dockerImage: '',
    dockerDigest: '',
    source: 'PATH',
    executable: true,
  };
}

function probeDockerK6() {
  const inspect = runWSL(`docker image inspect ${JSON.stringify(K6_DOCKER_IMAGE)} 2>/dev/null`, { timeout: 30000 });
  if (inspect.status !== 0 || !inspect.stdout.includes('Id')) return null;
  let digest = '';
  try {
    const parsed = JSON.parse(inspect.stdout);
    digest = parsed[0]?.Id || parsed[0]?.RepoDigests?.[0] || '';
  } catch {
    digest = '';
  }
  const versionRes = runWSL(`docker run --rm ${JSON.stringify(K6_DOCKER_IMAGE)} version 2>&1`, { timeout: 60000 });
  const out = `${versionRes.stdout || ''}${versionRes.stderr || ''}`;
  if (versionRes.status !== 0 || !out.includes('k6')) return null;
  return {
    mode: 'docker',
    path: 'docker',
    version: out.split('\n').find((line) => line.includes('k6'))?.trim() || K6_VERSION,
    platform: 'linux',
    architecture: 'amd64',
    sha256: '',
    dockerImage: K6_DOCKER_IMAGE,
    dockerDigest: digest,
    source: 'docker-local',
    executable: true,
  };
}

export function discoverK6() {
  const checked = [];
  for (const resolver of COMMON_BINARY_PATHS) {
    const candidate = resolver();
    if (!candidate) continue;
    checked.push(candidate);
    const hit = probeBinaryWsl(candidate) || probeBinaryNative(candidate);
    if (hit) return { status: 'passed', ...hit, checked };
  }
  const pathHit = probePathK6();
  if (pathHit) return { status: 'passed', ...pathHit, checked: [...checked, 'PATH'] };
  const dockerHit = probeDockerK6();
  if (dockerHit) return { status: 'passed', ...dockerHit, checked: [...checked, K6_DOCKER_IMAGE] };
  return {
    status: 'blocked',
    mode: 'blocked',
    path: '',
    version: '',
    platform: '',
    architecture: '',
    sha256: '',
    dockerImage: '',
    dockerDigest: '',
    source: '',
    executable: false,
    checked,
  };
}

const isDiscoveryCli = process.argv[1]?.endsWith('p7-v2-k6-discovery.mjs');
if (isDiscoveryCli) {
  const args = process.argv.slice(2);
  const jsonOut = valueOf(args, '--json') || valueOf(args, '--out');
  const result = discoverK6();
  const payload = JSON.stringify(result, null, 2);
  if (jsonOut) {
    fs.mkdirSync(path.dirname(path.join(root, jsonOut)), { recursive: true });
    fs.writeFileSync(path.join(root, jsonOut), `${payload}\n`, 'utf8');
  }
  console.log(payload);
  process.exit(result.status === 'passed' ? 0 : 1);
}
