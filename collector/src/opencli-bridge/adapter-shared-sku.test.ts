import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

type AdapterModule = {
  buildFromSkuBase: (base: Record<string, unknown>) => {
    skuGroups: { name: string; options: { label: string; selected: boolean; disabled: boolean }[] }[];
    skus: Record<string, unknown>[];
  };
  mergeSku2InfoIntoSkus: (
    skus: Record<string, unknown>[],
    sku2info: Record<string, unknown>,
  ) => Record<string, unknown>[];
  applySkuPriceProbe: (
    skus: Record<string, unknown>[],
    probes: Record<string, unknown>,
  ) => Record<string, unknown>[];
};

async function loadShared(): Promise<AdapterModule> {
  const source = readFileSync(
    fileURLToPath(new URL('../../opencli-adapters/tmall/shared.js', import.meta.url)),
    'utf8',
  );
  const adapterURL = `data:text/javascript;base64,${Buffer.from(source).toString('base64')}`;
  return (await import(/* @vite-ignore */ adapterURL)) as AdapterModule;
}

describe('OpenCLI tmall adapter SKU 解析', () => {
  it('buildFromSkuBase 解析新版天猫 skuBase（propPath + skuId，无价格）', async () => {
    const shared = await loadShared();
    const base = {
      props: [
        {
          pid: '1627207',
          name: '颜色分类',
          values: [
            { vid: '40014758891', name: '奶白' },
            { vid: '40014758892', name: '桔色' },
          ],
        },
        {
          pid: '122216750',
          name: '适用尺码',
          values: [
            { vid: '28314', name: 'S' },
            { vid: '28315', name: 'M' },
          ],
        },
      ],
      skus: [
        { propPath: '1627207:40014758891;122216750:28314', skuId: '1001' },
        { propPath: '1627207:40014758891;122216750:28315', skuId: '1002' },
        { propPath: '1627207:40014758892;122216750:28315', skuId: '1003' },
      ],
    };
    const built = shared.buildFromSkuBase(base);
    expect(built.skuGroups).toHaveLength(2);
    expect(built.skus).toHaveLength(3);
    expect(built.skus[0].properties).toEqual({ 颜色分类: '奶白', 适用尺码: 'S' });
    expect(built.skus[0].skuCode).toBe('1001');
    expect(built.skus[0].price).toBeUndefined();
  });

  it('buildFromSkuBase 解析老版 skuMap（值内联 price/quantity）', async () => {
    const shared = await loadShared();
    const base = {
      props: [
        {
          pid: '1627207',
          name: '颜色分类',
          values: [{ vid: '1', name: '红色' }],
        },
      ],
      skuMap: {
        501: { propPath: '1627207:1', skuId: '501', price: '9.5', quantity: '20' },
      },
    };
    const built = shared.buildFromSkuBase(base);
    expect(built.skus).toHaveLength(1);
    expect(built.skus[0].properties).toEqual({ 颜色分类: '红色' });
    expect(built.skus[0].price).toBe(9.5);
    expect(built.skus[0].stock).toBe(20);
  });

  it('mergeSku2InfoIntoSkus 合并新版 skuCore.sku2info 库存与价格', async () => {
    const shared = await loadShared();
    const skus = [
      { properties: { 颜色: '红' }, skuCode: '1001', price: undefined, stock: undefined },
      { properties: { 颜色: '蓝' }, skuCode: '1002', price: undefined, stock: undefined },
      { properties: { 颜色: '绿' }, skuCode: '1003', price: undefined, stock: undefined },
    ];
    const sku2info = {
      1001: {
        quantity: '200',
        quantityText: '有货',
        logisticsTime: '预计明天发货',
        subPrice: { priceText: '8.6', priceTitle: '券后' },
        price: { priceText: '11', priceTitle: '优惠前' },
      },
      1002: { quantity: '7', quantityText: '即将售罄' },
      1003: { quantity: 0, quantityText: '无货' },
    };
    const merged = shared.mergeSku2InfoIntoSkus(skus, sku2info);
    expect(merged[0].stock).toBe(200);
    expect(merged[0].price).toBe(8.6);
    expect(merged[1].stock).toBe(7);
    expect(merged[1].price).toBeUndefined();
    expect(merged[2].stock).toBe(0);
  });

  it('applySkuPriceProbe 按 skuId 合并探测价格与库存（探测结果优先）', async () => {
    const shared = await loadShared();
    const skus = [
      { properties: { 颜色: '红' }, skuCode: '1001', price: 12, stock: 3 },
      { properties: { 颜色: '蓝' }, skuCode: '1002', price: undefined, stock: 5 },
      { properties: { 颜色: '绿' }, skuCode: '1003', price: undefined, stock: undefined },
    ];
    const probes = {
      1001: { priceText: '9.9', quantity: 99 },
      1002: { priceText: '8.6', originalPriceText: '11', quantity: 7 },
      9999: { error: 'no-context' },
    };
    const merged = shared.applySkuPriceProbe(skus, probes);
    expect(merged[0].price).toBe(9.9);
    expect(merged[0].stock).toBe(99);
    expect(merged[1].price).toBe(8.6);
    expect(merged[1].stock).toBe(7);
    expect(merged[2].price).toBeUndefined();
    expect(merged[2].stock).toBeUndefined();
  });
});
