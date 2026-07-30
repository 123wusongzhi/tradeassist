import fs from 'node:fs';
import path from 'node:path';
import { test, expect, type Page, type APIRequestContext } from '@playwright/test';

const enabled = process.env.P8_REAL_BACKEND_E2E === '1';
const apiBase = (process.env.TRADEMIND_API_BASE || 'http://127.0.0.1:8080').replace(/\/$/, '');

type LoginResult = {
  code: number;
  data?: { token?: string };
};

function loadRootEnv() {
  const envPath = path.resolve(__dirname, '../../../.env');
  if (!fs.existsSync(envPath)) return;
  for (const line of fs.readFileSync(envPath, 'utf8').split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) continue;
    const idx = trimmed.indexOf('=');
    if (idx <= 0) continue;
    const key = trimmed.slice(0, idx).trim();
    if (process.env[key]) continue;
    process.env[key] = trimmed.slice(idx + 1).trim().replace(/^['"]|['"]$/g, '');
  }
}

async function login(request: APIRequestContext) {
  loadRootEnv();
  const account = process.env.TRADEMIND_ADMIN_ACCOUNT || process.env.ADMIN_BOOTSTRAP_EMAIL;
  const password = process.env.TRADEMIND_ADMIN_PASSWORD || process.env.ADMIN_BOOTSTRAP_PASSWORD;
  if (!account || !password) throw new Error('missing_admin_credentials');
  const res = await request.post(`${apiBase}/api/v1/auth/login`, { data: { account, password } });
  expect(res.ok()).toBeTruthy();
  const body = (await res.json()) as LoginResult;
  expect(body.code).toBe(0);
  expect(body.data?.token).toBeTruthy();
  return body.data!.token!;
}

async function proxyBackendAPI(page: Page) {
  await page.route('**/api/v1/**', async (route) => {
    const req = route.request();
    const original = new URL(req.url());
    const headers = await req.allHeaders();
    delete headers.host;
    const response = await route.fetch({
      url: `${apiBase}${original.pathname}${original.search}`,
      headers,
    });
    await route.fulfill({ response });
  });
}

test.describe('@p8-real-backend operation task authenticated E2E', () => {
  test.describe.configure({ timeout: 120_000 });
  test.skip(!enabled, 'Set P8_REAL_BACKEND_E2E=1 to run against a local real backend.');

  test('real backend requires auth for operation task API', async ({ request }) => {
    const res = await request.get(`${apiBase}/api/v1/operation-tasks`);
    expect([401, 403]).toContain(res.status());
  });

  test('unauthenticated operation task route redirects to login and preserves target', async ({ page }) => {
    await proxyBackendAPI(page);
    await page.goto('/ops/task-center/operation-tasks');
    await expect(page).toHaveURL(/\/user\/login\?redirect=/);
    expect(new URL(page.url()).searchParams.get('redirect')).toBe('/ops/task-center/operation-tasks');
  });

  test('real backend Bearer token renders operation task center', async ({ page, request }) => {
    const token = await login(request);
    await proxyBackendAPI(page);
    await page.addInitScript((authToken) => {
      window.localStorage.setItem('trademind_admin_token', authToken);
    }, token);
    await page.goto('/ops/task-center/operation-tasks');
    await expect(page).not.toHaveURL(/\/user\/login/);
    await expect(page.locator('#root')).toBeVisible();
    await expect(page.getByText(/运营任务|任务中心|Operation Task/).first()).toBeVisible();
  });
});
