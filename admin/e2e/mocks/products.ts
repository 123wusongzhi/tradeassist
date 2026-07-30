import { ok, paged } from './envelope';
import { e2eProduct, e2eProductList, e2eProgress, E2E_PRODUCT_ID } from './product.fixture';

export function productsResponse(path: string) {
  if (path === '/api/v1/products') return ok(paged(e2eProductList));
  if (path === `/api/v1/products/${E2E_PRODUCT_ID}`) return ok(e2eProduct);
  if (path === `/api/v1/products/${E2E_PRODUCT_ID}/operation-progress`) return ok(e2eProgress);
  if (path === `/api/v1/products/${E2E_PRODUCT_ID}/ai/tasks`) return ok({ list: [] });
  return null;
}
