import { afterEach, describe, expect, it } from 'vitest';
import { getBrowserHeadless, getDefaultNavigationTimeoutMs, getHttpPort } from '../env.js';

const KEYS = ['COLLECTOR_HTTP_ADDR', 'COLLECTOR_GOTO_TIMEOUT_MS', 'COLLECTOR_HEADLESS'] as const;
const original = Object.fromEntries(KEYS.map((key) => [key, process.env[key]]));

afterEach(() => {
  for (const key of KEYS) {
    const value = original[key];
    if (value === undefined) delete process.env[key];
    else process.env[key] = value;
  }
});

describe('collector env helpers', () => {
  it('parses colon-prefixed HTTP ports and falls back safely', () => {
    process.env.COLLECTOR_HTTP_ADDR = ':3201';
    expect(getHttpPort()).toBe(3201);

    process.env.COLLECTOR_HTTP_ADDR = 'not-a-port';
    expect(getHttpPort()).toBe(3001);
  });

  it('uses a positive navigation timeout', () => {
    process.env.COLLECTOR_GOTO_TIMEOUT_MS = '60000';
    expect(getDefaultNavigationTimeoutMs()).toBe(60000);

    process.env.COLLECTOR_GOTO_TIMEOUT_MS = '-1';
    expect(getDefaultNavigationTimeoutMs()).toBe(45000);
  });

  it('keeps headless mode on unless explicitly disabled', () => {
    delete process.env.COLLECTOR_HEADLESS;
    expect(getBrowserHeadless()).toBe(true);

    process.env.COLLECTOR_HEADLESS = '0';
    expect(getBrowserHeadless()).toBe(false);

    process.env.COLLECTOR_HEADLESS = 'false';
    expect(getBrowserHeadless()).toBe(false);
  });
});
