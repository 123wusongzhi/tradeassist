export const ROLE_SYSTEM_ADMIN = 'system_admin';
export const ROLE_TENANT_ADMIN = 'tenant_admin';
export const ROLE_OPERATOR = 'operator';
export const ROLE_READONLY = 'readonly';
export const ROLE_DISABLED = 'disabled';

export const routeCredentialMatrix = {
  productList: {
    route: 'Product List',
    method: 'GET',
    path: '/api/v1/products?pageSize=20',
    scenario: 'productList',
    credentialRole: ROLE_TENANT_ADMIN,
    expectedStatus: 200,
    authRequired: true,
  },
  orderList: {
    route: 'Order List',
    method: 'GET',
    path: '/api/v1/orders?pageSize=20',
    scenario: 'orderList',
    credentialRole: ROLE_TENANT_ADMIN,
    expectedStatus: 200,
    authRequired: true,
  },
  inventoryList: {
    route: 'Inventory List',
    method: 'GET',
    path: '/api/v1/inventory?pageSize=20',
    scenario: 'inventoryList',
    credentialRole: ROLE_OPERATOR,
    expectedStatus: 200,
    authRequired: true,
  },
  taskList: {
    route: 'Task List',
    method: 'GET',
    path: '/api/v1/task-center/failures?pageSize=20',
    scenario: 'taskList',
    credentialRole: ROLE_OPERATOR,
    expectedStatus: 200,
    authRequired: true,
  },
  webhookEventList: {
    route: 'Webhook Event List',
    method: 'GET',
    path: '/api/v1/webhook-events?pageSize=20',
    scenario: 'webhookEventList',
    credentialRole: ROLE_TENANT_ADMIN,
    expectedStatus: 200,
    authRequired: true,
  },
  operationLogList: {
    route: 'Operation Log List',
    method: 'GET',
    path: '/api/v1/operation-logs?pageSize=20',
    scenario: 'operationLogList',
    credentialRole: ROLE_SYSTEM_ADMIN,
    expectedStatus: 200,
    authRequired: true,
  },
  webhookValidIngestion: {
    route: 'Webhook Ingestion',
    method: 'POST',
    path: '/api/v1/webhooks/internal-test/ping',
    scenario: 'webhook-valid-ingestion',
    credentialRole: 'none',
    expectedStatus: 200,
    authRequired: false,
  },
  webhookInvalidSignature: {
    route: 'Webhook Invalid Signature',
    method: 'POST',
    path: '/api/v1/webhooks/internal-test/ping',
    scenario: 'webhook-invalid-signature',
    credentialRole: 'none',
    expectedStatus: 401,
    authRequired: false,
    securityNegative: true,
  },
  authInvalidLogin: {
    route: 'Invalid Login',
    method: 'POST',
    path: '/api/v1/auth/login',
    scenario: 'auth-invalid-login',
    credentialRole: 'none',
    expectedStatus: 401,
    authRequired: false,
    securityNegative: true,
  },
  authRefresh: {
    route: 'Auth Refresh',
    method: 'POST',
    path: '/api/v1/auth/refresh',
    scenario: 'auth-refresh',
    credentialRole: ROLE_TENANT_ADMIN,
    expectedStatus: 200,
    authRequired: true,
  },
  healthLive: {
    route: 'Health Live',
    method: 'GET',
    path: '/health/live',
    scenario: 'provider-mock-flow',
    credentialRole: 'none',
    expectedStatus: 200,
    authRequired: false,
  },
};

export function accountForRole(role) {
  switch (role) {
    case ROLE_SYSTEM_ADMIN:
      return __ENV.P7_PERF_SYSTEM_ADMIN_EMAIL || 'p7v2-perf-admin@example.invalid';
    case ROLE_TENANT_ADMIN:
      return __ENV.P7_PERF_TENANT_ADMIN_EMAIL || 'p7v2-perf-tenant-admin@example.invalid';
    case ROLE_OPERATOR:
      return __ENV.P7_PERF_OPERATOR_EMAIL || 'p7v2-perf-operator@example.invalid';
    case ROLE_READONLY:
      return __ENV.P7_PERF_READONLY_EMAIL || 'p7v2-perf-readonly@example.invalid';
    case ROLE_DISABLED:
      return __ENV.P7_PERF_DISABLED_EMAIL || 'p7v2-perf-disabled@example.invalid';
    default:
      return '';
  }
}

export function passwordForRole(role) {
  switch (role) {
    case ROLE_SYSTEM_ADMIN:
      return __ENV.P7V2_PERF_ADMIN_PASSWORD || __ENV.P7_AUTH_PASSWORD || '';
    case ROLE_TENANT_ADMIN:
      return __ENV.P7V2_PERF_TENANT_ADMIN_PASSWORD || '';
    case ROLE_OPERATOR:
      return __ENV.P7V2_PERF_OPERATOR_PASSWORD || '';
    case ROLE_READONLY:
      return __ENV.P7V2_PERF_READONLY_PASSWORD || '';
    case ROLE_DISABLED:
      return __ENV.P7V2_PERF_DISABLED_PASSWORD || __ENV.P7V2_PERF_OPERATOR_PASSWORD || '';
    default:
      return '';
  }
}

export function webhookTestSecret() {
  return __ENV.P7V2_WEBHOOK_TEST_SECRET || 'trademind-internal-test-webhook-secret';
}

export function matrixFingerprint() {
  return Object.keys(routeCredentialMatrix)
    .sort()
    .map((k) => `${k}:${routeCredentialMatrix[k].credentialRole}`)
    .join('|');
}
