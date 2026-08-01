import { describe, expect, it } from 'vitest';
import {
  buildBrowserExtensionConnectionInfo,
  isRemoteHttpOrigin,
  PAIRING_PREFIX,
} from '../browserExtensionPairing';

function fromBase64Url(raw: string): string {
  const b64 = raw.replace(/-/g, '+').replace(/_/g, '/') + '='.repeat((4 - (raw.length % 4)) % 4);
  return Buffer.from(b64, 'base64').toString('utf8');
}

describe('browser extension pairing info', () => {
  it('builds a TM-PAIR payload matching the extension pairing protocol', () => {
    const info = buildBrowserExtensionConnectionInfo('http://127.0.0.1:8000/', 'abcd-efgh-jk');
    expect(info.startsWith(PAIRING_PREFIX)).toBe(true);
    const payload = JSON.parse(fromBase64Url(info.slice(PAIRING_PREFIX.length))) as {
      v: number;
      apiBase: string;
      code: string;
    };
    expect(payload).toEqual({
      v: 1,
      apiBase: 'http://127.0.0.1:8000',
      code: 'ABCD-EFGH-JK',
    });
  });

  it('normalizes loopback apiBase and uppercases the code', () => {
    const info = buildBrowserExtensionConnectionInfo('http://localhost:8000', 'ab-cd');
    const payload = JSON.parse(fromBase64Url(info.slice(PAIRING_PREFIX.length))) as {
      apiBase: string;
      code: string;
    };
    expect(payload.apiBase).toBe('http://localhost:8000');
    expect(payload.code).toBe('AB-CD');
  });

  it('rejects a remote HTTP apiBase', () => {
    expect(() =>
      buildBrowserExtensionConnectionInfo('http://trade.example.com', 'abcd-efgh-jk'),
    ).toThrow('远程 TradeMind 地址必须使用 HTTPS');
  });

  it('detects remote HTTP origins that cannot connect the extension', () => {
    expect(isRemoteHttpOrigin('http://trade.example.com')).toBe(true);
    expect(isRemoteHttpOrigin('http://127.0.0.1:8000')).toBe(false);
    expect(isRemoteHttpOrigin('http://localhost:8000')).toBe(false);
    expect(isRemoteHttpOrigin('https://trade.example.com')).toBe(false);
  });
});
