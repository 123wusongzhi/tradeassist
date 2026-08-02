import { describe, expect, it } from 'vitest';
import { PinnedPublicProxy, resolvePinnedTarget } from './pinned-public-proxy.js';
import type { HostResolver, ResolvedAddress } from './public-url.js';

function sequenceResolver(...answers: ResolvedAddress[][]): { resolver: HostResolver; calls: () => number } {
  let calls = 0;
  const resolver: HostResolver = async () => {
    const index = calls++;
    const answer = answers[index];
    if (!answer) throw new Error('unexpected extra DNS lookup');
    return answer;
  };
  return { resolver, calls: () => calls };
}

describe('pinned public proxy', () => {
  it('returns the exact IP from one validated lookup', async () => {
    const sequence = sequenceResolver(
      [{ address: '8.8.8.8', family: 4 }],
      [{ address: '127.0.0.1', family: 4 }],
    );
    const target = await resolvePinnedTarget('https://rebind.example/image.png', sequence.resolver);
    expect(target.address).toEqual({ address: '8.8.8.8', family: 4 });
    expect(target.port).toBe(443);
    expect(sequence.calls()).toBe(1);
  });

  it('rejects mixed public and private DNS answers', async () => {
    const sequence = sequenceResolver([
      { address: '8.8.8.8', family: 4 },
      { address: '127.0.0.1', family: 4 },
    ]);
    await expect(resolvePinnedTarget('https://mixed.example/', sequence.resolver)).rejects.toThrow(
      'UNSAFE_TARGET_URL:address_not_public',
    );
  });

  it('rejects non-web target ports', async () => {
    const sequence = sequenceResolver([{ address: '8.8.8.8', family: 4 }]);
    await expect(resolvePinnedTarget('https://public.example:22/', sequence.resolver)).rejects.toThrow(
      'UNSAFE_TARGET_URL:port_not_allowed',
    );
  });

  it('binds only to a random loopback port', async () => {
    const proxy = new PinnedPublicProxy();
    try {
      await expect(proxy.start()).resolves.toMatch(/^http:\/\/127\.0\.0\.1:\d+$/);
    } finally {
      await proxy.close();
    }
  });
});
