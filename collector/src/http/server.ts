import { timingSafeEqual } from 'node:crypto';
import { createServer, type IncomingMessage, type ServerResponse } from 'node:http';
import { BrowserManager } from '../browser/manager.js';
import { assertProviderProfileKey, isLegacyProviderProfileKey } from '../browser/provider-profile-key.js';
import { getCollectorHttpConfig } from '../config/env.js';
import { listRegisteredSources, listProviderPublicMetas } from '../providers/registry.js';
import { runCustomRuleTest } from '../providers/sourceCustom/index.js';
import { analyzeCustomPage } from '../providers/sourceCustom/analyze-page.js';
import type { CustomCollectOptions } from '../providers/sourceCustom/types.js';
import { assertPublicHttpURL, UnsafeTargetURLError } from '../security/public-url.js';
import { runCollectTask } from '../tasks/collect-task.js';

const DEFAULT_MAX_BODY_BYTES = 1024 * 1024;

type CollectorServerOptions = {
  token?: string;
  maxBodyBytes?: number;
};

class RequestBodyError extends Error {
  constructor(readonly code: 'invalid_json' | 'body_too_large') {
    super(code);
  }
}

function json(res: ServerResponse, status: number, body: unknown): void {
  const buf = Buffer.from(JSON.stringify(body), 'utf8');
  res.writeHead(status, {
    'Content-Type': 'application/json; charset=utf-8',
    'Content-Length': buf.length,
  });
  res.end(buf);
}

function matchBrowserProfileRoute(
  method: string,
  url: string,
): { profileKey: string; action: 'open-login' | 'check' } | null {
  if (method !== 'POST') return null;
  const path = url.split('?')[0] ?? '';
  const m = path.match(/^\/v1\/browser-profiles\/([^/]+)\/(open-login|check)$/);
  if (!m) return null;
  return { profileKey: decodeURIComponent(m[1]), action: m[2] as 'open-login' | 'check' };
}

function hasValidBearerToken(req: IncomingMessage, expectedToken: string): boolean {
  if (!expectedToken) return true;
  const header = String(req.headers.authorization ?? '');
  if (!header.startsWith('Bearer ')) return false;
  const actual = Buffer.from(header.slice('Bearer '.length), 'utf8');
  const expected = Buffer.from(expectedToken, 'utf8');
  return actual.length === expected.length && timingSafeEqual(actual, expected);
}

function providerProfileKey(provider: string, raw: unknown): string {
  return assertProviderProfileKey(provider, String(raw ?? provider));
}

function writeBodyError(res: ServerResponse, error: unknown): void {
  if (error instanceof RequestBodyError && error.code === 'body_too_large') {
    json(res, 413, {
      ok: false,
      error: { code: 'PAYLOAD_TOO_LARGE', message: 'request body is too large' },
    });
    return;
  }
  json(res, 400, {
    ok: false,
    error: { code: 'INVALID_REQUEST', message: 'body must be valid JSON' },
  });
}

function writeTargetError(res: ServerResponse, error: unknown): boolean {
  if (!(error instanceof UnsafeTargetURLError)) return false;
  json(res, 422, {
    ok: false,
    error: { code: error.code, message: 'target URL is not publicly routable' },
  });
  return true;
}

