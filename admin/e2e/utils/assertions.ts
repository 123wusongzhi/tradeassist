import { expect, type Locator, type Page } from '@playwright/test';
import type { ConsoleGuard } from './console-guard';
import type { NetworkWriteGuard } from './network-guard';

export async function expectNoRootOverflow(page: Page) {
  const value = await page.evaluate(() => ({
    htmlScrollWidth: document.documentElement.scrollWidth,
    htmlClientWidth: document.documentElement.clientWidth,
    bodyScrollWidth: document.body.scrollWidth,
    bodyClientWidth: document.body.clientWidth,
  }));
  expect(value.htmlScrollWidth, `html overflow: ${JSON.stringify(value)}`).toBeLessThanOrEqual(value.htmlClientWidth + 1);
  expect(value.bodyScrollWidth, `body overflow: ${JSON.stringify(value)}`).toBeLessThanOrEqual(value.bodyClientWidth + 1);
}

export async function expectActiveTab(page: Page, tabName: string) {
  const tab = page.getByRole('tab', { name: new RegExp(tabName) });
  await expect(tab, `active tab ${tabName}`).toHaveAttribute('aria-selected', 'true');
}

export async function expectSectionVisible(page: Page, sectionId: string) {
  await expect(page.locator(`#${sectionId}`), `section ${sectionId}`).toBeVisible();
}

export async function expectModalWithinViewport(page: Page) {
  await expectOverlayWithinViewport(page.locator('.ant-modal:visible').first(), page, 'modal');
}

export async function expectDrawerWithinViewport(page: Page) {
  await expectOverlayWithinViewport(page.locator('.ant-drawer-content-wrapper:visible').first(), page, 'drawer');
}

export async function expectHeaderContentAligned(page: Page) {
  const value = await page.evaluate(() => {
    const header = document.querySelector('.ant-pro-page-container-warp-page-header, .tm-page-container__header, .ant-page-header')?.getBoundingClientRect();
    const content = document.querySelector('.ant-pro-page-container-children-container, .tm-page-container__content, main .ant-card')?.getBoundingClientRect();
    if (!header || !content) return null;
    return { headerLeft: header.left, contentLeft: content.left, delta: Math.abs(header.left - content.left) };
  });
  if (!value) return;
  expect(value.delta, `header/content left delta ${JSON.stringify(value)}`).toBeLessThanOrEqual(4);
}

export async function expectRequestCount(tracker: NetworkWriteGuard, operation: string, count: number) {
  await tracker.expectRequestCount(operation, count);
}

export async function expectNoUnexpectedWrites(tracker: NetworkWriteGuard) {
  await tracker.expectNoUnexpectedWrites();
}

export async function expectNoFatalConsoleErrors(consoleGuard: ConsoleGuard) {
  await consoleGuard.expectNoFatalErrors();
}

async function expectOverlayWithinViewport(locator: Locator, page: Page, label: string) {
  await expect(locator, `${label} visible`).toBeVisible();
  const box = await locator.boundingBox();
  const viewport = page.viewportSize();
  expect(box, `${label} bounding box`).not.toBeNull();
  expect(viewport, 'viewport size').not.toBeNull();
  if (!box || !viewport) return;
  expect(box.x, `${label} left`).toBeGreaterThanOrEqual(0);
  expect(box.y, `${label} top`).toBeGreaterThanOrEqual(0);
  expect(box.x + box.width, `${label} right`).toBeLessThanOrEqual(viewport.width + 1);
  expect(box.y + box.height, `${label} bottom`).toBeLessThanOrEqual(viewport.height + 1);
}
