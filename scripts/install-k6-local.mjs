import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { discoverK6, K6_VERSION } from './p7-v2-k6-discovery.mjs';
import { root, runWSL, valueOf } from './p7-v2-lib.mjs';

const args = process.argv.slice(2);
const version = process.env.K6_VERSION || K6_VERSION;
const toolsDir = path.join(root, 'tools', 'k6');
const finalPath = path.join(toolsDir, 'k6');
const archiveArg = valueOf(args, '--archive');
const binaryArg = valueOf(args, '--binary');
const sha256Arg = valueOf(args, '--sha256');
const allowGoInstall = !args.includes('--no-go-install');
const force = args.includes('--force');

function sha256Buffer(buf) {
  return crypto.createHash('sha256').update(buf).digest('hex');
}

function sha256File(filePath) {
  return sha256Buffer(fs.readFileSync(filePath));
}

function isHtml(buf) {
  const head = buf.slice(0, 256).toString('utf8').toLowerCase();
  return head.includes('<!doctype html') || head.includes('<html');
}

function verifyElfLinux(filePath) {
  const res = runWSL(`file ${JSON.stringify(filePath)} 2>/dev/null`, { timeout: 15000 });
  const out = `${res.stdout || ''}${res.stderr || ''}`.toLowerCase();
  return out.includes('elf') && out.includes('x86-64');
}

function verifyInstalledBinary(binPath) {
  const res = runWSL(`${JSON.stringify(binPath)} version 2>&1`, { timeout: 20000 });
  const out = `${res.stdout || ''}${res.stderr || ''}`;
  if (res.status !== 0 || !out.includes('k6')) {
    throw new Error(`k6 binary verification failed: ${out.slice(0, 500)}`);
  }
  if (!out.includes(version.replace(/^v/, '')) && !out.includes(version)) {
    throw new Error(`unexpected k6 version: ${out.split('\n')[0]}`);
  }
  if (!verifyElfLinux(binPath)) {
    throw new Error('binary is not a Linux amd64 ELF executable');
  }
  return out.split('\n').find((line) => line.includes('k6'))?.trim() || '';
}

