import { lookup } from 'node:dns/promises';
import { isIP } from 'node:net';
import type { BrowserContext } from 'playwright';

export type ResolvedAddress = { address: string; family: number };
export type HostResolver = (hostname: string) => Promise<ResolvedAddress[]>;
export type PublicURLGuardOptions = { dnsTimeoutMs?: number };
export type ResolvedPublicTarget = { url: URL; addresses: ResolvedAddress[] };

const DEFAULT_DNS_TIMEOUT_MS = 2_000;
const guardedContexts = new WeakSet<BrowserContext>();

const defaultResolver: HostResolver = async (hostname) =>
  lookup(hostname, { all: true, verbatim: true });

export class UnsafeTargetURLError extends Error {
  readonly code = 'UNSAFE_TARGET_URL';

  constructor(reason: string) {
    super(`UNSAFE_TARGET_URL:${reason}`);
    this.name = 'UnsafeTargetURLError';
  }
}

function parseIPv4(address: string): number[] | null {
  const parts = address.split('.');
  if (parts.length !== 4) return null;
  const bytes = parts.map((part) => Number(part));
  if (bytes.some((value) => !Number.isInteger(value) || value < 0 || value > 255)) {
    return null;
  }
  return bytes;
}

function isPublicIPv4(address: string): boolean {
  const bytes = parseIPv4(address);
  if (!bytes) return false;
  const [a, b, c] = bytes;
  if (a === 0 || a === 10 || a === 127) return false;
  if (a === 100 && b >= 64 && b <= 127) return false;
  if (a === 169 && b === 254) return false;
  if (a === 172 && b >= 16 && b <= 31) return false;
  if (a === 192 && b === 0 && c === 0) return false;
  if (a === 192 && b === 0 && c === 2) return false;
  if (a === 192 && b === 168) return false;
  if (a === 198 && (b === 18 || b === 19)) return false;
  if (a === 198 && b === 51 && c === 100) return false;
  if (a === 203 && b === 0 && c === 113) return false;
  if (a === 192 && b === 88 && c === 99) return false;
  return a > 0 && a < 224;
}

function parseIPv6(address: string): number[] | null {
  let value = address.toLowerCase().replace(/^\[|\]$/g, '').split('%')[0] ?? '';
  const ipv4Match = value.match(/(\d+\.\d+\.\d+\.\d+)$/);
  if (ipv4Match) {
    const bytes = parseIPv4(ipv4Match[1]);
    if (!bytes) return null;
    const replacement = `${((bytes[0] << 8) | bytes[1]).toString(16)}:${((bytes[2] << 8) | bytes[3]).toString(16)}`;
    value = value.slice(0, -ipv4Match[1].length) + replacement;
  }

  const halves = value.split('::');
  if (halves.length > 2) return null;
  const left = halves[0] ? halves[0].split(':') : [];
  const right = halves.length === 2 && halves[1] ? halves[1].split(':') : [];
  const missing = 8 - left.length - right.length;
  if ((halves.length === 1 && missing !== 0) || (halves.length === 2 && missing < 1)) {
    return null;
  }
  const groups = [...left, ...Array.from({ length: Math.max(missing, 0) }, () => '0'), ...right];
  if (groups.length !== 8) return null;

  const bytes: number[] = [];
  for (const group of groups) {
    if (!/^[0-9a-f]{1,4}$/.test(group)) return null;
    const value16 = Number.parseInt(group, 16);
    bytes.push(value16 >> 8, value16 & 0xff);
  }
  return bytes;
}

function isPublicIPv6(address: string): boolean {
  const bytes = parseIPv6(address);
  if (!bytes) return false;
  const allZero = bytes.every((value) => value === 0);
  const loopback = bytes.slice(0, 15).every((value) => value === 0) && bytes[15] === 1;
  if (allZero || loopback) return false;
  if ((bytes[0] & 0xfe) === 0xfc) return false;
  if (bytes[0] === 0xfe && (bytes[1] & 0xc0) === 0x80) return false;
  if (bytes[0] === 0xff) return false;

  const mappedIPv4 = bytes.slice(0, 10).every((value) => value === 0) && bytes[10] === 0xff && bytes[11] === 0xff;
  if (mappedIPv4) return isPublicIPv4(bytes.slice(12).join('.'));

  // Only globally routable unicast space is accepted. Transition and
  // documentation prefixes are rejected because they can conceal IPv4 targets.
  if ((bytes[0] & 0xe0) !== 0x20) return false;
  if (bytes[0] === 0x20 && bytes[1] === 0x02) return false; // 6to4
  if (bytes[0] === 0x20 && bytes[1] === 0x01 && bytes[2] === 0x00 && bytes[3] === 0x00) return false; // Teredo
  if (bytes[0] === 0x20 && bytes[1] === 0x01 && bytes[2] === 0x00 && (bytes[3] & 0xf0) === 0x10) return false; // ORCHID
  if (bytes[0] === 0x20 && bytes[1] === 0x01 && bytes[2] === 0x00 && (bytes[3] & 0xf0) === 0x20) return false; // ORCHIDv2
  if (bytes[0] === 0x20 && bytes[1] === 0x01 && bytes[2] === 0x00 && bytes[3] === 0x02) return false; // benchmarking
  if (bytes[0] === 0x20 && bytes[1] === 0x01 && bytes[2] === 0x0d && bytes[3] === 0xb8) return false;
  if (bytes[0] === 0x3f && bytes[1] === 0xff && (bytes[2] & 0xf0) === 0) return false;
  return true;
}

