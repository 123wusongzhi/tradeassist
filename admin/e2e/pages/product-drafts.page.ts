import { expect, type Page } from '@playwright/test';
import { E2E_PRODUCT_ID } from '../mocks/product.fixture';

export class ProductDraftsPage {
  constructor(readonly page: Page) {}

  async expectLoaded() {
    await expect(this.page.getByText(E2E_PRODUCT_ID).or(this.page.getByText('商品草稿')).first()).toBeVisible();
  }
}
