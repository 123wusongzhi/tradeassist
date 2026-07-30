import http from 'k6/http';
import { check, sleep } from 'k6';

const base = __ENV.BASE_URL || 'http://127.0.0.1:8080';

export const options = {
  vus: Number(__ENV.VUS || 5),
  duration: __ENV.DURATION || '30s',
  thresholds: { http_req_duration: ['p(95)<1500'] },
};

export default function () {
  const eventId = `p7-${__VU}-${__ITER}`;
  const res = http.post(`${base}/api/v1/webhooks/mock/order_created`, JSON.stringify({ eventId, type: 'order_created' }), {
    headers: { 'Content-Type': 'application/json', 'X-P7-Mock-Signature': 'mock' },
    tags: { scenario: 'webhook-burst' },
  });
  check(res, { 'bounded response': (r) => r.status < 500 || r.status === 501 });
  sleep(0.2);
}
