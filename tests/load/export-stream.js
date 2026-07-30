import http from 'k6/http';
import { check, sleep } from 'k6';

const base = __ENV.BASE_URL || 'http://127.0.0.1:8080';

export const options = {
  vus: Number(__ENV.VUS || 2),
  duration: __ENV.DURATION || '30s',
  thresholds: { http_req_duration: ['p(95)<2000'] },
};

export default function () {
  const res = http.get(`${base}/api/v1/exports/mock-p7`, { tags: { scenario: 'export-stream' } });
  check(res, { 'bounded response': (r) => r.status < 500 || r.status === 404 || r.status === 403 });
  sleep(1);
}
