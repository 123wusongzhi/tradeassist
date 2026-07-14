import { check, sleep } from 'k6';
import { failFastHost } from './lib/guards.js';
import { followCursor } from './lib/cursor.js';
import { thresholds } from './lib/thresholds.js';

failFastHost();

export const options = {
  scenarios: {
    smoke: {
      executor: 'constant-vus',
      vus: Number(__ENV.VUS || 2),
      duration: __ENV.DURATION || '2m',
    },
  },
  thresholds,
};

export default function () {
  const res = followCursor('productList', 1);
  check(res, { 'status bounded': (r) => r.status < 500 });
  sleep(1);
}
