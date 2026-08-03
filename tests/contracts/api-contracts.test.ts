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
        'GET /api/v1/collect/engines/status',
        'GET /api/v1/image/providers',
        'GET /api/v1/products/:id',
        'GET /api/v1/products/:id/readiness',
        'GET /api/v1/products/:id/publications',
        'GET /api/v1/product-publications/:id/douyin/sku-bindings',
        'GET /api/v1/products/:id/publish-targets',
        'POST /api/v1/collect/batches',
        'POST /api/v1/collect/tasks',
        'POST /api/v1/products/:id/platform-configs/douyin_shop/create-draft',
        'POST /api/v1/products/:id/publish',
        'GET /api/v1/platform/ozon/categories',
        'POST /api/v1/platform/ozon/categories/sync',
        'GET /api/v1/platform/ozon/categories/sync-runs',
        'GET /api/v1/platform/ozon/categories/sync-runs/:id',
        'GET /api/v1/platform/ozon/categories/changes',
        'GET /api/v1/platform/ozon/categories/stats',
        'GET /api/v1/platform/ozon/categories/:id/attributes',
        'GET /api/v1/platform/ozon/categories/:id/attributes/:attrId/values',
        'POST /api/v1/platform/ozon/categories/:id/attributes/sync',
        'GET /api/v1/platform/ozon/categories/:id/attribute-mappings',
        'PUT /api/v1/platform/ozon/categories/:id/attribute-mappings',
        'GET /api/v1/platform/ozon/category-mappings',
        'POST /api/v1/platform/ozon/category-mappings/recommend',
        'PUT /api/v1/platform/ozon/category-mappings',
        'GET /api/v1/products/:id/platform-configs/ozon',
        'PUT /api/v1/products/:id/platform-configs/ozon',
        'POST /api/v1/products/:id/readiness/validate',
        'POST /api/v1/product-publish/ozon/category-groups/check',
        'POST /api/v1/product-publish/ozon/category-groups/confirm',
      ]),
    );
  });

  it('defines payload/query contracts for state-changing publish APIs', () => {
    const createDraft = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/products/:id/platform-configs/douyin_shop/create-draft');
    const publish = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/products/:id/publish');
    const readiness = contracts.endpoints.find((item) => routeKey(item) === 'GET /api/v1/products/:id/readiness');
    const collectTask = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/collect/tasks');
    const collectBatch = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/collect/batches');
    const ozonMappings = contracts.endpoints.find((item) => routeKey(item) === 'PUT /api/v1/platform/ozon/categories/:id/attribute-mappings');
    const ozonCategories = contracts.endpoints.find((item) => routeKey(item) === 'GET /api/v1/platform/ozon/categories');
    const ozonDictionaryValues = contracts.endpoints.find((item) => routeKey(item) === 'GET /api/v1/platform/ozon/categories/:id/attributes/:attrId/values');
    const ozonConfig = contracts.endpoints.find((item) => routeKey(item) === 'PUT /api/v1/products/:id/platform-configs/ozon');
    const ozonReadiness = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/products/:id/readiness/validate');
    const ozonGroupConfirm = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/product-publish/ozon/category-groups/confirm');
    const ozonChanges = contracts.endpoints.find((item) => routeKey(item) === 'GET /api/v1/platform/ozon/categories/changes');

    expect(createDraft?.requestBody).toEqual(['shopId', 'publishMode', 'force']);
    expect(publish?.requestBody).toEqual(['shopId', 'options', 'force']);
    expect(readiness?.query).toEqual(['platform', 'shopId', 'mode']);
    expect(collectTask?.requestBody).toContain('engine');
    expect(collectBatch?.requestBody).toEqual(['source', 'urls', 'engine']);
    expect(ozonMappings?.requestBody).toEqual(['items']);
    expect(ozonCategories?.query).toEqual(['keyword', 'onlyLeaf', 'activeOnly', 'limit']);
    expect(ozonDictionaryValues?.query).toEqual(['shopId', 'keyword']);
    expect(publish?.conditionalRequiredHeaders).toEqual({ ozon: ['Idempotency-Key'] });
    expect(ozonConfig?.requestBody).toEqual(['shopId', 'categoryId', 'categoryPath', 'platformAttributes', 'sourceCategoryKey', 'sourceCategoryName']);
    expect(ozonConfig?.responseFields).toContain('id');
    expect(ozonConfig?.errorStatuses).toEqual([400, 403, 404]);
    expect(ozonReadiness?.requestBody).toEqual(['platform', 'shopId']);
    expect(ozonReadiness?.errorStatuses).toEqual([400, 403, 404, 502, 503]);
    expect(ozonReadiness?.errorDataFields).toEqual(['errorCode']);
    expect(ozonGroupConfirm?.requestBody).toEqual(['shopId', 'groups', 'saveMappings']);
    expect(ozonChanges?.responseFields).toEqual(['list[].categoryName', 'list[].occurredAt', 'list[].detail']);
  });

  it('marks every protected Admin endpoint as authenticated', () => {
    expect(contracts.endpoints).toHaveLength(31);
    expect(contracts.endpoints.every((endpoint) => endpoint.auth === true)).toBe(true);
  });
});
