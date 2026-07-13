import { request } from '@umijs/max';

export type ObservabilityOverview = {
  enabled: boolean;
  mode: string;
  metricsEnabled: boolean;
  tracingEnabled: boolean;
  alertingEnabled: boolean;
  metricsPath: string;
  metricsInternal: boolean;
  otelExportBlocked: boolean;
  runtimeStatus?: {
    otlpExporter?: string;
    otlpProtocol?: string;
    mockCollectorVerification?: string;
    [key: string]: string | number | undefined;
  };
  telemetry?: {
    dropped?: number;
    exportFailures?: number;
    exportSuccess?: number;
  };
  environment: string;
  timestamp: string;
};

export type AlertEvent = {
  id: string;
  ruleId: string;
  severity: string;
  status: string;
  module: string;
  summary: string;
  occurrenceCount: number;
  lastSeenAt: string;
};

export async function fetchObservabilityOverview() {
  return request<{ data: ObservabilityOverview }>('/api/v1/observability/overview', { method: 'GET' });
}

export async function fetchObservabilityAlerts(params?: { status?: string; limit?: number }) {
  return request<{ data: { items: AlertEvent[] } }>('/api/v1/observability/alerts', {
    method: 'GET',
    params,
  });
}

export async function ackAlert(id: string) {
  return request(`/api/v1/observability/alerts/${id}/ack`, { method: 'POST' });
}

export async function silenceAlert(id: string, body: { reason?: string; durationHours?: number }) {
  return request(`/api/v1/observability/alerts/${id}/silence`, { method: 'POST', data: body });
}
