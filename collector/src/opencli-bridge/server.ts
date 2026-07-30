import { timingSafeEqual } from 'node:crypto';
import { createServer, type IncomingMessage, type ServerResponse } from 'node:http';
import {
  collectViaOpenCli,
  OpenCliCollectError,
  probeOpenCliStatus,
  type OpenCliRuntimeStatus,
} from '../driver/opencli-driver.js';
import { validateTaobaoTmallUrl } from '../providers/sourceTaobaoTmall/validate-url.js';
import type { NormalizedProduct } from '../types/product.js';
import type { OpenCliBridgeConfig } from './config.js';

const MAX_BODY_BYTES = 1024 * 1024;

export type OpenCliBridgeDependencies = {
  collect: (
    source: string,
    url: string,
    options?: Record<string, unknown>,
  ) => Promise<NormalizedProduct>;
  status: () => Promise<OpenCliRuntimeStatus>;
};

const defaultDependencies: OpenCliBridgeDependencies = {
  collect: collectViaOpenCli,
  status: probeOpenCliStatus,
};

function json(res: ServerResponse, status: number, body: unknown): void {
  const payload = Buffer.from(JSON.stringify(body), 'utf8');
  res.writeHead(status, {
    'Content-Type': 'application/json; charset=utf-8',
    'Content-Length': payload.length,
    'Cache-Control': 'no-store',
  });
  res.end(payload);
}

async function readJsonBody(req: IncomingMessage): Promise<unknown> {
  const raw = await new Promise<string>((resolve, reject) => {
    const chunks: Buffer[] = [];
    let size = 0;
    let tooLarge = false;
    req.on('data', (chunk: Buffer | string) => {
      if (tooLarge) return;
      const buffer = Buffer.from(chunk);
      size += buffer.length;
      if (size > MAX_BODY_BYTES) {
        tooLarge = true;
        return;
      }
      chunks.push(buffer);
    });
    req.on('end', () => {
      if (tooLarge) {
        reject(new Error('body_too_large'));
        return;
      }
      resolve(Buffer.concat(chunks).toString('utf8'));
    });
    req.on('error', reject);
  });
  if (!raw.trim()) return {};
  try {
    return JSON.parse(raw) as unknown;
  } catch {
    throw new Error('invalid_json');
  }
}

function tokenMatches(req: IncomingMessage, expected: string): boolean {
  if (!expected) return true;
  const authorization = String(req.headers.authorization ?? '');
  const provided = authorization.startsWith('Bearer ')
    ? authorization.slice('Bearer '.length).trim()
    : '';
  const actualBuffer = Buffer.from(provided);
  const expectedBuffer = Buffer.from(expected);
  return (
    actualBuffer.length === expectedBuffer.length &&
    timingSafeEqual(actualBuffer, expectedBuffer)
  );
}

function errorResponse(error: unknown): {
  status: number;
  code: string;
  message: string;
} {
  if (error instanceof OpenCliCollectError) {
    const status =
      error.code === 'PROVIDER_NOT_AVAILABLE'
        ? 503
        : error.code === 'TIMEOUT'
          ? 504
          : 422;
    return { status, code: error.code, message: error.message };
  }
  const message = error instanceof Error ? error.message : String(error);
  return { status: 500, code: 'OPENCLI_BRIDGE_INTERNAL', message };
}

export function createOpenCliBridgeServer(
  config: OpenCliBridgeConfig,
  dependencies: OpenCliBridgeDependencies = defaultDependencies,
) {
  return createServer(async (req, res) => {
    const path = (req.url ?? '').split('?')[0];

    if (req.method === 'GET' && path === '/health') {
      json(res, 200, { ok: true, service: 'trademind-opencli-bridge' });
      return;
    }

    if (!tokenMatches(req, config.token)) {
      json(res, 401, {
        ok: false,
        error: { code: 'UNAUTHORIZED', message: 'Invalid OpenCLI Bridge token' },
      });
      return;
    }

    if (req.method === 'GET' && path === '/v1/opencli/status') {
      try {
        const status = await dependencies.status();
        json(res, 200, { ok: true, data: status });
      } catch (error) {
        const mapped = errorResponse(error);
        json(res, mapped.status, {
          ok: false,
          error: { code: mapped.code, message: mapped.message },
        });
      }
      return;
    }

    if (req.method === 'POST' && path === '/v1/collect') {
      let rawBody: unknown;
      try {
        rawBody = await readJsonBody(req);
      } catch (error) {
        const code = error instanceof Error && error.message === 'body_too_large'
          ? 'BODY_TOO_LARGE'
          : 'INVALID_REQUEST';
        json(res, code === 'BODY_TOO_LARGE' ? 413 : 400, {
          ok: false,
          error: { code, message: 'Request body must be valid JSON under 1 MiB' },
        });
        return;
      }

      if (!rawBody || typeof rawBody !== 'object' || Array.isArray(rawBody)) {
        json(res, 400, {
          ok: false,
          error: {
            code: 'INVALID_REQUEST',
            message: 'Request body must be a JSON object',
          },
        });
        return;
      }

      const body = rawBody as {
        source?: string;
        url?: string;
        options?: Record<string, unknown>;
      };
      const source = String(body.source ?? '').trim().toLowerCase();
      const url = String(body.url ?? '').trim();
      if (source !== 'taobao_tmall' || !validateTaobaoTmallUrl(url)) {
        json(res, 400, {
          ok: false,
          error: {
            code: 'OPENCLI_SOURCE_UNSUPPORTED',
            message: 'OpenCLI Bridge supports only taobao_tmall product-detail URLs',
          },
        });
        return;
      }

      try {
        const product = await dependencies.collect(source, url, body.options ?? {});
        json(res, 200, { ok: true, data: { product } });
      } catch (error) {
        const mapped = errorResponse(error);
        json(res, mapped.status, {
          ok: false,
          error: { code: mapped.code, message: mapped.message },
        });
      }
      return;
    }

    json(res, 404, {
      ok: false,
      error: { code: 'NOT_FOUND', message: 'Not found' },
    });
  });
}

export function listenOpenCliBridge(
  config: OpenCliBridgeConfig,
  dependencies: OpenCliBridgeDependencies = defaultDependencies,
) {
  const server = createOpenCliBridgeServer(config, dependencies);
  server.listen(config.port, config.host, () => {
    console.info(
      `[opencli-bridge] listening on http://${config.host}:${config.port}`,
    );
  });
  return server;
}
