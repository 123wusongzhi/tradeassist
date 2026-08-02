import { getPinnedPublicProxyServer } from './pinned-public-proxy.js';

// Chromium otherwise has an implicit loopback proxy bypass and may use QUIC,
// either of which would escape the TCP proxy's DNS-pinned connection path.
export function pinnedChromiumArgs(existing: string[]): string[] {
  return [...existing, '--disable-quic', '--proxy-bypass-list=<-loopback>'];
}

export async function pinnedBrowserProxy(): Promise<{ server: string }> {
  return { server: await getPinnedPublicProxyServer() };
}
