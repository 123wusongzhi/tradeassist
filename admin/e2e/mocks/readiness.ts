import { ok } from './envelope';
import { e2eReadinessPassed, E2E_PRODUCT_ID } from './product.fixture';

export function readinessResponse(path: string) {
  if (path === `/api/v1/products/${E2E_PRODUCT_ID}/readiness`) return ok(e2eReadinessPassed);
  return null;
}
