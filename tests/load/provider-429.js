import http from 'k6/http';
import { check, sleep } from 'k6';

const base = __ENV.BASE_URL || 'http://127.0.0.1:8080';

export const options = {
  vus: Number(__ENV.VUS || 3),
  duration: __ENV.DURATION || '30s',
  thresholds: { http_req_duration: ['p(95)<1500'] },
};

export default function () {
  const res = http.get(`${base}/api/v1/ops/provider-health?mode=mock429`, { tags: { scenario: 'provider-429' } });
  check(res, { 'bounded response': (r) => r.status < 500 || r.status === 404 || r.status === 403 });
  sleep(1);
}
