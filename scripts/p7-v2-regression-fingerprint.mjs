const SHA256 = /^[a-f0-9]{64}$/;
export const supportedLoadProfileFingerprintVersions = [1, 2, 3];

function result(classification, details = []) {
  return { regressionAllowed: classification === 'accepted', classification, details };
}

function validStage(stage) {
  return stage && Number.isSafeInteger(stage.durationMs) && stage.durationMs > 0 &&
    Number.isSafeInteger(stage.targetVUs) && stage.targetVUs >= 0;
}

function validateV3(side, label) {
  if (!side?.loadProfileFingerprint) return result('missing_load_profile_fingerprint', [`${label} fingerprint is missing`]);
  if (!SHA256.test(side.loadProfileFingerprint)) return result('invalid_load_profile_fingerprint', [`${label} fingerprint is not a canonical SHA-256`]);
  const profile = side.canonicalLoadProfile || side.canonicalProfile;
  if (!profile || typeof profile !== 'object' || !profile.load || !Array.isArray(profile.load.stages) || !profile.load.stages.length) {
    return result('invalid_canonical_load_profile', [`${label} canonical profile is empty or has no stages`]);
  }
  if (profile.schemaVersion !== 3 || !Number.isSafeInteger(profile.load.configuredVUs) || profile.load.configuredVUs <= 0 ||
      !SHA256.test(profile.loadScript?.sha256 || '')) {
    return result('invalid_canonical_load_profile', [`${label} canonical v3 profile is invalid`]);
  }
  if (!profile.load.stages.every(validStage)) return result('invalid_canonical_stage', [`${label} canonical profile contains an invalid stage`]);
  return result('accepted');
}

export function validateLoadProfileFingerprintEvidence(baseline, current) {
  const baselineVersion = baseline?.loadProfileFingerprintVersion ?? baseline?.fingerprintVersion;
  const currentVersion = current?.loadProfileFingerprintVersion ?? current?.fingerprintVersion;
  if (!supportedLoadProfileFingerprintVersions.includes(baselineVersion) || !supportedLoadProfileFingerprintVersions.includes(currentVersion)) {
    return result('unsupported_fingerprint_version');
  }
  if (baselineVersion !== currentVersion) return result('fingerprint_version_mismatch');
  if (baselineVersion === 3) {
    const baselineCheck = validateV3(baseline, 'baseline');
    if (!baselineCheck.regressionAllowed) return baselineCheck;
    const currentCheck = validateV3(current, 'current');
    if (!currentCheck.regressionAllowed) return currentCheck;
  } else if (!baseline?.loadProfileFingerprint || !current?.loadProfileFingerprint) {
    return result('missing_load_profile_fingerprint');
  }
  if (!SHA256.test(baseline.loadProfileFingerprint) || !SHA256.test(current.loadProfileFingerprint)) {
    return result('invalid_load_profile_fingerprint');
  }
  return baseline.loadProfileFingerprint === current.loadProfileFingerprint
    ? result('accepted')
    : result('load_profile_fingerprint_mismatch');
}
