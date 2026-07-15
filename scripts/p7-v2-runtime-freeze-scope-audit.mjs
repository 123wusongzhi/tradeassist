import { fileURLToPath } from 'node:url';
import path from 'node:path';
import { buildRuntimeFreezeContract, RUNTIME_FREEZE_PATH } from './p7-v2-r3b-lpc-r3-runtime-freeze.mjs';
import { readJSON, run, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';
import {
  buildFormalConfigFingerprint,
  classifyFreezePath,
  CONFIG_FINGERPRINT_VERSION,
  freezeCurrentContract,
  immutableTrackedDiffHash,
  RUNTIME_FREEZE_SCOPE_VERSION,
} from './p7-v2-runtime-freeze-scope.mjs';

export const SCOPE_AUDIT_JSON_PATH = 'docs/p7-v2-r3b-runtime-freeze-scope-audit.json';
export const SCOPE_AUDIT_MD_PATH = 'docs/P7_V2_R3B_RUNTIME_FREEZE_SCOPE_AUDIT.md';

function parseGitStatus() {
  const status = run('git', ['status', '--porcelain=v1', '--untracked-files=all']);
  return (status.stdout || '')
    .split(/\r?\n/)
    .filter(Boolean)
    .map((line) => {
      const code = line.slice(0, 2).trim() || '??';
      const rawPath = line.slice(3).replaceAll('\\', '/');
      const renamed = rawPath.includes(' -> ');
      const filePath = renamed ? rawPath.split(' -> ').pop() : rawPath;
      const changeType = renamed ? 'renamed' : code === '??' ? 'added' : code.includes('D') ? 'deleted' : 'modified';
      const classified = classifyFreezePath(filePath);
      return {
        path: filePath,
        changeType,
        includedInStoredScope: classified.classification === 'immutable_execution_input',
        includedInRebuiltScope: classified.classification === 'immutable_execution_input',
        classification: classified.classification,
        reason: classified.reason,
      };
    });
}

function changedConfigFields(stored, rebuilt) {
  const storedPayload = stored.formalConfig || {};
  const rebuiltPayload = rebuilt.formalConfig || buildFormalConfigFingerprint().payload;
  const fields = [];
  const visit = (prefix, left, right) => {
    const keys = new Set([...Object.keys(left || {}), ...Object.keys(right || {})]);
    for (const key of [...keys].sort()) {
      const nextPrefix = prefix ? `${prefix}.${key}` : key;
      const l = left?.[key];
      const r = right?.[key];
      if (l && typeof l === 'object' && !Array.isArray(l) && r && typeof r === 'object' && !Array.isArray(r)) {
        visit(nextPrefix, l, r);
      } else if (JSON.stringify(l) !== JSON.stringify(r)) {
        fields.push({
          jsonPath: nextPrefix,
          storedValue: l ?? null,
          rebuiltValue: r ?? null,
          classification: nextPrefix.startsWith('version') ? 'semantic' : 'semantic',
          reason: 'canonical config v2 includes only explicit semantic execution inputs',
        });
      }
    }
  };
  visit('', storedPayload, rebuiltPayload);
  if (!stored.formalConfig && stored.configFingerprint) {
    fields.push({
      jsonPath: '$legacy.runtime.env',
      storedValue: '<legacy-env-hash-only>',
      rebuiltValue: '<canonical-config-v2>',
      classification: 'runtime_state',
      reason: 'legacy config fingerprint was built from runtime environment state and is replaced by explicit canonical config v2',
    });
  }
  return fields;
}

export function auditRuntimeFreezeScope() {
  const storedDoc = readJSON(RUNTIME_FREEZE_PATH) || {};
  const stored = freezeCurrentContract(storedDoc) || {};
  let rebuilt = {};
  let rebuildError = '';
  try {
    rebuilt = buildRuntimeFreezeContract({ manifest: readJSON('docs/p7-v2-r3b-run-manifest.json') || {}, now: stored.createdAt || new Date().toISOString() });
  } catch (error) {
    rebuildError = error.message;
  }
  const rebuiltDiff = immutableTrackedDiffHash();
  const changedFilesSinceFreeze = parseGitStatus();
  const changedConfig = changedConfigFields(stored, rebuilt);
  const unexpectedSourceChanges = changedFilesSinceFreeze.filter((file) => file.classification === 'unexpected_source_change');
  const report = {
    phase: 'P7-V2-R3B-FINAL-CLOSE-REFREEZE',
    component: 'runtime-freeze-scope-audit',
    status: rebuildError ? 'failed' : 'passed',
    scopeAuditComplete: !rebuildError,
    runtimeFreezeScopeVersion: RUNTIME_FREEZE_SCOPE_VERSION,
    configFingerprintVersion: CONFIG_FINGERPRINT_VERSION,
    storedRuntimeFreezeId: stored.runtimeFreezeId || '',
    storedTrackedDiffHash: stored.git?.immutableTrackedDiffHash || stored.git?.trackedDiffHash || '',
    rebuiltTrackedDiffHash: rebuilt.git?.immutableTrackedDiffHash || rebuiltDiff.hash,
    trackedDiffHashMatch: Boolean((stored.git?.immutableTrackedDiffHash || stored.git?.trackedDiffHash) && (stored.git?.immutableTrackedDiffHash || stored.git?.trackedDiffHash) === (rebuilt.git?.immutableTrackedDiffHash || rebuiltDiff.hash)),
    storedConfigFingerprint: stored.configFingerprint || '',
    rebuiltConfigFingerprint: rebuilt.configFingerprint || '',
    configFingerprintMatch: Boolean(stored.configFingerprint && rebuilt.configFingerprint && stored.configFingerprint === rebuilt.configFingerprint),
    storedRuntimeSourceTreeHash: stored.runtimeSourceTreeHash || '',
    rebuiltRuntimeSourceTreeHash: rebuilt.runtimeSourceTreeHash || '',
    runtimeSourceTreeHashMatch: Boolean(stored.runtimeSourceTreeHash && rebuilt.runtimeSourceTreeHash && stored.runtimeSourceTreeHash === rebuilt.runtimeSourceTreeHash),
    storedEvidenceToolingHash: stored.evidenceToolingHash || '',
    rebuiltEvidenceToolingHash: rebuilt.evidenceToolingHash || '',
    evidenceToolingHashMatch: Boolean(stored.evidenceToolingHash && rebuilt.evidenceToolingHash && stored.evidenceToolingHash === rebuilt.evidenceToolingHash),
    changedFilesSinceFreeze,
    changedConfigFields: changedConfig,
    allChangedFilesClassified: changedFilesSinceFreeze.every((file) => Boolean(file.classification)),
    allChangedConfigFieldsClassified: changedConfig.every((field) => Boolean(field.classification)),
    unexpectedSourceChangeCount: unexpectedSourceChanges.length,
    unexpectedSourceChanges,
    rebuildError,
    oldRuntimeFreezeInvalidation: {
      runtimeFreezeId: stored.runtimeFreezeId || '',
      status: 'invalidated',
      active: false,
      validForFormalExecution: false,
      reason: 'runtime_freeze_scope_and_config_fingerprint_changed',
    },
    oldRecovery6PlanSupersession: {
      baselineRunId: readJSON('docs/p7-v2-r3b-run-manifest.json')?.baselineRunId || '',
      currentRunId: readJSON('docs/p7-v2-r3b-run-manifest.json')?.currentRunId || '',
      status: 'superseded_before_formal_execution',
      active: false,
      validForExecution: false,
      reason: 'runtime_freeze_revalidation_failed',
    },
  };
  writeJSON(SCOPE_AUDIT_JSON_PATH, report);
  writeMarkdown(
    SCOPE_AUDIT_MD_PATH,
    `# P7-V2-R3B Runtime Freeze Scope Audit\n\nStatus: **${report.status}**\n\n- Stored runtime freeze ID: \`${report.storedRuntimeFreezeId}\`\n- Runtime freeze scope version: \`${report.runtimeFreezeScopeVersion}\`\n- Config fingerprint version: \`${report.configFingerprintVersion}\`\n- Runtime source match: \`${report.runtimeSourceTreeHashMatch}\`\n- Evidence tooling match: \`${report.evidenceToolingHashMatch}\`\n- Config fingerprint match: \`${report.configFingerprintMatch}\`\n- Tracked diff match: \`${report.trackedDiffHashMatch}\`\n- Changed files classified: \`${report.allChangedFilesClassified}\`\n- Changed config fields classified: \`${report.allChangedConfigFieldsClassified}\`\n- Unexpected source changes: \`${report.unexpectedSourceChangeCount}\`\n`,
  );
  return report;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = auditRuntimeFreezeScope();
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}
