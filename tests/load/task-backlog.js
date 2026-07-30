import http from 'k6/http';
import { check, sleep } from 'k6';

const base = __ENV.BASE_URL || 'http://127.0.0.1:8080';

export const options = {
  vus: Number(__ENV.VUS || 4),
  duration: __ENV.DURATION || '30s',
  thresholds: { http_req_duration: ['p(95)<1500'] },
};

export default function () {
  const res = http.get(`${base}/api/v1/task-center/failures?page=1&pageSize=20`, { tags: { scenario: 'task-backlog' } });
  check(res, { 'no server error': (r) => r.status < 500 });
  sleep(0.5);
}
