import { check, sleep } from 'k6';
import http from 'k6/http';
import crypto from 'k6/crypto';
import { failFastHost, baseUrl } from './lib/guards.js';
import {
  ROLE_SYSTEM_ADMIN,
  ROLE_TENANT_ADMIN,
  ROLE_OPERATOR,
  ROLE_READONLY,
  routeCredentialMatrix,
  webhookTestSecret,
} from './lib/credentials.js';
import {
  classifyResponse,
  recordClassification,
  authLoginFailures,
} from './lib/classify.js';
import { loginAllRoles, authHeadersForRole, refreshRoleIfNeeded } from './lib/auth.js';
import { postSignedWebhook } from './lib/webhook.js';

failFastHost();

export const thresholds = {
  http_req_failed: ['rate<0.01'],
  unexpected_401: ['count==0'],
  unexpected_403: ['count==0'],
  unexpected_404: ['count==0'],
  unexpected_5xx: ['rate<0.002'],
  auth_login_failures: ['count==0'],
  'http_req_duration{group:read-heavy}': ['p(95)<800', 'p(99)<1500'],
  'http_req_duration{group:mixed}': ['p(95)<1200', 'p(99)<2500'],
  'http_req_duration{group:auth}': ['p(95)<500', 'p(99)<1000'],
};

const targetVUs = Number(__ENV.TARGET_VUS || 10);
const warmupDur = __ENV.WARMUP || '5m';
const rampDur = __ENV.RAMP || '3m';
const steadyDur = __ENV.STEADY || '10m';
const rampdownDur = __ENV.RAMPDOWN || '2m';

export const options = {
  scenarios: {
    warmup: {
      executor: 'constant-vus',
      vus: Math.max(2, Math.floor(targetVUs * 0.3)),
      duration: warmupDur,
      startTime: '0s',
      exec: 'warmupPhase',
    },
    ramp: {
      executor: 'ramping-vus',
      startVUs: Math.max(2, Math.floor(targetVUs * 0.3)),
      stages: [{ duration: rampDur, target: targetVUs }],
      startTime: warmupDur,
      exec: 'steadyPhase',
    },
    steady: {
      executor: 'constant-vus',
      vus: targetVUs,
      duration: steadyDur,
      startTime: addDuration(warmupDur, rampDur),
      exec: 'steadyPhase',
    },
    rampdown: {
      executor: 'ramping-vus',
      startVUs: targetVUs,
      stages: [{ duration: rampdownDur, target: 0 }],
      startTime: addDuration(warmupDur, rampDur, steadyDur),
      exec: 'steadyPhase',
    },
    security_negative: {
      executor: 'constant-vus',
      vus: 1,
      duration: addDuration(warmupDur, rampDur, steadyDur, rampdownDur),
      startTime: '0s',
      exec: 'securityNegativePhase',
    },
  },
  thresholds,
};

export function setup() {
  const tokens = loginAllRoles();
  const required = [ROLE_SYSTEM_ADMIN, ROLE_TENANT_ADMIN, ROLE_OPERATOR, ROLE_READONLY];
  for (const role of required) {
    if (!tokens[role]) {
      authLoginFailures.add(1);
      throw new Error(`setup failed: missing token for ${role}`);
    }
  }
  const probe = http.get(`${baseUrl()}${routeCredentialMatrix.productList.path}`, {
    headers: authHeadersForRole(tokens, ROLE_TENANT_ADMIN),
    tags: { scenario: 'setup-probe', group: 'read-heavy' },
  });
  if (probe.status !== 200) {
    authLoginFailures.add(1);
    throw new Error(`setup probe failed: product list status=${probe.status}`);
  }
  return { tokens };
}

export function warmupPhase(data) {
  const res = readList('productList', data?.tokens, '');
  check(res, { 'warmup ok': (r) => r.status === 200 });
  sleep(0.5);
}

