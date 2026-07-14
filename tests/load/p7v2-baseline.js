import { check, sleep } from 'k6';
import { failFastHost } from './lib/guards.js';
import { followCursor } from './lib/cursor.js';
import { authScenario, mixedScenario, providerScenario, thresholds, webhookScenario } from './lib/thresholds.js';

failFastHost();

const targetVUs = Number(__ENV.TARGET_VUS || 10);
const warmupDur = __ENV.WARMUP || '5m';
const rampDur = __ENV.RAMP || '3m';
const steadyDur = __ENV.STEADY || '10m';
const rampdownDur = __ENV.RAMPDOWN || '2m';

export const options = {
  scenarios: {
    warmup: {
      executor: 'constant-vus',
      vus: Math.max(2, Math.floor(targetVUs * 0.3)),
      duration: warmupDur,
      startTime: '0s',
      exec: 'warmupPhase',
    },
    ramp: {
      executor: 'ramping-vus',
      startVUs: Math.max(2, Math.floor(targetVUs * 0.3)),
      stages: [
        { duration: rampDur, target: targetVUs },
      ],
      startTime: warmupDur,
      exec: 'steadyPhase',
    },
    steady: {
      executor: 'constant-vus',
      vus: targetVUs,
      duration: steadyDur,
      startTime: addDuration(warmupDur, rampDur),
      exec: 'steadyPhase',
    },
    rampdown: {
      executor: 'ramping-vus',
      startVUs: targetVUs,
      stages: [{ duration: rampdownDur, target: 0 }],
      startTime: addDuration(warmupDur, rampDur, steadyDur),
      exec: 'steadyPhase',
    },
  },
  thresholds,
};

export function warmupPhase() {
  const res = mixedScenario();
  check(res, { 'warmup bounded': (r) => r.status < 500 });
  sleep(0.5);
}

export function steadyPhase() {
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
  steadyPhase();
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
