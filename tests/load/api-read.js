import http from 'k6/http';
import { check, sleep } from 'k6';

const base = __ENV.BASE_URL || 'http://127.0.0.1:8080';

export const options = {
  vus: Number(__ENV.VUS || 5),
  duration: __ENV.DURATION || '30s',
  thresholds: {
    http_req_failed: ['rate<0.05'],
    http_req_duration: ['p(95)<1000'],
  },
};

export default function () {
  for (const path of ['/health/live', '/api/v1/products?page=1&pageSize=20', '/api/v1/orders?page=1&pageSize=20', '/api/v1/inventory?page=1&pageSize=20']) {
    const res = http.get(`${base}${path}`, { tags: { scenario: 'api-read', path } });
    check(res, { 'status is bounded': (r) => r.status < 500 });
  }
  sleep(1);
}
