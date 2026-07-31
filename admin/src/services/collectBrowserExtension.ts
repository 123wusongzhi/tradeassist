import { deleteJSON, getJSON, postJSON } from './request';

/** 已配对扩展设备（与后端 DeviceDTO 对齐）。 */
export type BrowserExtensionDeviceRow = {
  id: string;
  name: string;
  status: 'active' | 'expired' | 'revoked' | string;
  expiresAt: string;
  lastUsedAt?: string | null;
  createdAt: string;
};

/** 一次性配对码。 */
export type BrowserExtensionPairing = {
  code: string;
  expiresAt: string;
};

export async function createBrowserExtensionPairing(): Promise<BrowserExtensionPairing> {
  return postJSON<BrowserExtensionPairing>('/api/v1/collect/browser-extension/pairings', {});
}

export async function queryBrowserExtensionDevices(): Promise<{ list: BrowserExtensionDeviceRow[] }> {
  return getJSON<{ list: BrowserExtensionDeviceRow[] }>('/api/v1/collect/browser-extension/devices');
}

export async function revokeBrowserExtensionDevice(id: string): Promise<{ revoked: boolean }> {
  return deleteJSON<{ revoked: boolean }>(
    `/api/v1/collect/browser-extension/devices/${encodeURIComponent(id)}`,
  );
}
