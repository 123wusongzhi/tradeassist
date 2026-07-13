import { request } from '@umijs/max';

export type BackupJob = {
  id: string;
  backupId: string;
  backupType: string;
  environment: string;
  status: string;
  verificationStatus: string;
  storageProvider: string;
  encrypted: boolean;
  artifactSize?: number;
  createdAt: string;
  completedAt?: string;
  errorSummary?: string;
};

export type RestoreJob = {
  id: string;
  restoreId: string;
  backupId: string;
  targetEnvironment: string;
  status: string;
  safetyGateStatus: string;
  validationStatus?: string;
  createdAt: string;
  completedAt?: string;
  errorSummary?: string;
};

export type ReleaseRun = {
  id: string;
  releaseId: string;
  version: string;
  gitCommit?: string;
  environment: string;
  strategy: string;
  state: string;
  preBackupId?: string;
  createdAt: string;
  completedAt?: string;
  errorSummary?: string;
};

export type DRStatus = {
  status: string;
  rpoTarget: string;
  rtoTarget: string;
  realProductionDRVerification: string;
  realProductionBackupVerification: string;
  realPITRDrill: string;
  lastDrill?: Record<string, unknown>;
};

type ListResult<T> = {
  items: T[];
  total: number;
  page: number;
  pageSize: number;
};

export async function fetchBackups(params?: { page?: number; pageSize?: number }) {
  return request<{ data: ListResult<BackupJob> }>('/api/v1/ops/backups', { method: 'GET', params });
}

export async function createBackup(data?: { dryRun?: boolean; reason?: string }) {
  return request<{ data: BackupJob }>('/api/v1/ops/backups', { method: 'POST', data });
}

export async function verifyBackup(id: string) {
  return request(`/api/v1/ops/backups/${id}/verify`, { method: 'POST' });
}

export async function holdBackup(id: string, reason: string) {
  return request(`/api/v1/ops/backups/${id}/hold`, {
    method: 'POST',
    data: { holdType: 'manual_hold', reason },
  });
}

export async function fetchRestores(params?: { page?: number; pageSize?: number }) {
  return request<{ data: ListResult<RestoreJob> }>('/api/v1/ops/restores', { method: 'GET', params });
}

export async function createRestore(data: {
  backupId: string;
  targetEnvironment: string;
  targetDatabaseName: string;
  targetIsIsolated: boolean;
  operatorReauthenticated: boolean;
  highRiskConfirmed: boolean;
}) {
  return request<{ data: RestoreJob }>('/api/v1/ops/restores', { method: 'POST', data });
}

export async function verifyRestore(id: string) {
  return request(`/api/v1/ops/restores/${id}/verify`, { method: 'POST' });
}

export async function fetchReleases(params?: { page?: number; pageSize?: number }) {
  return request<{ data: ListResult<ReleaseRun> }>('/api/v1/ops/releases', { method: 'GET', params });
}

export async function createRelease(data: { version: string; gitCommit?: string }) {
  return request<{ data: ReleaseRun }>('/api/v1/ops/releases', { method: 'POST', data });
}

export async function executeRelease(id: string) {
  return request(`/api/v1/ops/releases/${id}/execute`, { method: 'POST' });
}

export async function rollbackRelease(id: string, reason: string) {
  return request(`/api/v1/ops/releases/${id}/rollback`, { method: 'POST', data: { reason } });
}

export async function fetchDRStatus() {
  return request<{ data: DRStatus }>('/api/v1/ops/dr/status', { method: 'GET' });
}

export async function createDRDrill(data: {
  drillType: string;
  backupId?: string;
  restoreId?: string;
  releaseId?: string;
  confirmedIsolated: boolean;
}) {
  return request('/api/v1/ops/dr/drills', { method: 'POST', data });
}
