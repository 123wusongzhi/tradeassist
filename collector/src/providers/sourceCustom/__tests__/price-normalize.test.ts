import { describe, expect, it } from 'vitest';
import { fixMisplacedPriceInCurrency, normalizePriceText } from '../price-normalize.js';

describe('price normalization', () => {
  it('extracts numeric price and currency from localized text', () => {
    expect(normalizePriceText('￥1,299.50')).toEqual({ price: 1299.5, currency: 'CNY', priceText: '￥1,299.50' });
    expect(normalizePriceText('$12.99')).toEqual({ price: 12.99, currency: 'USD', priceText: '$12.99' });
    expect(normalizePriceText('€8')).toEqual({ price: 8, currency: 'EUR', priceText: '€8' });
  });

  it('returns only source text when no positive price exists', () => {
    expect(normalizePriceText('价格面议')).toEqual({ priceText: '价格面议' });
    expect(normalizePriceText('￥0')).toEqual({ priceText: '￥0', currency: 'CNY' });
    expect(normalizePriceText('')).toEqual({});
  });

  it('fixes price values mistakenly extracted into the currency field', () => {
    expect(fixMisplacedPriceInCurrency('￥99')).toEqual({ currency: 'CNY', price: 99, priceText: '￥99' });
    expect(fixMisplacedPriceInCurrency('usd')).toEqual({ currency: 'USD' });
    expect(fixMisplacedPriceInCurrency('平台币')).toEqual({ currency: '平台币' });
  });
});
