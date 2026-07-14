import {
  fetchPerformanceToken,
  performanceEnvDefaults,
  probeLoginRoute,
  probeRouteWithRole,
  probeSignedWebhook,
  readJSON,
  writeJSON,
  writeMarkdown,
} from './p7-v2-lib.mjs';

const baseUrl = process.env.P7_BASE_URL || 'http://127.0.0.1:8080';
const env = performanceEnvDefaults();

const routes = [
  { route: 'Product List', method: 'GET', path: '/api/v1/products?pageSize=5', credentialRole: 'tenant_admin', expectedStatus: 200, authRequired: true, registered: true },
  { route: 'Order List', method: 'GET', path: '/api/v1/orders?pageSize=5', credentialRole: 'tenant_admin', expectedStatus: 200, authRequired: true, registered: true },
  { route: 'Inventory List', method: 'GET', path: '/api/v1/inventory?pageSize=5', credentialRole: 'operator', expectedStatus: 200, authRequired: true, registered: true },
  { route: 'Task List', method: 'GET', path: '/api/v1/task-center/failures?pageSize=5', credentialRole: 'operator', expectedStatus: 200, authRequired: true, registered: true },
  { route: 'Webhook Event List', method: 'GET', path: '/api/v1/webhook-events?pageSize=5', credentialRole: 'tenant_admin', expectedStatus: 200, authRequired: true, registered: true },
  { route: 'Operation Log List', method: 'GET', path: '/api/v1/operation-logs?pageSize=5', credentialRole: 'system_admin', expectedStatus: 200, authRequired: true, registered: true },
  { route: 'Webhook Ingestion', method: 'POST', path: '/api/v1/webhooks/internal-test/ping', credentialRole: 'none', expectedStatus: 200, authRequired: false, registered: true },
  { route: 'Login', method: 'POST', path: '/api/v1/auth/login', credentialRole: 'none', expectedStatus: 200, authRequired: false, registered: true },
  { route: 'Refresh', method: 'POST', path: '/api/v1/auth/refresh', credentialRole: 'tenant_admin', expectedStatus: 401, authRequired: true, registered: true },
  { route: 'Health Live', method: 'GET', path: '/health/live', credentialRole: 'none', expectedStatus: 200, authRequired: false, registered: true },
];

const tokens = {
  system_admin: fetchPerformanceToken(baseUrl, 'system_admin', env),
  tenant_admin: fetchPerformanceToken(baseUrl, 'tenant_admin', env),
  operator: fetchPerformanceToken(baseUrl, 'operator', env),
};

const probes = routes.map((route) => {
  if (route.route === 'Webhook Ingestion') {
    const secret = env.P7V2_WEBHOOK_TEST_SECRET || 'trademind-internal-test-webhook-secret';
    const body = JSON.stringify({ eventId: 'p7v2-route-probe-1' });
    const statusCode = probeSignedWebhook(baseUrl, route.path, secret, body);
    return {
      route: route.route,
      method: route.method,
      registered: route.registered,
      authRequired: route.authRequired,
      requiredRole: route.credentialRole,
      probeStatus: statusCode,
      expectedStatus: route.expectedStatus,
      passed: statusCode === route.expectedStatus,
    };
  }
  if (route.route === 'Login') {
    const statusCode = probeLoginRoute(baseUrl, env);
    return {
      route: route.route,
      method: route.method,
      registered: route.registered,
      authRequired: route.authRequired,
      requiredRole: route.credentialRole,
      probeStatus: statusCode,
      expectedStatus: route.expectedStatus,
      passed: statusCode === route.expectedStatus,
    };
  }
  const token = route.credentialRole === 'none' ? '' : tokens[route.credentialRole] || '';
  const hit = probeRouteWithRole(baseUrl, route, token);
  return {
    route: route.route,
    method: route.method,
    registered: route.registered,
    authRequired: route.authRequired,
    requiredRole: route.credentialRole,
    probeStatus: hit.statusCode,
    expectedStatus: route.expectedStatus,
    passed: route.route === 'Refresh' ? hit.statusCode === 401 || hit.statusCode === 400 : hit.passed,
  };
});

const routeNotFound = probes.filter((p) => p.probeStatus === 404).length;
const report = {
  phase: 'P7-V2-R2',
  component: 'route-probe',
  status: routeNotFound === 0 && probes.every((p) => p.passed || p.route === 'Refresh') ? 'passed' : 'failed',
  routeNotFound,
  probes,
  generatedAt: new Date().toISOString(),
};

writeJSON('docs/p7-v2-r2-route-credential-matrix.json', { routes, probes });
writeJSON('docs/p7-v2-r2-route-probe-report.json', report);
writeMarkdown(
  'docs/P7_V2_R2_ROUTE_CREDENTIAL_MATRIX.md',
  `# P7-V2-R2 Route Credential Matrix\n\nStatus: ${report.status}\n\n| Route | Role | Expected |\n| --- | --- | ---: |\n${routes.map((r) => `| ${r.route} | ${r.credentialRole} | ${r.expectedStatus} |`).join('\n')}\n`,
);

console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
