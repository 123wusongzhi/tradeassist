import fs from 'node:fs';
import path from 'node:path';
import { root, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const consumers = {
  producer: ['scripts/p7-v2-load-profile-fingerprint.mjs', 'LOAD_PROFILE_FINGERPRINT_VERSION = 3', 'CANONICAL_LOAD_PROFILE_SCHEMA_VERSION = 3'],
  artifactFreeze: ['scripts/p7-v2-artifact-freeze.mjs', 'report.loadProfileFingerprintVersion === 3', 'runtimeFreezeContractHash'],
  loadWrappers: ['scripts/p7-v2-baseline.mjs', 'validateRuntimeFreezeContract', 'runtimeFreezeId = runtimeFreeze.contractId'],
  registry: ['scripts/p7-v2-artifact-freeze.mjs', 'activeVersion: 3', 'fingerprintVersion: 3'],
  evidenceResolver: ['scripts/p7-v2-evidence-resolver.mjs', 'SUPPORTED_LOAD_PROFILE_FINGERPRINT_VERSIONS = [1, 2, 3]', 'validateLoadProfileEvidence'],
  comparability: ['scripts/p7-v2-r3-comparability-check.mjs', '[1, 2, 3]', 'loadProfileFingerprintV${fingerprintVersion}'],
  regression: ['scripts/p7-v2-regression.mjs', 'supportedLoadProfileFingerprintVersions', 'validateLoadProfileFingerprintEvidence'],
  scopedGate: ['scripts/p7-v2-r3b-lpc-r3-gatefix-final-gate.mjs', 'loadProfileFingerprintVersion', 'regressionV3FixturesPassed'],
  fastCloseOrchestrator: ['scripts/p7-v2-r3b-fast-close.mjs', 'comparability-v3', 'p7-v2:r3b:lpc-r3:determinism'],
};
const rows = Object.entries(consumers).map(([name, [file, ...needles]]) => {
  const source = fs.existsSync(path.join(root, file)) ? fs.readFileSync(path.join(root, file), 'utf8') : '';
  const supported = needles.every((needle) => source.includes(needle));
  return { consumer: name, supportsCanonicalSchemaV3: supported, supportsFingerprintV3: supported, source: file };
});
const report = { phase: 'P7-V2-R3B-LPC-R3-GATEFIX', status: rows.every((row) => row.supportsCanonicalSchemaV3 && row.supportsFingerprintV3) ? 'passed' : 'failed', consumers: rows, issues: [] };
if (report.status !== 'passed') report.issues.push('one or more consumers do not declare V3 support');
writeJSON('docs/p7-v2-r3b-lpc-r3-consumer-compatibility.json', report);
writeMarkdown('docs/P7_V2_R3B_LPC_R3_CONSUMER_COMPATIBILITY.md', `# P7-V2-R3B LPC-R3 Consumer Compatibility\n\nStatus: **${report.status}**\n\n${rows.map((row) => `- ${row.consumer}: canonical V3=${row.supportsCanonicalSchemaV3}; fingerprint V3=${row.supportsFingerprintV3}`).join('\n')}\n`);
console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
