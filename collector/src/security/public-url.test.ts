import type { BrowserContext, Route } from 'playwright';
import { describe, expect, it, vi } from 'vitest';
import {
  assertPublicHttpURL,
  installPublicNetworkGuard,
  isPublicAddress,
  type HostResolver,
} from './public-url.js';

const publicResolver: HostResolver = async () => [
  { address: '93.184.216.34', family: 4 },
];

describe('public URL guard', () => {
  it.each([
    'file:///etc/passwd',
    'ftp://example.com/file',
    'http://user:password@example.com/',
    'http://localhost/',
    'http://service.internal/',
    'http://127.0.0.1/',
    'http://10.0.0.1/',
    'http://169.254.169.254/latest/meta-data/',
    'http://[::1]/',
    'http://[fc00::1]/',
    'http://[::ffff:127.0.0.1]/',
    'http://[2001::1]/',
    'http://[2002:c000:0204::1]/',
    'http://[2001:10::1]/',
    'http://[2001:20::1]/',
    'http://[2001:2::1]/',
  ])('rejects unsafe target %s', async (target) => {
    await expect(assertPublicHttpURL(target, publicResolver)).rejects.toThrow(
      /UNSAFE_TARGET_URL/,
    );
  });

  it('rejects a hostname if any DNS answer is not public', async () => {
    const resolver: HostResolver = async () => [
      { address: '93.184.216.34', family: 4 },
      { address: '10.0.0.8', family: 4 },
    ];
    await expect(assertPublicHttpURL('https://shop.example/path', resolver)).rejects.toThrow(
      /address_not_public/,
    );
  });

  it('accepts public IPv4, IPv6 and public-only DNS answers', async () => {
    await expect(assertPublicHttpURL('https://8.8.8.8/', publicResolver)).resolves.toBeInstanceOf(URL);
    await expect(
      assertPublicHttpURL('https://[2606:4700:4700::1111]/', publicResolver),
    ).resolves.toBeInstanceOf(URL);
    await expect(
      assertPublicHttpURL('https://shop.example/path', publicResolver),
    ).resolves.toBeInstanceOf(URL);
    expect(isPublicAddress('192.168.1.2')).toBe(false);
    expect(isPublicAddress('1.1.1.1')).toBe(true);
  });

  it('normalizes numeric IPv4 hostnames before rejecting them', async () => {
    await expect(assertPublicHttpURL('http://2130706433/', publicResolver)).rejects.toThrow(
      /address_not_public/,
    );
    await expect(assertPublicHttpURL('http://0177.0.0.1/', publicResolver)).rejects.toThrow(
      /address_not_public/,
    );
  });

  it('fails closed when DNS lookup exceeds its timeout', async () => {
    const never: HostResolver = async () => new Promise(() => undefined);
    await expect(
      assertPublicHttpURL('https://shop.example/path', never, { dnsTimeoutMs: 1 }),
    ).rejects.toThrow(/dns_lookup_failed/);
  });

  it('guards every HTTP request in a context, including an unsafe redirect, exactly once', async () => {
    const handlers: Array<(route: Route) => Promise<void>> = [];
    const context = {
      route: async (_matcher: unknown, handler: (route: Route) => Promise<void>) => {
        handlers.push(handler);
      },
    } as unknown as BrowserContext;
    await installPublicNetworkGuard(context, publicResolver);
    await installPublicNetworkGuard(context, publicResolver);
    expect(handlers).toHaveLength(1);

    const safeAbort = vi.fn(async () => undefined);
    const safeContinue = vi.fn(async () => undefined);
    await handlers[0]({
      request: () => ({ url: () => 'https://shop.example/product' }),
      abort: safeAbort,
      continue: safeContinue,
    } as unknown as Route);
    expect(safeContinue).toHaveBeenCalledTimes(1);
    expect(safeAbort).not.toHaveBeenCalled();

    const abort = vi.fn(async () => undefined);
    const continueRequest = vi.fn(async () => undefined);
    const route = {
      request: () => ({ url: () => 'http://169.254.169.254/latest/meta-data/' }),
      abort,
      continue: continueRequest,
    } as unknown as Route;

    await handlers[0](route); // redirect/subrequest target is checked independently.
    expect(abort).toHaveBeenCalledWith('blockedbyclient');
    expect(continueRequest).not.toHaveBeenCalled();
  });
});
