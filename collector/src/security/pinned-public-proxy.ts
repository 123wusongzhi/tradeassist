import http, { type IncomingHttpHeaders, type IncomingMessage, type ServerResponse } from 'node:http';
import https from 'node:https';
import net, { type AddressInfo, type Socket } from 'node:net';
import {
  resolvePublicHttpTarget,
  UnsafeTargetURLError,
  type HostResolver,
  type PublicURLGuardOptions,
  type ResolvedAddress,
} from './public-url.js';

const LOOPBACK_HOST = '127.0.0.1';
const ALLOWED_TARGET_PORTS = new Set([80, 443]);
const DEFAULT_SOCKET_TIMEOUT_MS = 45_000;

type ConnectSocket = (address: string, port: number, family: number) => Socket;

export type PinnedTarget = {
  url: URL;
  address: ResolvedAddress;
  port: number;
};

export type PinnedPublicProxyOptions = {
  resolver?: HostResolver;
  guard?: PublicURLGuardOptions;
  connectSocket?: ConnectSocket;
  socketTimeoutMs?: number;
};

function effectivePort(target: URL): number {
  if (target.port) return Number(target.port);
  return target.protocol === 'https:' ? 443 : 80;
}

export async function resolvePinnedTarget(
  rawURL: string,
  resolver?: HostResolver,
  guard?: PublicURLGuardOptions,
): Promise<PinnedTarget> {
  const resolved = await resolvePublicHttpTarget(rawURL, resolver, guard);
  const port = effectivePort(resolved.url);
  if (!Number.isInteger(port) || !ALLOWED_TARGET_PORTS.has(port)) {
    throw new UnsafeTargetURLError('port_not_allowed');
  }
  const address = resolved.addresses[0];
  if (!address) throw new UnsafeTargetURLError('dns_lookup_failed');
  return { url: resolved.url, address, port };
}