async function readJsonBody(req: IncomingMessage, maxBodyBytes: number): Promise<unknown> {
  const contentLength = Number(req.headers['content-length'] ?? 0);
  if (Number.isFinite(contentLength) && contentLength > maxBodyBytes) {
    req.resume();
    throw new RequestBodyError('body_too_large');
  }
  const raw = await new Promise<string>((resolve, reject) => {
    const chunks: Buffer[] = [];
    let size = 0;
    let tooLarge = false;
    req.on('data', (c) => {
      if (tooLarge) return;
      const chunk = Buffer.from(c);
      size += chunk.length;
      if (size > maxBodyBytes) {
        tooLarge = true;
        chunks.length = 0;
        return;
      }
      chunks.push(chunk);
    });
    req.on('end', () => {
      if (tooLarge) {
        reject(new RequestBodyError('body_too_large'));
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
    throw new RequestBodyError('invalid_json');
  }
}

/**
 * HTTP 任务入口：POST /v1/collect
 * body: { "source": "1688", "url": "https://..." }
 */
export function createCollectorServer(
  browser: BrowserManager,
  options: CollectorServerOptions = {},
) {
  const expectedToken = options.token?.trim() ?? '';
  const maxBodyBytes = options.maxBodyBytes ?? DEFAULT_MAX_BODY_BYTES;
  if (!Number.isSafeInteger(maxBodyBytes) || maxBodyBytes <= 0) {
    throw new Error('maxBodyBytes must be a positive integer');
  }
  return createServer(async (req, res) => {
    try {
      if (req.method === 'GET' && req.url === '/health') {
        json(res, 200, {
          ok: true,
          service: 'trademind-collector',
          sources: listRegisteredSources(),
        });
        return;
      }

      if ((req.url ?? '').startsWith('/v1/') && !hasValidBearerToken(req, expectedToken)) {
        json(res, 401, {
          ok: false,
          error: { code: 'UNAUTHORIZED', message: 'valid internal bearer token is required' },
        });
        return;
      }

      if (req.method === 'GET' && req.url === '/v1/providers') {
        json(res, 200, { ok: true, data: listProviderPublicMetas() });
        return;
      }

      if (req.method === 'GET' && (req.url === '/v1/providers/1688/auth-status' || req.url?.startsWith('/v1/providers/1688/auth-status?'))) {
        const q = new URL(req.url ?? '', 'http://local').searchParams;
        const profileKey = providerProfileKey('1688', q.get('profileKey'));
        const status = isLegacyProviderProfileKey('1688', profileKey)
          ? await browser.sessions.check1688AuthStatus()
          : await browser.customProfiles.checkProfileAccess(profileKey, 'https://www.1688.com/').then((out) => ({ provider: '1688', profileKey, status: out.accessStatus, loggedIn: out.accessStatus === 'public', needVerification: out.accessStatus === 'verify_required', message: out.message, lastCheckedAt: new Date().toISOString() }));
        json(res, 200, { ok: true, data: status });
        return;
      }

      if (req.method === 'POST' && req.url === '/v1/providers/1688/open-login-browser') {
        const body = await readJsonBody(req, maxBodyBytes) as { profileKey?: string };
        const profileKey = providerProfileKey('1688', body.profileKey);
        const result = isLegacyProviderProfileKey('1688', profileKey)
          ? await browser.sessions.openLoginBrowser('1688')
          : await browser.customProfiles.openLoginBrowser(profileKey, 'https://www.1688.com/');
        json(res, 200, { ok: true, data: result });
        return;
      }

      if (req.method === 'GET' && (req.url === '/v1/providers/pinduoduo/auth-status' || req.url?.startsWith('/v1/providers/pinduoduo/auth-status?'))) {
        const raw = req.url ?? '';
        const q = raw.includes('?') ? new URL(raw, 'http://local').searchParams : null;
        const checkUrl = q?.get('url')?.trim() || undefined;
        const testUrl = q?.get('testUrl')?.trim() || undefined;
        await Promise.all(
          [checkUrl, testUrl]
            .filter((value): value is string => Boolean(value))
            .map((value) => assertPublicHttpURL(value)),
        );
        const status = await browser.sessions.checkPinduoduoAuthStatus(checkUrl, testUrl);
        json(res, 200, { ok: true, data: status });
        return;
      }

      if (req.method === 'POST' && req.url === '/v1/providers/pinduoduo/check-login') {
        let body: unknown = {};
        try {
          body = await readJsonBody(req, maxBodyBytes);
        } catch (error) {
          writeBodyError(res, error);
          return;
        }
        const b = body as { profileKey?: string; url?: string; testUrl?: string };
        const profileKey = providerProfileKey('pinduoduo', b.profileKey);
        const checkUrl = String(b.url ?? '').trim() || undefined;
        const testUrl = String(b.testUrl ?? '').trim() || undefined;
        await Promise.all(
          [checkUrl, testUrl]
            .filter((value): value is string => Boolean(value))
            .map((value) => assertPublicHttpURL(value)),
        );
        const status = isLegacyProviderProfileKey('pinduoduo', profileKey)
          ? await browser.sessions.checkPinduoduoAuthStatus(checkUrl, testUrl)
          : await browser.customProfiles.checkProfileAccess(profileKey, checkUrl || testUrl || 'https://pifa.pinduoduo.com/').then((out) => ({ provider: 'pinduoduo', profileKey, status: out.accessStatus, loginStatus: out.accessStatus, loggedIn: out.accessStatus === 'public', needVerification: out.accessStatus === 'verify_required', message: out.message, lastCheckedAt: new Date().toISOString(), checkedUrl: checkUrl || testUrl || '', finalUrl: out.finalUrl, accessStatus: out.accessStatus, urlType: 'unknown', evidence: {} }));
        json(res, 200, { ok: true, data: status });
        return;
      }

      if (req.method === 'POST' && req.url === '/v1/providers/pinduoduo/open-login-browser') {
        let body: unknown = {};
        try {
          body = await readJsonBody(req, maxBodyBytes);
        } catch (error) {
          writeBodyError(res, error);
          return;
        }
        const b = body as { profileKey?: string; url?: string };
        const profileKey = providerProfileKey('pinduoduo', b.profileKey);
        const loginUrl = String(b.url ?? '').trim();
        if (loginUrl) await assertPublicHttpURL(loginUrl);
        const result = isLegacyProviderProfileKey('pinduoduo', profileKey)
          ? await browser.sessions.openPinduoduoLoginBrowser(loginUrl || undefined)
          : await browser.customProfiles.openLoginBrowser(profileKey, loginUrl || 'https://pifa.pinduoduo.com/');
        json(res, 200, { ok: true, data: result });
        return;
      }

      if (req.method === 'POST' && req.url === '/v1/providers/taobao_tmall/check-login') {
        let body: unknown = {};
        try {
          body = await readJsonBody(req, maxBodyBytes);
        } catch (error) {
          writeBodyError(res, error);
          return;
        }
        const b = body as { profileKey?: string; url?: string; testUrl?: string };
        const profileKey = providerProfileKey('taobao_tmall', b.profileKey);
        const checkUrl = String(b.url ?? '').trim()
          || String(b.testUrl ?? '').trim()
          || undefined;
        if (checkUrl) await assertPublicHttpURL(checkUrl);
        const status = isLegacyProviderProfileKey('taobao_tmall', profileKey)
          ? await browser.sessions.checkTaobaoTmallAuthStatus(checkUrl)
          : await browser.customProfiles.checkProfileAccess(profileKey, checkUrl || 'https://www.taobao.com/').then((out) => ({ provider: 'taobao_tmall', profileKey, status: out.accessStatus, loginStatus: out.accessStatus, loggedIn: out.accessStatus === 'public', needVerification: out.accessStatus === 'verify_required', message: out.message, lastCheckedAt: new Date().toISOString(), checkedUrl: checkUrl || '', finalUrl: out.finalUrl }));
        json(res, 200, { ok: true, data: status });
        return;
      }

      if (req.method === 'POST' && req.url === '/v1/providers/taobao_tmall/open-login-browser') {
        let body: unknown = {};
        try {
          body = await readJsonBody(req, maxBodyBytes);
        } catch (error) {
          writeBodyError(res, error);
          return;
        }
        const b = body as { profileKey?: string; url?: string };
        const profileKey = providerProfileKey('taobao_tmall', b.profileKey);
        const loginUrl = String(b.url ?? '').trim();
        if (loginUrl) await assertPublicHttpURL(loginUrl);
        const result = isLegacyProviderProfileKey('taobao_tmall', profileKey)
          ? await browser.sessions.openTaobaoTmallLoginBrowser(loginUrl || undefined)
          : await browser.customProfiles.openLoginBrowser(profileKey, loginUrl || 'https://www.taobao.com/');
        json(res, 200, { ok: true, data: result });
        return;
      }

      const profileRoute = matchBrowserProfileRoute(req.method ?? '', req.url ?? '');
      if (profileRoute) {
        let body: unknown = {};
        try {
          body = await readJsonBody(req, maxBodyBytes);
        } catch (error) {
          writeBodyError(res, error);
          return;
        }
        const url = String((body as { url?: string }).url ?? '').trim();
        if (!url) {
          json(res, 400, {
            ok: false,
            error: { code: 'INVALID_REQUEST', message: 'url is required' },
          });
          return;
        }
        try {
          await assertPublicHttpURL(url);
          if (profileRoute.action === 'open-login') {
            const data = await browser.customProfiles.openLoginBrowser(profileRoute.profileKey, url);
            json(res, 200, { ok: true, data });
            return;
          }
          const data = await browser.customProfiles.checkProfileAccess(profileRoute.profileKey, url);
          json(res, 200, { ok: true, data });
        } catch (e) {
          if (writeTargetError(res, e)) return;
          const message = e instanceof Error ? e.message : String(e);
          const code = message.startsWith('HEADED_BROWSER_REQUIRED')
            ? 'HEADED_BROWSER_REQUIRED'
            : message.startsWith('INVALID_PROFILE_KEY')
              ? 'INVALID_PROFILE_KEY'
              : 'INTERNAL';
          const status = code === 'HEADED_BROWSER_REQUIRED' ? 422 : code === 'INVALID_PROFILE_KEY' ? 400 : 500;
          json(res, status, { ok: false, error: { code, message } });
        }
        return;
      }

      if (req.method === 'POST' && req.url === '/v1/custom/analyze-page') {
        let body: unknown;
        try {
          body = await readJsonBody(req, maxBodyBytes);
        } catch (error) {
          writeBodyError(res, error);
          return;
        }
        const b = body as {
          url?: string;
          profileKey?: string;
          useBrowserProfile?: boolean;
          maxCandidates?: number;
        };
        const url = b.url?.trim() ?? '';
        if (!url) {
          json(res, 400, {
            ok: false,
            error: { code: 'INVALID_REQUEST', message: 'url is required' },
          });
          return;
        }
        try {
          const digest = await analyzeCustomPage(browser, url, {
            profileKey: b.profileKey,
            useBrowserProfile: b.useBrowserProfile,
            maxCandidates: b.maxCandidates,
          });
          json(res, 200, { ok: true, data: digest });
        } catch (e) {
          if (writeTargetError(res, e)) return;
          const message = e instanceof Error ? e.message : String(e);
          json(res, 500, { ok: false, error: { code: 'INTERNAL', message } });
        }
        return;
      }

      if (req.method === 'POST' && req.url === '/v1/collect/custom-rule-test') {
        let body: unknown;
        try {
          body = await readJsonBody(req, maxBodyBytes);
        } catch (error) {
          writeBodyError(res, error);
          return;
        }
        const b = body as { url?: string; options?: CustomCollectOptions };
        const url = b.url?.trim() ?? '';
        const opts = b.options;
        if (!url || !opts?.rule) {
          json(res, 400, {
            ok: false,
            error: { code: 'INVALID_REQUEST', message: 'url and options.rule are required' },
          });
          return;
        }
        try {
          const result = await runCustomRuleTest(browser, url, opts);
          json(res, 200, {
            ok: true,
            data: {
              accessStatus: result.report.accessStatus,
              finalUrl: result.report.finalUrl,
              httpStatus: result.report.httpStatus,
              extractedFields: result.report.extractedFields,
              missingFields: result.report.missingFields,
              warnings: result.report.warnings,
              qualityScore: result.report.qualityScore,
              errorCode: result.report.errorCode,
              suggestion: result.report.suggestion,
              product: result.product ?? null,
            },
          });
        } catch (e) {
          if (writeTargetError(res, e)) return;
          const message = e instanceof Error ? e.message : String(e);
          json(res, 500, { ok: false, error: { code: 'INTERNAL', message } });
        }
        return;
      }

      if (req.method === 'POST' && req.url === '/v1/collect') {
        let body: unknown;
        try {
          body = await readJsonBody(req, maxBodyBytes);
        } catch (error) {
          writeBodyError(res, error);
          return;
        }
        const b = body as { source?: string; url?: string; options?: Record<string, unknown> };
        if (b.url?.trim()) await assertPublicHttpURL(b.url.trim());
        const result = await runCollectTask(
          { source: b.source ?? '', url: b.url ?? '', options: b.options },
          browser,
        );
        if (result.status === 'success') {
          json(res, 200, { ok: true, data: { product: result.product } });
        } else {
          json(res, 422, {
            ok: false,
            error: result.error,
            data: result.access ? { accessReport: result.access } : undefined,
          });
        }
        return;
      }

      json(res, 404, {
        ok: false,
        error: { code: 'NOT_FOUND' as const, message: String(req.url ?? '') },
      });
    } catch (e) {
      if (writeTargetError(res, e)) return;
      const message = e instanceof Error ? e.message : String(e);
      json(res, 500, { ok: false, error: { code: 'INTERNAL', message } });
    }
  });
}

export function listenCollectorHttp(browser: BrowserManager): ReturnType<typeof createServer> {
  const config = getCollectorHttpConfig();
  const server = createCollectorServer(browser, { token: config.token });
  server.listen(config.port, config.host, () => {
    console.info(
      `[collector] listening on ${config.host}:${config.port} (POST /v1/collect, GET /v1/providers/1688|pinduoduo/auth-status, POST .../open-login-browser, GET /health)`,
    );
  });
  return server;
}