export function isPublicAddress(address: string): boolean {
  const family = isIP(address.replace(/^\[|\]$/g, ''));
  if (family === 4) return isPublicIPv4(address);
  if (family === 6) return isPublicIPv6(address);
  return false;
}

function normalizeHostname(hostname: string): string {
  return hostname.trim().toLowerCase().replace(/^\[|\]$/g, '').replace(/\.$/, '');
}

function isBlockedHostname(hostname: string): boolean {
  const host = normalizeHostname(hostname);
  return (
    host === 'localhost' ||
    host.endsWith('.localhost') ||
    host.endsWith('.local') ||
    host.endsWith('.internal') ||
    host.endsWith('.lan') ||
    host === 'home.arpa' ||
    host.endsWith('.home.arpa')
  );
}

export async function assertPublicHttpURL(
  rawURL: string,
  resolver: HostResolver = defaultResolver,
  options: PublicURLGuardOptions = {},
): Promise<URL> {
  return (await resolvePublicHttpTarget(rawURL, resolver, options)).url;
}

// resolvePublicHttpTarget returns the exact validated addresses that a caller
// must connect to. Callers must not resolve the hostname again after this step.
export async function resolvePublicHttpTarget(
  rawURL: string,
  resolver: HostResolver = defaultResolver,
  options: PublicURLGuardOptions = {},
): Promise<ResolvedPublicTarget> {
  let target: URL;
  try {
    target = new URL(rawURL);
  } catch {
    throw new UnsafeTargetURLError('invalid_url');
  }
  if (target.protocol !== 'http:' && target.protocol !== 'https:') {
    throw new UnsafeTargetURLError('scheme_not_allowed');
  }
  if (target.username || target.password) {
    throw new UnsafeTargetURLError('credentials_not_allowed');
  }
  const hostname = normalizeHostname(target.hostname);
  if (!hostname || isBlockedHostname(hostname)) {
    throw new UnsafeTargetURLError('hostname_not_allowed');
  }
  if (isIP(hostname)) {
    if (!isPublicAddress(hostname)) throw new UnsafeTargetURLError('address_not_public');
    return { url: target, addresses: [{ address: hostname, family: isIP(hostname) }] };
  }

  let addresses: ResolvedAddress[];
  try {
    const dnsTimeoutMs = options.dnsTimeoutMs ?? DEFAULT_DNS_TIMEOUT_MS;
    addresses = await new Promise<ResolvedAddress[]>((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error('dns_lookup_timeout')), dnsTimeoutMs);
      void resolver(hostname).then(
        (result) => {
          clearTimeout(timer);
          resolve(result);
        },
        (error: unknown) => {
          clearTimeout(timer);
          reject(error);
        },
      );
    });
  } catch {
    throw new UnsafeTargetURLError('dns_lookup_failed');
  }
  if (addresses.length === 0 || addresses.some(({ address }) => !isPublicAddress(address))) {
    throw new UnsafeTargetURLError('address_not_public');
  }
  return { url: target, addresses };
}

export async function installPublicNetworkGuard(
  context: BrowserContext,
  resolver: HostResolver = defaultResolver,
  options: PublicURLGuardOptions = {},
): Promise<void> {
  if (guardedContexts.has(context)) return;
  guardedContexts.add(context);
  try {
    await context.route(/^https?:\/\//i, async (route) => {
      try {
        await assertPublicHttpURL(route.request().url(), resolver, options);
        await route.continue();
      } catch (error) {
        if (error instanceof UnsafeTargetURLError) {
          await route.abort('blockedbyclient');
          return;
        }
        throw error;
      }
    });
  } catch (error) {
    guardedContexts.delete(context);
    throw error;
  }
}
