import { describe, expect, it } from 'vitest';
import {
  extract1688OfferPriceFromUnknown,
  extract1688SkuPriceFromBucket,
  extractMinOrderFromUnknown,
  extractPriceTiersFromUnknown,
  extract1688OfferId,
  isSupported1688URL,
  isValid1688SkuDimensionValue,
  parse1688SkuComboKey,
  parse1688Price,
  parse1688Quantity,
} from './1688.js';

describe('1688 pure helpers', () => {
  it('accepts offer detail URLs only', () => {
    expect(isSupported1688URL('https://detail.1688.com/offer/1054514049952.html')).toBe(true);
    expect(isSupported1688URL('https://detail.1688.com/offer/1054514049952.html?offerId=1054514049952')).toBe(
      true,
    );
    expect(isSupported1688URL('https://m.1688.com/offer/1.html')).toBe(true);
    expect(isSupported1688URL('https://www.1688.com/')).toBe(false);
    expect(isSupported1688URL('http://detail.1688.com/offer/1.html')).toBe(false);
    expect(isSupported1688URL('https://www.1688.com/offer/1.html')).toBe(false);
    expect(isSupported1688URL('https://detail.1688.com/offer/')).toBe(false);
    expect(isSupported1688URL('https://detail.1688.com/offer/not-a-number.html')).toBe(false);
    expect(isSupported1688URL('https://detail.1688.com/offer.html')).toBe(false);
    expect(isSupported1688URL('https://detail.tmall.com/item.htm?id=1')).toBe(false);
  });

  it('extracts offer id from path and query', () => {
    expect(extract1688OfferId('https://detail.1688.com/offer/1054514049952.html')).toBe('1054514049952');
    expect(extract1688OfferId('https://detail.1688.com/?offerId=1054514049952')).toBe('1054514049952');
    expect(extract1688OfferId('https://detail.1688.com/?offerId=not-a-number')).toBeUndefined();
  });

  it('prefers nested SKU prices over an offer-level fallback', () => {
    expect(extract1688SkuPriceFromBucket({ price: { value: '12.50' } })).toBe(12.5);
    expect(extract1688SkuPriceFromBucket({ price: { number: 13.8 } })).toBe(13.8);
    expect(
      extract1688SkuPriceFromBucket({ promotionPrices: { salePriceMoney: { value: '9.90' } } }),
    ).toBe(9.9);
    expect(extract1688SkuPriceFromBucket({ stock: 99 })).toBeUndefined();
  });

  it('parses price and quantity safely', () => {
    expect(parse1688Price('¥12.50')).toBe(12.5);
    expect(parse1688Price(0)).toBeUndefined();
    expect(parse1688Quantity('100')).toBe(100);
    expect(parse1688Quantity(-1)).toBe(0);
  });

  it('extracts ladder tiers without inventing data', () => {
    const tiers = extractPriceTiersFromUnknown({
      tradeModel: {
        priceRange: [
          { beginAmount: 2, price: 12.5 },
          { beginAmount: 100, price: 11.8 },
        ],
      },
    });
    expect(tiers).toEqual([
      { beginAmount: 2, price: 12.5 },
      { beginAmount: 100, price: 11.8 },
    ]);
    expect(extractPriceTiersFromUnknown({ price: 9.9 })).toEqual([]);
  });

  it('extracts min order quantity', () => {
    expect(extractMinOrderFromUnknown({ orderModel: { minOrderQuantity: 5 } })).toBe(5);
    expect(extractMinOrderFromUnknown({ moq: '3' })).toBe(3);
  });

  it('does not treat weights, dimensions, or title numbers as prices', () => {
    const noPrice = {
      title: '304 不锈钢测试商品',
      productPackInfo: {
        fields: {
          unitWeight: 39,
          netWeight: 304,
          length: 120,
        },
      },
    };
    expect(extract1688OfferPriceFromUnknown(noPrice)).toBeUndefined();
    expect(
      extract1688OfferPriceFromUnknown({
        ...noPrice,
        tradeModel: { fields: { price: 720 } },
      }),
    ).toBe(720);
  });

  it('parses common greater-than-delimited 1688 SKU keys into separate dimensions', () => {
    const dimensions = [
      { name: '颜色', values: ['蓝色【F106】'] },
      { name: '尺码', values: ['内长12【鞋底标12.5】'] },
    ];
    expect(parse1688SkuComboKey('蓝色【F106】>内长12【鞋底标12.5】', dimensions)).toEqual({
      颜色: '蓝色【F106】',
      尺码: '内长12【鞋底标12.5】',
    });
    expect(parse1688SkuComboKey('蓝色【F106】&gt;内长12【鞋底标12.5】', dimensions)).toEqual({
      颜色: '蓝色【F106】',
      尺码: '内长12【鞋底标12.5】',
    });
  });

  it('rejects DOM SKU values containing price or stock table noise', () => {
    expect(isValid1688SkuDimensionValue('黑色', '颜色')).toBe(true);
    expect(isValid1688SkuDimensionValue('M', '尺码')).toBe(true);
    expect(isValid1688SkuDimensionValue('颜色', '颜色')).toBe(false);
    expect(isValid1688SkuDimensionValue('库存299件', '颜色')).toBe(false);
    expect(isValid1688SkuDimensionValue('尺寸1.2mm ¥790 库存299件', '颜色')).toBe(false);
  });
});
