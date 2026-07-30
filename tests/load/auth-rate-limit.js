import http from 'k6/http';
import { check, sleep } from 'k6';

const base = __ENV.BASE_URL || 'http://127.0.0.1:8080';

export const options = {
  vus: Number(__ENV.VUS || 3),
  duration: __ENV.DURATION || '30s',
  thresholds: { http_req_duration: ['p(95)<1500'] },
};

export default function () {
  const res = http.post(`${base}/api/v1/auth/login`, JSON.stringify({ account: 'p7-invalid@example.invalid', password: 'wrong-password' }), {
    headers: { 'Content-Type': 'application/json' },
    tags: { scenario: 'auth-rate-limit' },
  });
  check(res, { 'no server error': (r) => r.status < 500 });
  sleep(1);
}
