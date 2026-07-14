import fs from 'node:fs';
import path from 'node:path';
import { k6Binary, root, run, runWSL } from './p7-v2-lib.mjs';

const version = process.env.K6_VERSION || 'v0.57.0';
const toolsDir = path.join(root, 'tools', 'k6');

const existing = k6Binary();
if (existing.path && existing.version) {
  console.log(JSON.stringify({ status: 'already_installed', path: existing.path, version: existing.version }, null, 2));
  process.exit(0);
}

fs.mkdirSync(toolsDir, { recursive: true });
const archive = `k6-${version}-linux-amd64.tar.gz`;
const url = `https://github.com/grafana/k6/releases/download/${version}/${archive}`;
const installCmd = [
  `mkdir -p ${JSON.stringify(toolsDir)}`,
  `cd ${JSON.stringify(toolsDir)}`,
  `curl -fsSL ${JSON.stringify(url)} -o ${JSON.stringify(archive)}`,
  `tar -xzf ${JSON.stringify(archive)}`,
  `mv k6-${version}-linux-amd64/k6 ./k6`,
  `chmod +x ./k6`,
  `./k6 version`,
].join(' && ');

const res = runWSL(installCmd, { timeout: 10 * 60 * 1000 });
const installed = k6Binary();
const ok = res.status === 0 && installed.version;
console.log(
  JSON.stringify(
    {
      status: ok ? 'installed' : 'failed',
      version: installed.version,
      path: installed.path,
      stderr: res.stderr.slice(0, 2000),
    },
    null,
    2,
  ),
);
process.exit(ok ? 0 : 1);
