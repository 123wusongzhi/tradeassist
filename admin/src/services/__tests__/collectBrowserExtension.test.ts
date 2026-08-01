import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import {
  createBrowserExtensionPairing,
  queryBrowserExtensionDevices,
  revokeBrowserExtensionDevice,
} from '../collectBrowserExtension';

const requestMock = vi.mocked(request);

describe('browser extension pairing service', () => {
  it('creates a one-time pairing code via POST', async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: 'ok',
      data: { code: 'ABCDE-FGHJK', expiresAt: '2026-08-01T00:10:00Z' },
    });

    const pairing = await createBrowserExtensionPairing();

    expect(pairing.code).toBe('ABCDE-FGHJK');
    expect(requestMock).toHaveBeenCalledWith('/api/v1/collect/browser-extension/pairings', {
      method: 'POST',
      data: {},
    });
  });

  it('lists paired devices via GET', async () => {
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { list: [] } });

    const res = await queryBrowserExtensionDevices();

    expect(res.list).toEqual([]);
    expect(requestMock).toHaveBeenCalledWith('/api/v1/collect/browser-extension/devices', {
      method: 'GET',
    });
  });

  it('revokes a device via DELETE', async () => {
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { revoked: true } });

    await revokeBrowserExtensionDevice('e2e-device-1');

    expect(requestMock).toHaveBeenCalledWith('/api/v1/collect/browser-extension/devices/e2e-device-1', {
      method: 'DELETE',
    });
  });
});
