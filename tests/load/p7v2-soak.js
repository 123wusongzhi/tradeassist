import { check, sleep } from 'k6';
import { failFastHost } from './lib/guards.js';
import { mixedScenario, thresholds } from './lib/thresholds.js';

failFastHost();

const targetVUs = Number(__ENV.TARGET_VUS || 8);
const warmupDur = __ENV.WARMUP || '5m';
const steadyDur = __ENV.STEADY || '30m';
const rampdownDur = __ENV.RAMPDOWN || '2m';

export const options = {
  scenarios: {
    warmup: {
      executor: 'constant-vus',
      vus: Math.max(2, Math.floor(targetVUs * 0.5)),
      duration: warmupDur,
      exec: 'warmupPhase',
    },
    steady: {
      executor: 'constant-vus',
      vus: targetVUs,
      duration: steadyDur,
      startTime: warmupDur,
      exec: 'steadyPhase',
    },
    rampdown: {
      executor: 'ramping-vus',
      startVUs: targetVUs,
      stages: [{ duration: rampdownDur, target: 0 }],
      startTime: addDuration(warmupDur, steadyDur),
      exec: 'steadyPhase',
    },
  },
  thresholds,
};

export function warmupPhase() {
  const res = mixedScenario();
  check(res, { 'soak warmup bounded': (r) => r.status < 500 });
  sleep(0.5);
}

export function steadyPhase() {
  const res = mixedScenario();
  check(res, { 'soak steady bounded': (r) => r.status < 500 || r.status === 401 || r.status === 429 });
  sleep(0.4);
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
