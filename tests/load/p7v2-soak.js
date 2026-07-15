import { check, sleep } from 'k6';
import http from 'k6/http';
import { failFastHost, baseUrl } from './lib/guards.js';
import {
  ROLE_OPERATOR,
  ROLE_READONLY,
  ROLE_SYSTEM_ADMIN,
  ROLE_TENANT_ADMIN,
  routeCredentialMatrix,
} from './lib/credentials.js';
import { authHeadersForRole, loginAllRoles, refreshRoleIfNeeded } from './lib/auth.js';
import { classifyResponse, recordClassification, authLoginFailures } from './lib/classify.js';
import { postSignedWebhook } from './lib/webhook.js';
import {
  createFormalRouteMetrics,
  formalThresholds,
  metricTags,
  recordFormalRouteMetric,
} from './lib/formal-metrics.js';

failFastHost();

const targetVUs = Number(__ENV.TARGET_VUS || 8);
const warmupDur = __ENV.WARMUP || '5m';
const steadyDur = __ENV.STEADY || '30m';
const rampdownDur = __ENV.RAMPDOWN || '2m';
const formalMetrics = createFormalRouteMetrics();
let cachedTokens;

export const options = {
  summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(90)', 'p(95)', 'p(99)'],
  scenarios: {
    warmup: {
      executor: 'constant-vus',
      vus: Math.max(2, Math.floor(targetVUs * 0.5)),
      duration: warmupDur,
      exec: 'warmupPhase',
    },
    steady: {
      executor: 'constant-vus',
      vus: targetVUs,
      duration: steadyDur,
      startTime: warmupDur,
      exec: 'steadyPhase',
    },
    rampdown: {
      executor: 'ramping-vus',
      startVUs: targetVUs,
      stages: [{ duration: rampdownDur, target: 0 }],
      startTime: addDuration(warmupDur, steadyDur),
      exec: 'steadyPhase',
    },
  },
  thresholds: formalThresholds,
};

export function setup() {
  const tokens = loginAllRoles();
  for (const role of [ROLE_SYSTEM_ADMIN, ROLE_TENANT_ADMIN, ROLE_OPERATOR, ROLE_READONLY]) {
    if (!tokens[role]) {
      authLoginFailures.add(1);
      throw new Error(`setup failed: missing token for ${role}`);
    }
  }
  return {};
}

export function warmupPhase() {
  const res = readList('productList', tokensFor(), '');
  check(res, { 'soak warmup bounded': (r) => r.status < 500 });
  sleep(0.5);
}

export function steadyPhase() {
  const tokens = tokensFor();
  refreshRoleIfNeeded(tokens);
  const pick = __ITER % 12;
  let metricId;
  let res;
  if (pick < 2) {
    metricId = 'productList';
    res = readList(metricId, tokens, '');
  } else if (pick < 4) {
    metricId = 'orderList';
    res = readList(metricId, tokens, '');
  } else if (pick < 5) {
    metricId = 'inventoryList';
    res = readList(metricId, tokens, '');
  } else if (pick < 6) {
    metricId = 'taskList';
    res = readList(metricId, tokens, '');
  } else if (pick < 7) {
    metricId = 'webhookEventList';
    res = readList(metricId, tokens, '');
  } else if (pick < 8) {
    metricId = 'operationLogList';
    res = readList(metricId, tokens, '');
  } else if (pick === 8) {
    metricId = 'webhookIngestion';
    res = webhookValidScenario();
  } else if (pick === 9) {
    metricId = 'providerMockFlow';
    res = providerHealthScenario();
  } else if (pick === 10) {
    metricId = 'authInvalidLogin';
    res = authInvalidLoginScenario();
  } else {
    metricId = 'webhookInvalidSignature';
    res = webhookInvalidScenario();
  }
  const expectedStatusClass = res.status >= 400 ? '4xx' : '2xx';
  recordFormalRouteMetric(formalMetrics, metricId, res?.timings?.duration || 0, 'steady', expectedStatusClass);
  const route = routeForMetric(metricId);
  const cls = classifyResponse(res, route, route.securityNegative ? 'security' : 'steady');
  recordClassification(cls);
  check(res, { 'soak steady bounded': () => cls.unexpected === false });
  sleep(0.4);
}

function readList(metricId, tokens, cursor) {
  const route = routeForMetric(metricId);
  const path = route?.path || routeCredentialMatrix.productList.path;
  const query = cursor ? `${path}${path.includes('?') ? '&' : '?'}cursor=${encodeURIComponent(cursor)}` : path;
  return http.get(`${baseUrl()}${query}`, {
    headers: authHeadersForRole(tokens, route.credentialRole),
    tags: { ...metricTags(metricId, 'steady', '2xx'), group: 'read-heavy', credential: route.credentialRole },
  });
}

function webhookValidScenario() {
  const eventId = `p7v2-soak-valid-${__VU}-${__ITER}`;
  return postSignedWebhook('/api/v1/webhooks/internal-test/ping', { eventId }, {
    ...metricTags('webhookIngestion', 'steady', '2xx'),
    group: 'mixed',
  });
}

function authInvalidLoginScenario() {
  return http.post(
    `${baseUrl()}/api/v1/auth/login`,
    JSON.stringify({ account: 'p7v2-invalid@example.invalid', password: 'wrong-password' }),
    {
      headers: { 'Content-Type': 'application/json' },
      tags: { ...metricTags('authInvalidLogin', 'steady', '4xx'), group: 'auth-security' },
      responseCallback: http.expectedStatuses(401),
    },
  );
}

function webhookInvalidScenario() {
  const body = JSON.stringify({ eventId: `p7v2-soak-bad-${__VU}-${__ITER}` });
  const ts = Math.floor(Date.now() / 1000);
  return http.post(`${baseUrl()}/api/v1/webhooks/internal-test/ping`, body, {
    headers: {
      'Content-Type': 'application/json',
      'X-Webhook-Signature': 'deadbeef',
      'X-Webhook-Timestamp': String(ts),
    },
    tags: { ...metricTags('webhookInvalidSignature', 'steady', '4xx'), group: 'auth-security' },
    responseCallback: http.expectedStatuses(401, 403),
  });
}

function providerHealthScenario() {
  return http.get(`${baseUrl()}/health/live`, { tags: { ...metricTags('providerMockFlow', 'steady', '2xx'), group: 'mixed' } });
}

function routeForMetric(metricId) {
  if (metricId === 'webhookIngestion') return routeCredentialMatrix.webhookValidIngestion;
  if (metricId === 'providerMockFlow') return routeCredentialMatrix.healthLive;
  if (metricId === 'authInvalidLogin') return routeCredentialMatrix.authInvalidLogin;
  if (metricId === 'webhookInvalidSignature') return routeCredentialMatrix.webhookInvalidSignature;
  return routeCredentialMatrix[metricId] || routeCredentialMatrix.productList;
}

function tokensFor() {
  if (!cachedTokens) cachedTokens = loginAllRoles();
  return cachedTokens;
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
