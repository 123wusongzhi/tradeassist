import { check, sleep } from 'k6';
import { failFastHost } from './lib/guards.js';
import { followCursor } from './lib/cursor.js';
import { authScenario, mixedScenario, providerScenario, thresholds, webhookScenario } from './lib/thresholds.js';

failFastHost();

const targetVUs = Number(__ENV.TARGET_VUS || 10);
const warmup = __ENV.WARMUP || '5m';
const ramp = __ENV.RAMP || '3m';
const steady = __ENV.STEADY || '10m';
const rampdown = __ENV.RAMPDOWN || '2m';

export const options = {
  scenarios: {
    warmup: {
      executor: 'constant-vus',
      vus: Math.max(2, Math.floor(targetVUs * 0.3)),
      duration: warmup,
      startTime: '0s',
      exec: 'warmup',
    },
    ramp: {
      executor: 'ramping-vus',
      startVUs: Math.max(2, Math.floor(targetVUs * 0.3)),
      stages: [{ duration: ramp, target: targetVUs }],
      startTime: warmup,
      exec: 'steady',
    },
    steady: {
      executor: 'constant-vus',
      vus: targetVUs,
      duration: steady,
      startTime: addDuration(warmup, ramp),
      exec: 'steady',
    },
    rampdown: {
      executor: 'ramping-vus',
      startVUs: targetVUs,
      stages: [{ duration: rampdown, target: 0 }],
      startTime: addDuration(warmup, ramp, steady),
      exec: 'steady',
    },
  },
  thresholds,
};

export function warmup() {
  const res = mixedScenario();
  check(res, { 'warmup bounded': (r) => r.status < 500 });
  sleep(0.5);
}

export function steady() {
  const pick = __ITER % 10;
  let res;
  if (pick < 2) res = followCursor('productList', 1);
  else if (pick < 4) res = followCursor('orderList', 1);
  else if (pick < 5) res = followCursor('inventoryList', 1);
  else if (pick < 6) res = followCursor('taskList', 1);
  else if (pick < 7) res = followCursor('webhookEventList', 1);
  else if (pick < 8) res = followCursor('operationLogList', 1);
  else if (pick === 8) res = webhookScenario();
  else if (pick === 9) res = providerScenario();
  else res = authScenario();
  check(res, { 'steady bounded': (r) => r.status < 500 || r.status === 401 || r.status === 429 });
  sleep(0.3);
}

export default function () {
  steady();
}

function addDuration(...parts) {
  let total = 0;
  for (const part of parts) {
    const m = String(part).match(/^(\d+)(s|m|h)$/);
    if (!m) continue;
    const n = Number(m[1]);
    total += m[2] === 'h' ? n * 3600 : m[2] === 'm' ? n * 60 : n;
  }
  return `${total}s`;
}
