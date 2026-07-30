import { deleteJSON, getJSON, postJSON } from '@/services/request';
import { fetchFiles, type FileRow } from '@/services/files';

/** GET /api/v1/security/overview */
export type SecurityOverview = {
  authSessionMode: string;
  accessTokenTTLMinutes: number;
  refreshTokenTTLDays: number;
  secureCookie: boolean;
  loginMaxAttempts: number;
  passwordMinLength: number;
  jwtActiveKeyId: string;
  appMasterActiveKeyId: string;
  activeSessionCount: number;
  productionDebugSurface: boolean;
};

export async function fetchSecurityOverview() {
  return getJSON<SecurityOverview>('/api/v1/security/overview');
}

/** GET /api/v1/auth/sessions */
export type AuthSessionRow = {
  id: string;
  deviceSummary?: string;
  browserSummary?: string;
  createdAt: string;
  lastActivityAt: string;
  status: string;
  isCurrent: boolean;
  userAgentSummary?: string;
};

export async function fetchAuthSessions() {
  return getJSON<{ items: AuthSessionRow[] }>('/api/v1/auth/sessions');
}

/** DELETE /api/v1/auth/sessions/:id */
export async function revokeAuthSession(id: string) {
  return deleteJSON<{ ok: boolean }>(`/api/v1/auth/sessions/${id}`);
}

/** POST /api/v1/auth/sessions/revoke-others */
export async function revokeOtherAuthSessions() {
  return postJSON<{ revoked: number }>('/api/v1/auth/sessions/revoke-others');
}

/** POST /api/v1/auth/logout-all */
export async function logoutAllSessions() {
  return postJSON<{ revoked: number }>('/api/v1/auth/logout-all');
}

/** GET /api/v1/security/keys/rotation/status | POST .../prepare */
export type KeyRotationStatus = {
  activeKeyId: string;
  pendingReencrypt: number;
  previousKeyCount: number;
  lastVerifiedAt?: string;
  integrityOk?: boolean;
  integrityCheckedAt?: string;
};

export async function fetchKeyRotationStatus() {
  return getJSON<KeyRotationStatus>('/api/v1/security/keys/rotation/status');
}

export async function prepareKeyRotation(confirmPhrase: string) {
  return postJSON<KeyRotationStatus>('/api/v1/security/keys/rotation/prepare', { confirmPhrase });
}

/** POST /api/v1/security/keys/rotation/start */
export type KeyRotationJob = {
  id: string;
  activeKeyId: string;
  sourceKeyIds?: string;
  scope: string;
  tenantId: number;
  tableScope?: string;
  dryRun: boolean;
  status: string;
  totalRecords: number;
  processedRecords: number;
  reencryptedRecords: number;
  skippedRecords: number;
  failedRecords: number;
  lastCursor?: string;
  startedBy?: string;
  startedAt?: string;
  finishedAt?: string;
  verificationStatus?: string;
  createdAt: string;
  updatedAt: string;
};

export async function startKeyRotation(confirmPhrase: string) {
  return postJSON<KeyRotationJob>('/api/v1/security/keys/rotation/start', { confirmPhrase });
}

export async function fetchKeyRotationJob(id: string) {
  return getJSON<KeyRotationJob>(`/api/v1/security/keys/rotation/${id}`);
}

export async function fetchKeyRotationProgress(id: string) {
  return getJSON<KeyRotationJob>(`/api/v1/security/keys/rotation/${id}/progress`);
}

export async function pauseKeyRotation(id: string) {
  return postJSON<{ paused: boolean }>(`/api/v1/security/keys/rotation/${id}/pause`);
}

export async function resumeKeyRotation(id: string) {
  return postJSON<{ resumed: boolean }>(`/api/v1/security/keys/rotation/${id}/resume`);
}

export async function verifyKeyRotation(id: string) {
  return postJSON<{ ok: boolean; references: SecretReferenceCount[] }>(
    `/api/v1/security/keys/rotation/${id}/verify`,
  );
}

/** GET /api/v1/security/keys/references */
export type SecretReferenceCount = {
  tableName: string;
  fieldName: string;
  tenantId: number;
  keyId: string;
  referenceCount: number;
  decryptFailures: number;
  unknownFormat: number;
};

export async function fetchKeyReferences() {
  return getJSON<{ items: SecretReferenceCount[] }>('/api/v1/security/keys/references');
}

/** GET /api/v1/security/audit/integrity/status */
export type AuditIntegrityStatus = {
  ok: boolean;
  checked: number;
};

export async function fetchAuditIntegrityStatus() {
  return getJSON<AuditIntegrityStatus>('/api/v1/security/audit/integrity/status');
}

/** POST /api/v1/security/audit/integrity/verify */
export async function verifyAuditIntegrity(days = 7) {
  return postJSON<AuditIntegrityStatus>('/api/v1/security/audit/integrity/verify', { days });
}

/** File security stats derived from files list (no dedicated backend endpoint yet). */
export type FileSecurityStats = {
  total: number;
  byStatus: Record<string, number>;
  sampled: number;
  partial: boolean;
};

const FILE_SECURITY_STATUSES = [
  'clean',
  'pending_scan',
  'scanning',
  'rejected',
  'quarantined',
  'scan_failed',
  'uploaded',
] as const;

export async function fetchFileSecurityStats(): Promise<FileSecurityStats> {
  const res = await fetchFiles({ page: 1, pageSize: 100 });
  const byStatus: Record<string, number> = {};
  for (const status of FILE_SECURITY_STATUSES) {
    byStatus[status] = 0;
  }
  for (const row of res.list) {
    const status = (row as FileRow & { securityStatus?: string }).securityStatus || 'unknown';
    byStatus[status] = (byStatus[status] || 0) + 1;
  }
  return {
    total: res.pagination.total,
    byStatus,
    sampled: res.list.length,
    partial: res.pagination.total > res.list.length,
  };
}
