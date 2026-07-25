import { expect, type Page } from '@playwright/test';
import { E2E_PRODUCT_ID } from '../mocks/product.fixture';
import { expectActiveTab } from '../utils/assertions';

export class ProductDraftDetailPage {
  constructor(readonly page: Page) {}

  async goto(tab = 'basic', section?: string) {
    const params = new URLSearchParams({ tab });
    if (section) params.set('section', section);
    await this.page.goto(`/product/drafts/${E2E_PRODUCT_ID}?${params.toString()}`);
    await expect(this.page.getByText(E2E_PRODUCT_ID).first()).toBeVisible();
  }

  async openTab(tabName: string) {
    await this.page.getByRole('tab', { name: new RegExp(tabName) }).click();
    await expectActiveTab(this.page, tabName);
  }
}
