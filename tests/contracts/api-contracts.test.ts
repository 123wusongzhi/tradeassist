import { describe, expect, it } from 'vitest';
import contracts from './api-contracts.json';

const routeKey = (endpoint: { method: string; path: string }) => `${endpoint.method} ${endpoint.path}`;

describe('TradeMind API contract registry', () => {
  it('keeps the backend envelope explicit for frontend and E2E mocks', () => {
    expect(contracts.envelope.success).toEqual(['code', 'message', 'data']);
    expect(contracts.envelope.optional).toContain('traceId');
    expect(contracts.envelope.errorCodeRule).toContain('non-zero');
  });

  it('covers the core Admin product publishing and readiness endpoints', () => {
    const routes = new Set(contracts.endpoints.map(routeKey));

    expect(routes).toEqual(
      new Set([
        'GET /api/v1/auth/profile',
        'GET /api/v1/image/providers',
        'GET /api/v1/products/:id',
        'GET /api/v1/products/:id/readiness',
        'GET /api/v1/products/:id/publications',
        'GET /api/v1/product-publications/:id/douyin/sku-bindings',
        'GET /api/v1/products/:id/publish-targets',
        'POST /api/v1/products/:id/platform-configs/douyin_shop/create-draft',
        'POST /api/v1/products/:id/publish',
      ]),
    );
  });

  it('defines payload/query contracts for state-changing publish APIs', () => {
    const createDraft = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/products/:id/platform-configs/douyin_shop/create-draft');
    const publish = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/products/:id/publish');
    const readiness = contracts.endpoints.find((item) => routeKey(item) === 'GET /api/v1/products/:id/readiness');

    expect(createDraft?.requestBody).toEqual(['shopId', 'publishMode', 'force']);
    expect(publish?.requestBody).toEqual(['shopId', 'options', 'force']);
    expect(readiness?.query).toEqual(['platform', 'shopId', 'mode']);
  });

  it('marks every protected Admin endpoint as authenticated', () => {
    expect(contracts.endpoints).toHaveLength(9);
    expect(contracts.endpoints.every((endpoint) => endpoint.auth === true)).toBe(true);
  });
});
