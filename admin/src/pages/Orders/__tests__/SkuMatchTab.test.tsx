import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import OrderSkuMatchTab from '../SkuMatchTab';

const mocks = vi.hoisted(() => ({
  getOrderSKUMatches: vi.fn(),
  bindOrderItemSku: vi.fn(),
  getOrderItemSkuCandidates: vi.fn(),
}));
vi.mock('@/services/orders', () => ({
  getOrderSKUMatches: mocks.getOrderSKUMatches,
  matchOrderSKUs: vi.fn(),
  bindOrderItemSku: mocks.bindOrderItemSku,
}));
vi.mock('@/services/skuCandidates', () => ({ getOrderItemSkuCandidates: mocks.getOrderItemSkuCandidates }));
vi.mock('@/services/products', () => ({ searchProductSkus: vi.fn() }));

describe('OrderSkuMatchTab', () => {
  beforeEach(() => {
    mocks.getOrderSKUMatches.mockResolvedValue({
      items: [{ id: 'match-1', orderItemId: 'item-1', productTitle: '待绑定明细', matchStatus: 'unmatched' }],
    });
    mocks.getOrderItemSkuCandidates.mockResolvedValue({
      list: [{ productSkuId: 'sku-1', skuCode: 'SKU-1', confidence: 92, source: 'rule', reason: 'test' }],
    });
    mocks.bindOrderItemSku.mockResolvedValue({ item: {} });
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: (query: string) => ({
        matches: false, media: query, onchange: null,
        addListener: () => {}, removeListener: () => {}, addEventListener: () => {}, removeEventListener: () => {}, dispatchEvent: () => false,
      }),
    });
  });

  it('requires explicit confirmation before a manual SKU binding writes', async () => {
    const user = userEvent.setup();
    render(<OrderSkuMatchTab orderId="o1" onRefreshOrder={vi.fn().mockResolvedValue(undefined)} />);
    await user.click(await screen.findByRole('button', { name: '绑定 SKU' }));
    await user.click(await screen.findByRole('button', { name: '选择候选' }));
    await user.click(screen.getByRole('button', { name: '二次确认绑定' }));
    const confirmButton = document.querySelector<HTMLButtonElement>('.ant-modal-confirm .ant-btn-primary');
    expect(confirmButton).not.toBeNull();
    expect(mocks.bindOrderItemSku).not.toHaveBeenCalled();
    await user.click(confirmButton!);
    expect(mocks.bindOrderItemSku).toHaveBeenCalledTimes(1);
  });

  it('does not expose candidate binding controls to a readonly user', async () => {
    const user = userEvent.setup();
    render(<OrderSkuMatchTab orderId="o1" readOnly onRefreshOrder={vi.fn().mockResolvedValue(undefined)} />);

    await user.click(await screen.findByText('待绑定明细'));
    await screen.findByText('SKU-1');

    expect(screen.queryByRole('button', { name: '以此为候选绑定' })).toBeNull();
    expect(screen.queryByRole('button', { name: '绑定 SKU' })).toBeNull();
    expect(screen.queryByRole('button', { name: '二次确认绑定' })).toBeNull();
  });
});
