import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const packages = JSON.parse(fs.readFileSync(path.join(root, 'docs/p7-c-race-package-mapping.json'), 'utf8')).packages || [];

if (process.platform !== 'linux') {
  const report = {
    phase: 'P7-C',
    status: 'blocked',
    environmentBlocked: true,
    allMandatoryCapabilitiesMapped: packages.every((item) => item.status === 'mapped'),
    dataRaces: null,
    deadlocks: null,
    exitCode: null,
    issues: ['Linux race verification must run under WSL2/Linux; current Node platform is not linux.'],
  };
  fs.writeFileSync(path.join(root, 'docs/p7-c-race-test-report.json'), `${JSON.stringify(report, null, 2)}\n`, 'utf8');
  fs.writeFileSync(path.join(root, 'docs/P7_C_RACE_TEST_REPORT.md'), `# P7-C Race Test Report\n\nStatus: ${report.status}\n\n- ${report.issues[0]}\n`, 'utf8');
  console.log(JSON.stringify(report, null, 2));
  process.exit(1);
}

const goPackages = packages.filter((item) => item.status === 'mapped').map((item) => item.actualPackage);
const run = spawnSync('go', ['test', '-race', ...goPackages], {
  cwd: path.join(root, 'backend'),
  env: { ...process.env, CGO_ENABLED: '1' },
  encoding: 'utf8',
});
const output = `${run.stdout}\n${run.stderr}`;
const report = {
  phase: 'P7-C',
  status: run.status === 0 ? 'passed' : 'failed',
  environmentBlocked: false,
  allMandatoryCapabilitiesMapped: packages.every((item) => item.status === 'mapped'),
  dataRaces: /DATA RACE/.test(output) ? 1 : 0,
  deadlocks: /deadlock/i.test(output) ? 1 : 0,
  exitCode: run.status,
  packages: goPackages,
  issues: run.status === 0 ? [] : ['go test -race failed; inspect local command output.'],
};
fs.writeFileSync(path.join(root, 'docs/p7-c-race-test-report.json'), `${JSON.stringify(report, null, 2)}\n`, 'utf8');
fs.writeFileSync(path.join(root, 'docs/P7_C_RACE_TEST_REPORT.md'), `# P7-C Race Test Report\n\nStatus: ${report.status}\n\nExit code: ${report.exitCode}\n`, 'utf8');
console.log(JSON.stringify(report, null, 2));
process.exit(run.status === 0 ? 0 : 1);
