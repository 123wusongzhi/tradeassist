import { describe, expect, it } from 'vitest';
import { buildQualityScore, DESCRIPTION_IMAGES_EMPTY_HINT, SKU_LIMITATION_HINT } from '../quality-score.js';

describe('custom source quality score', () => {
  it('scores complete normalized products without duplicate hints', () => {
    const result = buildQualityScore({
      title: '测试商品标题',
      raw: { productPrice: 99 },
      mainImages: ['https://img.example.com/1.jpg', 'https://img.example.com/2.jpg'],
      descriptionImages: ['https://img.example.com/detail.jpg'],
      attributes: { material: 'cotton' },
      skus: [{ name: '默认' }],
    } as never);

    expect(result).toMatchObject({
      titleOk: true,
      priceOk: true,
      mainImagesOk: true,
      descriptionImagesOk: true,
      attributesOk: true,
      skuSupported: true,
      score: 100,
      hints: [],
    });
  });

  it('warns about missing dynamic fields and icon-polluted images', () => {
    const result = buildQualityScore({
      title: '测试商品标题',
      raw: {},
      mainImages: ['https://cdn.example.com/logo.png'],
      descriptionImages: [],
      attributes: {},
      skus: [],
    } as never);

    expect(result.priceOk).toBe(false);
    expect(result.mainImagesOk).toBe(false);
    expect(result.hints).toContain(DESCRIPTION_IMAGES_EMPTY_HINT);
    expect(result.hints).toContain(SKU_LIMITATION_HINT);
    expect(result.hints).toContain('主图可能含有图标或装饰图，建议检查 filters 或 selector。');
  });

  it('penalizes broad title selectors and surfaces diagnostic hints', () => {
    const result = buildQualityScore(
      { title: 'TradeMind', raw: { productPrice: 10 }, mainImages: ['a.jpg', 'b.jpg'], descriptionImages: ['d.jpg'], attributes: { a: 'b' }, skus: [{ name: '默认' }] } as never,
      { selector: 'title', suspectWrongTitle: true, hint: '标题可能来自站点名称' } as never,
    );

    expect(result.titleOk).toBe(false);
    expect(result.hints[0]).toBe('标题可能来自站点名称');
    expect(result.hints).toContain('当前标题位置过于宽泛，可能会抓到非商品标题，建议重新生成或手动调整。');
  });
});