function atomicInstallFromSource(sourcePath, expectedSha256 = '') {
  fs.mkdirSync(toolsDir, { recursive: true });
  const wslSource = sourcePath.replace(/\\/g, '/').replace(/^([A-Za-z]):\//, (_, d) => `/mnt/${d.toLowerCase()}/`);
  const wslFinal = finalPath.replace(/\\/g, '/').replace(/^([A-Za-z]):\//, (_, d) => `/mnt/${d.toLowerCase()}/`);
  const wslTmp = `${wslFinal}.tmp-${Date.now()}`;
  if (expectedSha256) {
    const sum = runWSL(`sha256sum ${JSON.stringify(wslSource)} | awk '{print $1}'`, { timeout: 30000 });
    const actual = (sum.stdout || '').trim();
    if (actual !== expectedSha256.toLowerCase()) {
      throw new Error(`sha256 mismatch: expected ${expectedSha256}, got ${actual || 'unknown'}`);
    }
  }
  const installCmd = [
    `cp ${JSON.stringify(wslSource)} ${JSON.stringify(wslTmp)}`,
    `chmod +x ${JSON.stringify(wslTmp)}`,
    `${JSON.stringify(wslTmp)} version`,
    `mv ${JSON.stringify(wslTmp)} ${JSON.stringify(wslFinal)}`,
  ].join(' && ');
  const res = runWSL(installCmd, { timeout: 120000 });
  if (res.status !== 0) {
    runWSL(`rm -f ${JSON.stringify(wslTmp)}`, { timeout: 10000 });
    throw new Error(`atomic install failed: ${res.stderr.slice(0, 1000)}`);
  }
  return verifyInstalledBinary(wslFinal);
}

function existsWslOrNative(filePath) {
  if (!filePath) return false;
  if (fs.existsSync(filePath)) return true;
  if (filePath.startsWith('/')) {
    const res = runWSL(`test -f ${JSON.stringify(filePath)} && echo yes || true`, { timeout: 10000 });
    return (res.stdout || '').trim() === 'yes';
  }
  return false;
}

function installFromLocalBinary(localBinary) {
  if (!existsWslOrNative(localBinary)) {
    throw new Error(`local binary not found: ${localBinary}`);
  }
  if (localBinary.startsWith('/') && !fs.existsSync(localBinary)) {
    return atomicInstallFromSource(localBinary, sha256Arg);
  }
  const stat = fs.statSync(localBinary);
  if (!stat.isFile() || stat.size < 1024) {
    throw new Error('local binary is empty or too small');
  }
  return atomicInstallFromSource(localBinary, sha256Arg);
}

function installFromArchive(archivePath) {
  if (!fs.existsSync(archivePath)) {
    throw new Error(`archive not found: ${archivePath}`);
  }
  const archiveBuf = fs.readFileSync(archivePath);
  if (archiveBuf.length < 1024 || isHtml(archiveBuf)) {
    throw new Error('archive looks invalid or is an HTML error page');
  }
  if (sha256Arg && sha256Buffer(archiveBuf) !== sha256Arg.toLowerCase()) {
    throw new Error('archive sha256 mismatch');
  }
  const wslArchive = archivePath.replace(/\\/g, '/').replace(/^([A-Za-z]):\//, (_, d) => `/mnt/${d.toLowerCase()}/`);
  const wslTools = toolsDir.replace(/\\/g, '/').replace(/^([A-Za-z]):\//, (_, d) => `/mnt/${d.toLowerCase()}/`);
  const tmpDir = `${wslTools}/.tmp-${Date.now()}`;
  const extractCmd = [
    `rm -rf ${JSON.stringify(tmpDir)}`,
    `mkdir -p ${JSON.stringify(tmpDir)}`,
    `tar -xzf ${JSON.stringify(wslArchive)} -C ${JSON.stringify(tmpDir)}`,
    `find ${JSON.stringify(tmpDir)} -name k6 -type f | head -1`,
  ].join(' && ');
  const extract = runWSL(extractCmd, { timeout: 120000 });
  if (extract.status !== 0) {
    runWSL(`rm -rf ${JSON.stringify(tmpDir)}`, { timeout: 10000 });
    throw new Error(`archive extract failed: ${extract.stderr.slice(0, 1000)}`);
  }
  const extracted = (extract.stdout || '').trim().split('\n').pop();
  if (!extracted) {
    runWSL(`rm -rf ${JSON.stringify(tmpDir)}`, { timeout: 10000 });
    throw new Error('k6 binary not found inside archive');
  }
  const versionLine = verifyInstalledBinary(extracted);
  atomicInstallFromSource(extracted.replace(/^\/mnt\/([a-z])\//, (_, d) => `${d.toUpperCase()}:/`), '');
  runWSL(`rm -rf ${JSON.stringify(tmpDir)}`, { timeout: 10000 });
  return versionLine;
}

function installViaGo() {
  const wslFinal = finalPath.replace(/\\/g, '/').replace(/^([A-Za-z]):\//, (_, d) => `/mnt/${d.toLowerCase()}/`);
  const wslTmp = `${wslFinal}.tmp-${Date.now()}`;
  const cmd = [
    'export GOPROXY=https://goproxy.cn,direct',
    `go install go.k6.io/k6@${version}`,
    'test -x ~/go/bin/k6',
    `cp ~/go/bin/k6 ${JSON.stringify(wslTmp)}`,
    `chmod +x ${JSON.stringify(wslTmp)}`,
    `${JSON.stringify(wslTmp)} version`,
    `mv ${JSON.stringify(wslTmp)} ${JSON.stringify(wslFinal)}`,
  ].join(' && ');
  const res = runWSL(cmd, { timeout: 15 * 60 * 1000 });
  if (res.status !== 0) {
    runWSL(`rm -f ${JSON.stringify(wslTmp)}`, { timeout: 10000 });
    throw new Error(`go install failed: ${res.stderr.slice(0, 2000)}`);
  }
  return verifyInstalledBinary(wslFinal);
}

function installViaDownload() {
  if (args.includes('--offline-only')) {
    throw new Error('network install disabled (--offline-only) and no local source provided');
  }
  const wslTools = toolsDir.replace(/\\/g, '/').replace(/^([A-Za-z]):\//, (_, d) => `/mnt/${d.toLowerCase()}/`);
  const archive = `k6-${version}-linux-amd64.tar.gz`;
  const url = `https://github.com/grafana/k6/releases/download/${version}/${archive}`;
  const tmpDir = `${wslTools}/.tmp-${Date.now()}`;
  const cmd = [
    `rm -rf ${JSON.stringify(tmpDir)}`,
    `mkdir -p ${JSON.stringify(tmpDir)}`,
    `curl -fsSL ${JSON.stringify(url)} -o ${JSON.stringify(`${tmpDir}/${archive}`)}`,
    `tar -xzf ${JSON.stringify(`${tmpDir}/${archive}`)} -C ${JSON.stringify(tmpDir)}`,
    `test -x ${JSON.stringify(`${tmpDir}/k6-${version}-linux-amd64/k6`)}`,
    `${JSON.stringify(`${tmpDir}/k6-${version}-linux-amd64/k6`)} version`,
  ].join(' && ');
  const res = runWSL(cmd, { timeout: 10 * 60 * 1000 });
  if (res.status !== 0) {
    runWSL(`rm -rf ${JSON.stringify(tmpDir)}`, { timeout: 10000 });
    throw new Error(`download install failed: ${res.stderr.slice(0, 2000)}`);
  }
  const extracted = `${tmpDir}/k6-${version}-linux-amd64/k6`;
  verifyInstalledBinary(extracted);
  const wslFinal = finalPath.replace(/\\/g, '/').replace(/^([A-Za-z]):\//, (_, d) => `/mnt/${d.toLowerCase()}/`);
  const wslTmp = `${wslFinal}.tmp-${Date.now()}`;
  const finalize = runWSL(
    [
      `cp ${JSON.stringify(extracted)} ${JSON.stringify(wslTmp)}`,
      `chmod +x ${JSON.stringify(wslTmp)}`,
      `mv ${JSON.stringify(wslTmp)} ${JSON.stringify(wslFinal)}`,
      `rm -rf ${JSON.stringify(tmpDir)}`,
    ].join(' && '),
    { timeout: 120000 },
  );
  if (finalize.status !== 0) {
    runWSL(`rm -rf ${JSON.stringify(tmpDir)} ${JSON.stringify(wslTmp)}`, { timeout: 10000 });
    throw new Error(`finalize download install failed: ${finalize.stderr.slice(0, 1000)}`);
  }
  return verifyInstalledBinary(wslFinal);
}

const existing = discoverK6();
if (existing.status === 'passed' && existing.path.includes('/tools/k6/k6') && !force) {
  console.log(JSON.stringify({ status: 'already_installed', ...existing }, null, 2));
  process.exit(0);
}

let versionLine = '';
let method = '';
try {
  if (binaryArg) {
    method = 'local-binary';
    versionLine = installFromLocalBinary(binaryArg);
  } else if (archiveArg) {
    method = 'local-archive';
    versionLine = installFromArchive(archiveArg);
  } else if (allowGoInstall) {
    method = 'go-install';
    versionLine = installViaGo();
  } else {
    method = 'download';
    versionLine = installViaDownload();
  }
} catch (err) {
  if (!binaryArg && !archiveArg && method !== 'go-install' && allowGoInstall) {
    try {
      method = 'go-install-fallback';
      versionLine = installViaGo();
    } catch (goErr) {
      if (!args.includes('--offline-only')) {
        try {
          method = 'download-fallback';
          versionLine = installViaDownload();
        } catch (downloadErr) {
          console.log(
            JSON.stringify(
              {
                status: 'failed',
                method,
                error: err.message,
                goError: goErr.message,
                downloadError: downloadErr.message,
              },
              null,
              2,
            ),
          );
          process.exit(1);
        }
      } else {
        console.log(JSON.stringify({ status: 'failed', method, error: goErr.message }, null, 2));
        process.exit(1);
      }
    }
  } else if (method !== 'download' && !args.includes('--offline-only')) {
    try {
      method = 'download-fallback';
      versionLine = installViaDownload();
    } catch (downloadErr) {
      console.log(
        JSON.stringify(
          {
            status: 'failed',
            method,
            error: err.message,
            downloadError: downloadErr.message,
          },
          null,
          2,
        ),
      );
      process.exit(1);
    }
  } else {
    console.log(JSON.stringify({ status: 'failed', method, error: err.message }, null, 2));
    process.exit(1);
  }
}

const installed = discoverK6();
const ok = installed.status === 'passed' && installed.path.includes('/tools/k6/k6');
console.log(
  JSON.stringify(
    {
      status: ok ? 'installed' : 'failed',
      method,
      version: versionLine || installed.version,
      path: installed.path,
      sha256: installed.sha256,
      platform: installed.platform,
      architecture: installed.architecture,
    },
    null,
    2,
  ),
);
process.exit(ok ? 0 : 1);
