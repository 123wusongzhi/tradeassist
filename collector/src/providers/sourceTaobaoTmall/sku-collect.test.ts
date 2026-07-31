import { describe, expect, it } from 'vitest';
import { mergeSku2InfoIntoSkus } from './json-extract.js';
import { mergeSkuPriceProbe } from './sku-collect.js';

describe('mergeSku2InfoIntoSkus', () => {
  it('合并 skuCore.sku2info 库存（新版天猫 SSR 初始加载只有库存）', () => {
    const skus = [
      { properties: { 颜色: '红' }, skuCode: '1001' },
      { properties: { 颜色: '蓝' }, skuCode: '1002' },
    ];
    const sku2info = {
      1001: { quantity: '200', quantityText: '有货', logisticsTime: '预计明天发货' },
      1002: { quantity: 7, quantityText: '即将售罄' },
    };
    const merged = mergeSku2InfoIntoSkus(skus, sku2info);
    expect(merged[0].stock).toBe(200);
    expect(merged[0].raw).toMatchObject({
      sku2info: { quantity: '200', quantityText: '有货' },
    });
    expect(merged[1].stock).toBe(7);
  });

  it('sku2info 附带价格对象时解析券后价（subPrice 优先）', () => {
    const skus = [{ properties: { 型号: 'A' }, skuCode: '1001' }];
    const sku2info = {
      1001: {
        quantity: 20,
        subPrice: { priceText: '8.6' },
        price: { priceText: '11' },
      },
    };
    const merged = mergeSku2InfoIntoSkus(skus, sku2info);
    expect(merged[0].price).toBe(8.6);
    expect(merged[0].stock).toBe(20);
  });

  it('quantity 为 0 表示缺货，库存记录为 0', () => {
    const skus = [{ properties: { 型号: 'A' }, skuCode: '1001' }];
    const sku2info = { 1001: { quantity: 0, quantityText: '无货' } };
    const merged = mergeSku2InfoIntoSkus(skus, sku2info);
    expect(merged[0].stock).toBe(0);
  });
});

describe('mergeSkuPriceProbe', () => {
  it('按 skuCode 合并探测价格与库存，错误条目被忽略', () => {
    const skus = [
      { properties: { 型号: 'A' }, skuCode: '1001' },
      { properties: { 型号: 'B' }, skuCode: '1002' },
    ];
    const probes = {
      1001: { skuId: '1001', priceText: '9.5', originalPriceText: '12', quantity: 29 },
      1002: { skuId: '1002', error: 'no-context' },
    };
    const merged = mergeSkuPriceProbe(skus, probes);
    expect(merged[0].price).toBe(9.5);
    expect(merged[0].stock).toBe(29);
    expect(merged[0].raw).toMatchObject({
      skuPriceProbe: { priceText: '9.5', quantity: 29 },
    });
    expect(merged[1].price).toBeUndefined();
  });

  it('探测结果为每 SKU SSR 数据，优先于已有值', () => {
    const skus = [{ properties: { 型号: 'A' }, skuCode: '1001', price: 12, stock: 3 }];
    const probes = { 1001: { skuId: '1001', priceText: '8.6', quantity: 99 } };
    const merged = mergeSkuPriceProbe(skus, probes);
    expect(merged[0].price).toBe(8.6);
    expect(merged[0].stock).toBe(99);
  });

  it('探测到 quantity 0 时记录缺货库存', () => {
    const skus = [{ properties: { 型号: 'A' }, skuCode: '1001' }];
    const probes = { 1001: { skuId: '1001', priceText: '25.2', quantity: 0 } };
    const merged = mergeSkuPriceProbe(skus, probes);
    expect(merged[0].stock).toBe(0);
  });

  it('无有效探测结果时原样返回', () => {
    const skus = [{ properties: { 型号: 'A' }, skuCode: '1001' }];
    const probes = { 1001: { skuId: '1001', error: 'HTTP 429' } };
    expect(mergeSkuPriceProbe(skus, probes)).toBe(skus);
  });
});
