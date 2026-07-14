import { baselineRequests, validateFrozenBaseline } from './p7-v2-evidence-resolver.mjs';

const requiredCooldown = [
  'queueRecovered',
  'workerInflightRecovered',
  'providerInflightRecovered',
  'dbConnectionsRecovered',
  'goroutinesRecovered',
  'memoryRecovered',
  'webhookBacklogRecovered',
  'providerStateRecovered',
  'circuitRecovered',
];

export function validateCurrent(current = {}) {
  const evidence = current.restartEvidence || {};
  const required = [
    'restartPerformed', 'apiProcessChanged', 'workerProcessChanged', 'redisRestarted',
    'mockProviderRestarted', 'databaseStateReset', 'bootstrapPassed', 'authProbePassed',
    'routeProbePassed', 'datasetVerified', 'serverReady',
  ];
  const issues = [];
  if (current.status !== 'passed') issues.push('current status is not passed');
  if (baselineRequests(current) <= 0) issues.push('current has zero requests');
  for (const key of required) if (evidence[key] !== true) issues.push(`restart evidence missing: ${key}`);
  if (!evidence.databaseResetMethod) issues.push('database reset method is missing');
  if (current.independentRun !== true || current.currentRunIndependent !== true) issues.push('current run is not independently derived');
  return { valid: issues.length === 0, issues };
}

export function validateSoak(soak = {}) {
  const timing = soak.timing || soak;
  const cooldown = soak.cooldown || soak;
  const issues = [];
  if (soak.status !== 'passed') issues.push('soak status is not passed');
  if (Number(timing.actualSteadySeconds || 0) < 1800) issues.push('steady window is shorter than 1800 seconds');
  if (timing.continuousSteadyWindow !== true) issues.push('continuous steady window is not proven');
  if (Number(timing.samplesCount ?? timing.steadySampleCount ?? 0) < 29) issues.push('steady samples are insufficient');
  if (Number(timing.maxSampleGapSeconds || Infinity) > 90) issues.push('steady sample gap exceeds 90 seconds');
  if (Number(cooldown.actualMinutes || 0) < 5) issues.push('cooldown is shorter than 5 minutes');
  for (const key of requiredCooldown) if (cooldown[key] !== true) issues.push(`cooldown recovery missing: ${key}`);
  if (cooldown.cooldownRecoveryPassed !== true) issues.push('cooldown recovery is not proven');
  return { valid: issues.length === 0, issues };
}

export function validateDemo(run1 = {}, run2 = {}) {
  const issues = [];
  if (run1.status !== 'passed' || run2.status !== 'passed') issues.push('both demo runs must pass');
  if (!run1.runId || !run2.runId || run1.runId === run2.runId) issues.push('demo run IDs are missing or equal');
  if (run2.independent !== true) issues.push('demo run 2 is not independent');
  return { valid: issues.length === 0, issues };
}

export function validateCleanup(cleanup = {}) {
  const issues = [];
  for (const key of ['remainingDatabasesWithPrefix', 'processesRemaining', 'portsRemaining']) {
    if (Number(cleanup[key]) !== 0) issues.push(`${key} is not zero`);
  }
  return { valid: cleanup.status === 'passed' && issues.length === 0, issues };
}

export function validateRegression(regression = {}) {
  const issues = [];
  if (regression.status !== 'passed') issues.push('regression status is not passed');
  if (Number(regression.failedMetricCount || 0) !== 0) issues.push('regression has failed metrics');
  if (Number(regression.notComparableCount || 0) !== 0) issues.push('regression has non-comparable metrics');
  return { valid: issues.length === 0, issues };
}

export { validateFrozenBaseline };
