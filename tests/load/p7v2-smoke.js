import { check, sleep } from 'k6';
import { failFastHost } from './lib/guards.js';
import { routeCredentialMatrix } from './lib/credentials.js';
import { loginAllRoles, authHeadersForRole } from './lib/auth.js';
import http from 'k6/http';
import { baseUrl } from './lib/guards.js';

failFastHost();

export const options = {
  scenarios: {
    smoke: {
      executor: 'constant-vus',
      vus: Number(__ENV.VUS || 2),
      duration: __ENV.DURATION || '2m',
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
    unexpected_401: ['count==0'],
    auth_login_failures: ['count==0'],
  },
};

export function setup() {
  return { tokens: loginAllRoles() };
}

export default function (data) {
  const route = routeCredentialMatrix.productList;
  const res = http.get(`${baseUrl()}${route.path}`, {
    headers: authHeadersForRole(data.tokens, route.credentialRole),
    tags: { scenario: 'productList', group: 'read-heavy' },
  });
  check(res, { 'status ok': (r) => r.status === 200 });
  sleep(1);
}