export function steadyPhase(data) {
  refreshRoleIfNeeded(data?.tokens);
  const pick = __ITER % 10;
  let res;
  if (pick < 2) res = readList('productList', data?.tokens, '');
  else if (pick < 4) res = readList('orderList', data?.tokens, '');
  else if (pick < 5) res = readList('inventoryList', data?.tokens, '');
  else if (pick < 6) res = readList('taskList', data?.tokens, '');
  else if (pick < 7) res = readList('webhookEventList', data?.tokens, '');
  else if (pick < 8) res = readList('operationLogList', data?.tokens, '');
  else if (pick === 8) res = webhookValidScenario();
  else res = providerHealthScenario();
  const route = routeForPick(pick);
  const cls = classifyResponse(res, route, 'steady');
  recordClassification(cls);
  check(res, { 'steady success': (r) => cls.unexpected === false });
  sleep(0.3);
}

export function securityNegativePhase() {
  const invalidLogin = http.post(
    `${baseUrl()}/api/v1/auth/login`,
    JSON.stringify({ account: 'p7v2-invalid@example.invalid', password: 'wrong-password' }),
    {
      headers: { 'Content-Type': 'application/json' },
      tags: { scenario: 'auth-invalid-login', group: 'auth-security' },
      responseCallback: http.expectedStatuses(401),
    },
  );
  const cls1 = classifyResponse(invalidLogin, { scenario: 'auth-invalid-login', credentialRole: 'none', expectedStatus: 401 }, 'security');
  recordClassification(cls1);

  const invalidWebhook = webhookInvalidScenario();
  const cls2 = classifyResponse(invalidWebhook, { scenario: 'webhook-invalid-signature', credentialRole: 'none', expectedStatus: 401 }, 'security');
  recordClassification(cls2);

  sleep(1);
}

export default function (data) {
  steadyPhase(data);
}

function readList(name, tokens, cursor) {
  const route = routeCredentialMatrix[name];
  const path = route?.path || routeCredentialMatrix.productList.path;
  const query = cursor ? `${path}${path.includes('?') ? '&' : '?'}cursor=${encodeURIComponent(cursor)}` : path;
  return http.get(`${baseUrl()}${query}`, {
    headers: authHeadersForRole(tokens, route.credentialRole),
    tags: { scenario: name, group: 'read-heavy', credential: route.credentialRole },
  });
}

function webhookValidScenario() {
  const eventId = `p7v2-valid-${__VU}-${__ITER}`;
  return postSignedWebhook('/api/v1/webhooks/internal-test/ping', { eventId }, {
    scenario: 'webhook-valid-ingestion',
    group: 'mixed',
  });
}

function webhookInvalidScenario() {
  const body = JSON.stringify({ eventId: `p7v2-bad-${__VU}-${__ITER}` });
  const ts = Math.floor(Date.now() / 1000);
  return http.post(`${baseUrl()}/api/v1/webhooks/internal-test/ping`, body, {
    headers: {
      'Content-Type': 'application/json',
      'X-Webhook-Signature': 'deadbeef',
      'X-Webhook-Timestamp': String(ts),
    },
    tags: { scenario: 'webhook-invalid-signature', group: 'auth-security' },
    responseCallback: http.expectedStatuses(401, 403),
  });
}

function providerHealthScenario() {
  return http.get(`${baseUrl()}/health/live`, { tags: { scenario: 'provider-mock-flow', group: 'mixed' } });
}

function routeForPick(pick) {
  if (pick < 2) return routeCredentialMatrix.productList;
  if (pick < 4) return routeCredentialMatrix.orderList;
  if (pick < 5) return routeCredentialMatrix.inventoryList;
  if (pick < 6) return routeCredentialMatrix.taskList;
  if (pick < 7) return routeCredentialMatrix.webhookEventList;
  if (pick < 8) return routeCredentialMatrix.operationLogList;
  if (pick === 8) return { scenario: 'webhook-valid-ingestion', credentialRole: 'none', expectedStatus: 200 };
  return { scenario: 'provider-mock-flow', credentialRole: 'none', expectedStatus: 200 };
}

function addDuration(...parts) {
  let total = 0;
  for (const part of parts) {
    const m = String(part).match(/^(\d+)(s|m|h)$/);
    if (!m) continue;
    const n = Number(m[1]);
    total += m[2] === 'h' ? n * 3600 : m[2] === 'm' ? n * 60 : n;
  }
  return `${total}s`;
}
