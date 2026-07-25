import { ok } from './envelope';
import { E2E_PRODUCT_ID } from './product.fixture';

export function inventoryResponse(path: string) {
  if (path === `/api/v1/products/${E2E_PRODUCT_ID}/publication-skus`) return ok({ list: [] });
  if (path.includes('/inventory-logs')) return ok({ list: [], pagination: { page: 1, pageSize: 20, total: 0, totalPages: 1 } });
  if (path === '/api/v1/inventory') return ok({ list: [], pagination: { page: 1, pageSize: 20, total: 0, totalPages: 1 } });
  if (path === '/api/v1/inventory/alerts') return ok({ list: [], pagination: { page: 1, pageSize: 20, total: 0, totalPages: 1 } });
  return null;
}
