import { describe, expect, it } from 'vitest';
import { diffJSON, parseJSONInput, redactSensitiveValue, safeMetadata } from '../components/OperationTaskShared';

describe('operation task shared helpers', () => {
  it('redacts nested sensitive values', () => {
    expect(redactSensitiveValue({ token: 'abc', nested: { refreshToken: 'def', title: 'ok' } })).toEqual({
      token: '******',
      nested: { refreshToken: '******', title: 'ok' },
    });
  });

  it('only exposes allowlisted event metadata', () => {
    expect(safeMetadata({ platform: 'douyin', payload: { secret: 'raw' }, adapterMode: 'mock', cookie: 'x' })).toEqual({
      platform: 'douyin',
      adapterMode: 'mock',
    });
  });

  it('validates JSON editor input without executing payload content', () => {
    expect(parseJSONInput('{"title":"ok"}')).toEqual({ ok: true, value: { title: 'ok' } });
    expect(parseJSONInput('{title')).toMatchObject({ ok: false });
  });

  it('builds safe diff rows without exposing changed secrets', () => {
    const rows = diffJSON({ title: 'old', secret: 'raw' }, { title: 'new', secret: 'changed' });
    expect(rows.some((row) => String(row.after).includes('new'))).toBe(true);
    expect(rows.some((row) => String(row.before).includes('raw'))).toBe(false);
    expect(rows.some((row) => String(row.after).includes('changed'))).toBe(false);
  });
});
