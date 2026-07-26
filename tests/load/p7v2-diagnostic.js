import { check, sleep } from 'k6';
import http from 'k6/http';
import crypto from 'k6/crypto';
import { failFastHost, baseUrl } from './lib/guards.js';
import { routeCredentialMatrix, webhookTestSecret } from './lib/credentials.js';
import {
  classifyResponse,
  recordClassification,
  loginAllRoles,
  authHeadersForRole,
  refreshRoleIfNeeded,
} from './lib/auth.js';
import { postSignedWebhook } from './lib/webhook.js';

failFastHost();

export const options = {
  scenarios: {
    diagnostic: {
      executor: 'constant-vus',
      vus: Number(__ENV.VUS || 3),
      duration: __ENV.DURATION || '3m',
      exec: 'diagnosticPhase',
    },
    security_negative: {
      executor: 'constant-vus',
      vus: 1,
      duration: __ENV.DURATION || '3m',
      startTime: '0s',
      exec: 'securityNegativePhase',
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
    unexpected_401: ['count==0'],
    unexpected_403: ['count==0'],
    unexpected_404: ['count==0'],
    unexpected_5xx: ['count==0'],
    auth_login_failures: ['count==0'],
  },
};

export function setup() {
  const tokens = loginAllRoles();
  return { tokens };
}

export function diagnosticPhase(data) {
  refreshRoleIfNeeded(data.tokens);
  const routes = [
    'productList',
    'orderList',
    'inventoryList',
    'taskList',
    'webhookEventList',
    'operationLogList',
  ];
  for (const name of routes) {
    const route = routeCredentialMatrix[name];
    const res = http.get(`${baseUrl()}${route.path}`, {
      headers: authHeadersForRole(data.tokens, route.credentialRole),
      tags: { scenario: name, group: 'read-heavy', credential: route.credentialRole },
    });
    const cls = classifyResponse(res, route, 'diagnostic');
    recordClassification(cls);
    check(res, { [`${name} ok`]: () => !cls.unexpected });
  }
  const wh = webhookValidScenario();
  const whCls = classifyResponse(wh, routeCredentialMatrix.webhookValidIngestion, 'diagnostic');
  recordClassification(whCls);
  check(wh, { 'webhook ok': () => !whCls.unexpected });
  sleep(0.5);
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
  recordClassification(classifyResponse(invalidLogin, routeCredentialMatrix.authInvalidLogin, 'security'));

  const body = JSON.stringify({ eventId: `p7v2-bad-${__VU}-${__ITER}`, type: 'order_created' });
  const ts = Math.floor(Date.now() / 1000);
  const invalidWebhook = http.post(`${baseUrl()}/api/v1/webhooks/internal-test/order_created`, body, {
    headers: {
      'Content-Type': 'application/json',
      'X-Webhook-Signature': 'deadbeef',
      'X-Webhook-Timestamp': String(ts),
    },
    tags: { scenario: 'webhook-invalid-signature', group: 'auth-security' },
    responseCallback: http.expectedStatuses(401, 403),
  });
  recordClassification(classifyResponse(invalidWebhook, routeCredentialMatrix.webhookInvalidSignature, 'security'));
  sleep(1);
}

function webhookValidScenario() {
  const eventId = `p7v2-diag-${__VU}-${__ITER}`;
  return postSignedWebhook('/api/v1/webhooks/internal-test/ping', { eventId }, {
    scenario: 'webhook-valid-ingestion',
    group: 'mixed',
  });
}

export default function (data) {
  diagnosticPhase(data);
}
