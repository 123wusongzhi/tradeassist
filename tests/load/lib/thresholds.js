import http from 'k6/http';
import { authHeaders, baseUrl } from './guards.js';

export const thresholds = {
  http_req_failed: ['rate<0.01'],
  'http_req_duration{group:read-heavy}': ['p(95)<800', 'p(99)<1500'],
  'http_req_duration{group:mixed}': ['p(95)<1200', 'p(99)<2500'],
  'http_req_duration{group:auth}': ['p(95)<500', 'p(99)<1000'],
};

export function mixedScenario() {
  const reads = [
    '/api/v1/products?pageSize=20',
    '/api/v1/orders?pageSize=20',
    '/api/v1/inventory?pageSize=20',
    '/api/v1/task-center/failures?pageSize=20',
    '/api/v1/operation-logs?pageSize=20',
  ];
  const path = reads[__ITER % reads.length];
  return http.get(`${baseUrl()}${path}`, {
    headers: authHeaders(),
    tags: { scenario: 'mixed-read', group: 'mixed' },
  });
}

export function authScenario() {
  return http.post(
    `${baseUrl()}/api/v1/auth/login`,
    JSON.stringify({ account: 'p7v2-invalid@example.invalid', password: 'wrong-password' }),
    { headers: { 'Content-Type': 'application/json' }, tags: { scenario: 'auth-security', group: 'auth' } },
  );
}

export function webhookScenario() {
  const eventId = `p7v2-${__VU}-${__ITER}`;
  return http.post(
    `${baseUrl()}/api/v1/webhooks/mock/order_created`,
    JSON.stringify({ eventId, type: 'order_created' }),
    {
      headers: { 'Content-Type': 'application/json', 'X-P7-Mock-Signature': 'mock' },
      tags: { scenario: 'webhook-ingestion', group: 'mixed' },
    },
  );
}

export function providerScenario() {
  return http.get(`${baseUrl()}/health/live`, { tags: { scenario: 'provider-mock-flow', group: 'mixed' } });
}
