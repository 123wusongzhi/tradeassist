import { afterEach, describe, expect, it } from 'vitest';
import { getOpenCliBridgeConfig } from './config.js';

const originalAddress = process.env.OPENCLI_BRIDGE_HTTP_ADDR;
const originalToken = process.env.OPENCLI_BRIDGE_TOKEN;

afterEach(() => {
  if (originalAddress === undefined) delete process.env.OPENCLI_BRIDGE_HTTP_ADDR;
  else process.env.OPENCLI_BRIDGE_HTTP_ADDR = originalAddress;
  if (originalToken === undefined) delete process.env.OPENCLI_BRIDGE_TOKEN;
  else process.env.OPENCLI_BRIDGE_TOKEN = originalToken;
});

describe('OpenCLI Bridge config', () => {
  it('allows tokenless loopback development', () => {
    process.env.OPENCLI_BRIDGE_HTTP_ADDR = '127.0.0.1:3100';
    delete process.env.OPENCLI_BRIDGE_TOKEN;
    expect(getOpenCliBridgeConfig()).toEqual({
      host: '127.0.0.1',
      port: 3100,
      token: '',
    });
  });

  it('requires authentication on non-loopback bindings', () => {
    process.env.OPENCLI_BRIDGE_HTTP_ADDR = '0.0.0.0:3100';
    delete process.env.OPENCLI_BRIDGE_TOKEN;
    expect(() => getOpenCliBridgeConfig()).toThrow(/OPENCLI_BRIDGE_TOKEN/);
  });
});
