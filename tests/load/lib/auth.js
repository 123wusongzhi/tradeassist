import http from 'k6/http';
import { Counter } from 'k6/metrics';
import { baseUrl } from './guards.js';
import {
  ROLE_SYSTEM_ADMIN,
  ROLE_TENANT_ADMIN,
  ROLE_OPERATOR,
  ROLE_READONLY,
  accountForRole,
  passwordForRole,
} from './credentials.js';

export const unexpected401 = new Counter('unexpected_401');
export const unexpected403 = new Counter('unexpected_403');
export const unexpected404 = new Counter('unexpected_404');
export const unexpected429 = new Counter('unexpected_429');
export const unexpected5xx = new Counter('unexpected_5xx');
export const expectedSecurityRejections = new Counter('expected_security_rejections');
export const expectedRateLimits = new Counter('expected_rate_limits');
export const authLoginFailures = new Counter('auth_login_failures');

const tokenExpiry = {};

export function loginAllRoles() {
  const roles = [ROLE_SYSTEM_ADMIN, ROLE_TENANT_ADMIN, ROLE_OPERATOR, ROLE_READONLY];
  const tokens = {};
  for (const role of roles) {
    const session = loginRole(role);
    if (!session.token) {
      authLoginFailures.add(1);
      throw new Error(`login failed for ${role}`);
    }
    tokens[role] = session.token;
    tokenExpiry[role] = session.expiresAt;
  }
  return tokens;
}

export function loginRole(role) {
  const account = accountForRole(role);
  const password = passwordForRole(role);
  if (!account || !password) {
    return { token: '', expiresAt: 0 };
  }
  const res = http.post(
    `${baseUrl()}/api/v1/auth/login`,
    JSON.stringify({ account, password }),
    { headers: { 'Content-Type': 'application/json' }, tags: { scenario: `login-${role}`, group: 'auth' } },
  );
  if (res.status !== 200) {
    return { token: '', expiresAt: 0 };
  }
  try {
    const json = JSON.parse(res.body);
    const token = json?.data?.token || json?.data?.accessToken || '';
    const expiresAt = Number(json?.data?.expiresAt || 0);
    return { token, expiresAt };
  } catch {
    return { token: '', expiresAt: 0 };
  }
}

export function authHeadersForRole(tokens, role) {
  const token = tokens?.[role] || '';
  if (!token) return {};
  return { Authorization: `Bearer ${token}` };
}

export function refreshRoleIfNeeded(tokens) {
  if (!tokens) return;
  const now = Math.floor(Date.now() / 1000);
  const roles = [ROLE_SYSTEM_ADMIN, ROLE_TENANT_ADMIN, ROLE_OPERATOR, ROLE_READONLY];
  for (const role of roles) {
    const exp = tokenExpiry[role] || 0;
    if (exp > 0 && exp - now > 120) continue;
    const session = loginRole(role);
    if (session.token) {
      tokens[role] = session.token;
      tokenExpiry[role] = session.expiresAt;
    } else {
      authLoginFailures.add(1);
    }
  }
}

export function classifyResponse(res, route, phase) {
  const status = res?.status || 0;
  const expected = route?.expectedStatus || 200;
  const securityNegative = Boolean(route?.securityNegative);
  let failureClass = 'unknown';
  let unexpected = false;

  if (status === 0) {
    failureClass = 'network_error';
    unexpected = true;
  } else if (securityNegative && (status === 401 || status === 403)) {
    failureClass = 'expected_security_rejection';
    unexpected = false;
  } else if (status === expected) {
    failureClass = 'ok';
    unexpected = false;
  } else if (status === 401) {
    failureClass = 'unexpected_authentication_failure';
    unexpected = !securityNegative;
  } else if (status === 403) {
    failureClass = 'unexpected_authorization_failure';
    unexpected = !securityNegative;
  } else if (status === 404) {
    failureClass = 'route_not_found';
    unexpected = true;
  } else if (status === 429) {
    failureClass = phase === 'rate-limit' ? 'expected_rate_limit' : 'unexpected_rate_limit';
    unexpected = phase !== 'rate-limit';
  } else if (status >= 500) {
    failureClass = 'application_5xx';
    unexpected = true;
  } else if (status >= 400) {
    failureClass = 'application_4xx';
    unexpected = true;
  }

  return {
    route: route?.route || route?.scenario || 'unknown',
    method: route?.method || 'GET',
    scenario: route?.scenario || 'unknown',
    credentialRole: route?.credentialRole || 'unknown',
    statusCode: status,
    expectedStatus: expected,
    failureClass,
    unexpected,
  };
}

export function recordClassification(cls) {
  if (!cls) return;
  if (cls.failureClass === 'expected_security_rejection') {
    expectedSecurityRejections.add(1);
    return;
  }
  if (cls.failureClass === 'expected_rate_limit') {
    expectedRateLimits.add(1);
    return;
  }
  if (!cls.unexpected) return;
  if (cls.statusCode === 401) unexpected401.add(1);
  else if (cls.statusCode === 403) unexpected403.add(1);
  else if (cls.statusCode === 404) unexpected404.add(1);
  else if (cls.statusCode === 429) unexpected429.add(1);
  else if (cls.statusCode >= 500) unexpected5xx.add(1);
}
