import { describe, expect, it } from 'vitest';
import {
  buildSkusFromTaobaoSkuBase,
  mergeSkuPriceProbeResults,
  parseTaobaoQuantity,
} from './taobao-tmall.js';

describe('buildSkusFromTaobaoSkuBase', () => {
  it('解析新版天猫 skuBase：规格组合 + skuId + sku2info 库存', () => {
    const skuBase = {
      props: [
        {
          pid: '1627207',
          name: '颜色分类',
          values: [{ vid: '1', name: '红' }, { vid: '2', name: '蓝' }],
        },
        {
          pid: '122216750',
          name: '适用尺码',
          values: [{ vid: '10', name: 'S' }],
        },
      ],
      skus: [
        { propPath: '1627207:1;122216750:10', skuId: '1001' },
        { propPath: '1627207:2;122216750:10', skuId: '1002' },
      ],
    };
    const sku2info = {
      1001: { quantity: '200', quantityText: '有货' },
      1002: { quantity: 0, quantityText: '无货' },
    };
    const built = buildSkusFromTaobaoSkuBase(skuBase, sku2info);
    expect(built.skuGroups).toHaveLength(2);
    expect(built.skus).toHaveLength(2);
    expect(built.skus[0]).toMatchObject({
      properties: { 颜色分类: '红', 适用尺码: 'S' },
      skuCode: '1001',
      stock: 200,
    });
    expect(built.skus[1]?.stock).toBe(0);
  });

  it('sku2info 附带 subPrice/price 时解析券后价', () => {
    const skuBase = {
      props: [{ pid: '1627207', name: '颜色分类', values: [{ vid: '1', name: '红' }] }],
      skus: [{ propPath: '1627207:1', skuId: '1001' }],
    };
    const sku2info = {
      1001: {
        quantity: 7,
        subPrice: { priceText: '8.6' },
        price: { priceText: '11' },
      },
    };
    const built = buildSkusFromTaobaoSkuBase(skuBase, sku2info);
    expect(built.skus[0]?.price).toBe(8.6);
    expect(built.skus[0]?.originalPrice).toBe(11);
    expect(built.skus[0]?.stock).toBe(7);
  });

  it('sku2info 提供库存状态与发货时间时透出到 SKU', () => {
    const skuBase = {
      props: [{ pid: '1627207', name: '颜色分类', values: [{ vid: '1', name: '红' }] }],
      skus: [{ propPath: '1627207:1', skuId: '1001' }],
    };
    const sku2info = {
      1001: {
        quantity: 7,
        quantityText: '即将售罄',
        logisticsTime: '预计明天发货',
      },
    };
    const built = buildSkusFromTaobaoSkuBase(skuBase, sku2info);
    expect(built.skus[0]?.stock).toBe(7);
    expect(built.skus[0]?.stockStatus).toBe('即将售罄');
    expect(built.skus[0]?.logisticsTime).toBe('预计明天发货');
  });

  it('规格组存在但无 sku 明细时生成笛卡尔组合兜底', () => {
    const skuBase = {
      props: [
        { pid: '1', name: '颜色', values: [{ vid: '1', name: '红' }, { vid: '2', name: '蓝' }] },
        { pid: '2', name: '尺码', values: [{ vid: '1', name: 'S' }, { vid: '2', name: 'M' }] },
      ],
      skus: [],
    };
    const built = buildSkusFromTaobaoSkuBase(skuBase, {});
    expect(built.skus).toHaveLength(4);
  });
});

describe('mergeSkuPriceProbeResults', () => {
  it('按 skuCode 合并探测价格与库存，错误条目忽略', () => {
    const skus = [
      { properties: { 颜色: '红' }, skuCode: '1001' },
      { properties: { 颜色: '蓝' }, skuCode: '1002' },
    ];
    const probes = {
      1001: { priceText: '9.5', originalPriceText: '12', quantity: 29, quantityText: '有货' },
      1002: { error: 'no-context' },
    };
    const merged = mergeSkuPriceProbeResults(skus, probes);
    expect(merged[0]?.price).toBe(9.5);
    expect(merged[0]?.originalPrice).toBe(12);
    expect(merged[0]?.stock).toBe(29);
    expect(merged[0]?.stockStatus).toBe('有货');
    expect(merged[0]?.raw).toMatchObject({ skuPriceProbe: { priceText: '9.5', quantity: 29 } });
    expect(merged[1]?.price).toBeUndefined();
  });

  it('探测结果优先于已有值（每 SKU SSR 数据更权威）', () => {
    const skus = [{ properties: { 颜色: '红' }, skuCode: '1001', price: 12, stock: 3 }];
    const probes = { 1001: { priceText: '8.6', quantity: 99 } };
    const merged = mergeSkuPriceProbeResults(skus, probes);
    expect(merged[0]?.price).toBe(8.6);
    expect(merged[0]?.stock).toBe(99);
  });

  it('quantity 0 表示缺货', () => {
    const skus = [{ properties: { 颜色: '红' }, skuCode: '1001' }];
    const merged = mergeSkuPriceProbeResults(skus, { 1001: { priceText: '25.2', quantity: 0 } });
    expect(merged[0]?.stock).toBe(0);
  });
});

describe('parseTaobaoQuantity', () => {
  it('保留 0 并拒绝非数字', () => {
    expect(parseTaobaoQuantity(0)).toBe(0);
    expect(parseTaobaoQuantity('200')).toBe(200);
    expect(parseTaobaoQuantity('有货')).toBeUndefined();
    expect(parseTaobaoQuantity(undefined)).toBeUndefined();
  });
});
