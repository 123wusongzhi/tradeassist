function metricValues(rawMetric) {
  return rawMetric?.values || rawMetric || null;
}

function hasOwn(object, key) {
  return Boolean(object && Object.prototype.hasOwnProperty.call(object, key));
}

function numeric(value) {
  return Number.isFinite(Number(value)) ? Number(value) : null;
}

export function metricSummaryValue(rawMetric, key) {
  const values = metricValues(rawMetric);
  if (!values) return null;
  if (hasOwn(values, key)) return numeric(values[key]);
  if (key === 'p50' && hasOwn(values, 'med')) return numeric(values.med);
  if (key === 'count' && hasOwn(values, 'value')) return numeric(values.value);
  return null;
}

export function classifyMetricEvidence({ metricDefinition = {}, rawMetric, sampleMetric, minimumSampleCount = 100, aggregation = 'p(95)' } = {}) {
  if (!rawMetric) {
    return { classification: 'metric_missing', metricPresent: false, sampleMetricPresent: Boolean(sampleMetric), sampleCount: null, value: null };
  }
  const values = metricValues(rawMetric);
  const trendPresent = Boolean(values && (hasOwn(values, 'avg') || hasOwn(values, 'med') || hasOwn(values, 'p(95)') || hasOwn(values, 'p(99)')));
  if (!trendPresent && metricDefinition.metricType !== 'counter') {
    return { classification: 'trend_missing', metricPresent: true, sampleMetricPresent: Boolean(sampleMetric), sampleCount: null, value: null };
  }
  if (!sampleMetric) {
    return { classification: 'sample_counter_missing', metricPresent: true, sampleMetricPresent: false, sampleCount: null, value: null };
  }
  const sampleValues = metricValues(sampleMetric);
  if (!hasOwn(sampleValues, 'count')) {
    return { classification: 'sample_counter_missing', metricPresent: true, sampleMetricPresent: true, sampleCount: null, value: null };
  }
  const sampleCount = numeric(sampleValues.count);
  if (sampleCount === null) {
    return { classification: 'invalid_metric_value', metricPresent: true, sampleMetricPresent: true, sampleCount: null, value: null };
  }
  if (sampleCount < minimumSampleCount) {
    return { classification: 'insufficient_samples', metricPresent: true, sampleMetricPresent: true, sampleCount, value: null };
  }
  const value = metricSummaryValue(rawMetric, aggregation);
  if (value === null) {
    return { classification: 'summary_stat_missing', metricPresent: true, sampleMetricPresent: true, sampleCount, value: null };
  }
  return { classification: 'present', metricPresent: true, sampleMetricPresent: true, sampleCount, value };
}

export function evaluateAbsoluteSlo({ sloId, metricId, metricName, rawMetric, sampleMetric, minimumSampleCount = 100, aggregation = 'p(95)', threshold, unit = 'ms', direction = 'lower_is_better' } = {}) {
  const evidence = classifyMetricEvidence({
    metricDefinition: { metricId, metricName, metricType: 'trend' },
    rawMetric,
    sampleMetric,
    minimumSampleCount,
    aggregation,
  });
  const base = { sloId, metricId, metricName, sampleCount: evidence.sampleCount, minimumSampleCount, aggregation, threshold, unit, direction, actualValue: evidence.value };
  if (evidence.classification === 'metric_missing') return { ...base, metricPresent: false, evaluationStatus: 'not_evaluable_metric_missing', verdict: 'not_evaluable', realAbsoluteSloFailure: false };
  if (evidence.classification === 'sample_counter_missing' || evidence.classification === 'trend_missing') return { ...base, metricPresent: evidence.metricPresent, evaluationStatus: 'not_evaluable_binding_error', verdict: 'not_evaluable', realAbsoluteSloFailure: false };
  if (evidence.classification === 'insufficient_samples') return { ...base, metricPresent: true, evaluationStatus: 'not_evaluable_insufficient_samples', verdict: 'not_evaluable', realAbsoluteSloFailure: false };
  if (evidence.classification === 'invalid_metric_value' || evidence.classification === 'summary_stat_missing') return { ...base, metricPresent: true, evaluationStatus: 'invalid_metric', verdict: 'not_evaluable', realAbsoluteSloFailure: false };
  const failed = direction === 'lower_is_better' ? evidence.value > threshold : evidence.value < threshold;
  return { ...base, metricPresent: true, evaluationStatus: 'evaluated', verdict: failed ? 'failed' : 'passed', realAbsoluteSloFailure: failed };
}

export function evaluateTargetReached({
  loadTargetReached = false,
  steadyStageEntered = false,
  steadyStageCompleted = false,
  steadyDurationReached = false,
  scenarioCoverageReached = false,
  sampleTargetReached = false,
  sloEvaluationCompleted = false,
} = {}) {
  const components = {
    loadTargetReached: Boolean(loadTargetReached),
    steadyStageEntered: Boolean(steadyStageEntered),
    steadyStageCompleted: Boolean(steadyStageCompleted),
    steadyDurationReached: Boolean(steadyDurationReached),
    scenarioCoverageReached: Boolean(scenarioCoverageReached),
    sampleTargetReached: Boolean(sampleTargetReached),
    sloEvaluationCompleted: Boolean(sloEvaluationCompleted),
  };
  return { ...components, targetReached: Object.values(components).every(Boolean) };
}