function connectURL(authority: string): string {
  const value = authority.trim();
  if (!value || /[\s/@?#]/.test(value)) throw new UnsafeTargetURLError('invalid_connect_target');
  const bracketedIPv6 = value.startsWith('[');
  if (!bracketedIPv6 && value.split(':').length > 2) {
    throw new UnsafeTargetURLError('invalid_connect_target');
  }
  return `https://${value}/`;
}

const hopByHopHeaders = new Set([
  'connection',
  'keep-alive',
  'proxy-authenticate',
  'proxy-authorization',
  'proxy-connection',
  'te',
  'trailer',
  'transfer-encoding',
  'upgrade',
]);

function forwardedHeaders(source: IncomingHttpHeaders, host: string): IncomingHttpHeaders {
  const headers: IncomingHttpHeaders = {};
  for (const [name, value] of Object.entries(source)) {
    if (!hopByHopHeaders.has(name.toLowerCase()) && value !== undefined) headers[name] = value;
  }
  headers.host = host;
  return headers;
}

function failHTTP(res: ServerResponse, status: number): void {
  if (!res.headersSent) {
    res.writeHead(status, { 'content-type': 'text/plain; charset=utf-8', 'cache-control': 'no-store' });
  }
  res.end(status === 403 ? 'target blocked' : 'proxy request failed');
}

function failSocket(socket: Socket, status = 403): void {
  if (!socket.destroyed) {
    const reason = status === 403 ? 'Forbidden' : 'Bad Gateway';
    socket.end(`HTTP/1.1 ${status} ${reason}\r\nConnection: close\r\nContent-Length: 0\r\n\r\n`);
  }
}

export class PinnedPublicProxy {
  private readonly server: http.Server;
  private readonly resolver?: HostResolver;
  private readonly guard?: PublicURLGuardOptions;
  private readonly connectSocket: ConnectSocket;
  private readonly socketTimeoutMs: number;
  private address: string | null = null;

  constructor(options: PinnedPublicProxyOptions = {}) {
    this.resolver = options.resolver;
    this.guard = options.guard;
    this.socketTimeoutMs = Math.max(1_000, options.socketTimeoutMs ?? DEFAULT_SOCKET_TIMEOUT_MS);
    this.connectSocket =
      options.connectSocket ??
      ((address, port, family) => net.connect({ host: address, port, family: family === 6 ? 6 : 4 }));
    this.server = http.createServer({ maxHeaderSize: 32 << 10 }, (req, res) => {
      void this.handleHTTP(req, res);
    });
    this.server.on('connect', (req, client, head) => {
      void this.handleConnect(req, client as Socket, head);
    });
    this.server.on('upgrade', (_req, socket) => failSocket(socket as Socket, 403));
    this.server.on('clientError', (_error, socket) => failSocket(socket as Socket, 400));
  }

  async start(): Promise<string> {
    if (this.address) return this.address;
    await new Promise<void>((resolve, reject) => {
      const onError = (error: Error) => {
        this.server.off('listening', onListening);
        reject(error);
      };
      const onListening = () => {
        this.server.off('error', onError);
        resolve();
      };
      this.server.once('error', onError);
      this.server.once('listening', onListening);
      this.server.listen(0, LOOPBACK_HOST);
    });
    const info = this.server.address() as AddressInfo | null;
    if (!info || info.address !== LOOPBACK_HOST) {
      await this.close();
      throw new Error('pinned proxy failed to bind loopback');
    }
    this.address = `http://${LOOPBACK_HOST}:${info.port}`;
    this.server.unref();
    return this.address;
  }

  async close(): Promise<void> {
    this.address = null;
    if (!this.server.listening) return;
    await new Promise<void>((resolve) => this.server.close(() => resolve()));
  }

  private async handleHTTP(req: IncomingMessage, res: ServerResponse): Promise<void> {
    try {
      const pinned = await resolvePinnedTarget(req.url ?? '', this.resolver, this.guard);
      const headers = forwardedHeaders(req.headers, pinned.url.host);
      const requestOptions: https.RequestOptions = {
        protocol: pinned.url.protocol,
        hostname: pinned.address.address,
        family: pinned.address.family === 6 ? 6 : 4,
        port: pinned.port,
        method: req.method,
        path: `${pinned.url.pathname}${pinned.url.search}`,
        headers,
        servername: pinned.url.hostname,
        timeout: this.socketTimeoutMs,
      };
      const requestFn = pinned.url.protocol === 'https:' ? https.request : http.request;
      const upstream = requestFn(requestOptions, (upstreamResponse) => {
        const responseHeaders = forwardedHeaders(upstreamResponse.headers, pinned.url.host);
        delete responseHeaders.host;
        res.writeHead(upstreamResponse.statusCode ?? 502, responseHeaders);
        upstreamResponse.pipe(res);
      });
      upstream.once('timeout', () => upstream.destroy(new Error('upstream timeout')));
      upstream.once('error', () => failHTTP(res, 502));
      req.once('aborted', () => upstream.destroy());
      req.pipe(upstream);
    } catch (error) {
      failHTTP(res, error instanceof UnsafeTargetURLError ? 403 : 502);
    }
  }

  private async handleConnect(req: IncomingMessage, client: Socket, head: Buffer): Promise<void> {
    try {
      const pinned = await resolvePinnedTarget(connectURL(req.url ?? ''), this.resolver, this.guard);
      const upstream = this.connectSocket(pinned.address.address, pinned.port, pinned.address.family);
      upstream.setTimeout(this.socketTimeoutMs, () => upstream.destroy(new Error('upstream timeout')));
      client.setTimeout(this.socketTimeoutMs, () => client.destroy());
      upstream.once('connect', () => {
        if (client.destroyed) {
          upstream.destroy();
          return;
        }
        client.write('HTTP/1.1 200 Connection Established\r\nConnection: keep-alive\r\n\r\n');
        if (head.length > 0) upstream.write(head);
        client.pipe(upstream);
        upstream.pipe(client);
      });
      upstream.once('error', () => failSocket(client, 502));
      client.once('error', () => upstream.destroy());
      client.once('close', () => upstream.destroy());
    } catch (error) {
      failSocket(client, error instanceof UnsafeTargetURLError ? 403 : 502);
    }
  }
}

let sharedProxyPromise: Promise<PinnedPublicProxy> | null = null;

async function sharedProxy(): Promise<PinnedPublicProxy> {
  if (!sharedProxyPromise) {
    sharedProxyPromise = (async () => {
      const proxy = new PinnedPublicProxy();
      await proxy.start();
      return proxy;
    })().catch((error) => {
      sharedProxyPromise = null;
      throw error;
    });
  }
  return sharedProxyPromise;
}

export async function getPinnedPublicProxyServer(): Promise<string> {
  return (await sharedProxy()).start();
}

export async function closePinnedPublicProxy(): Promise<void> {
  if (!sharedProxyPromise) return;
  const pending = sharedProxyPromise;
  sharedProxyPromise = null;
  await (await pending).close();
}
