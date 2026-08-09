import { describe, expect, it } from 'vitest';
import { adapterForURL, supportedAdapters } from './registry.js';
import { extract1688OfferId, isSupported1688URL } from './1688.js';
import { isSupportedTaobaoTmallURL } from './taobao-tmall.js';

describe('packaged browser collection adapters', () => {
  it('routes supported Taobao and Tmall product pages', () => {
    for (const url of [
      'https://detail.tmall.com/item.htm?id=1',
      'https://item.taobao.com/item.htm?id=2',
      'https://detail.tmall.hk/hk/item.htm?id=3',
    ]) {
      expect(isSupportedTaobaoTmallURL(url)).toBe(true);
      expect(adapterForURL(url)?.id).toBe('taobao_tmall');
    }
  });

  it('routes supported 1688 offer pages including acceptance sample', () => {
    const sample =
      'https://detail.1688.com/offer/1054514049952.html?topicCode=x&offerId=1054514049952&object_id=1054514049952';
    expect(isSupported1688URL(sample)).toBe(true);
    expect(extract1688OfferId(sample)).toBe('1054514049952');
    expect(adapterForURL(sample)?.id).toBe('1688');
    expect(isSupported1688URL('https://www.1688.com/')).toBe(false);
    expect(adapterForURL('https://www.1688.com/')).toBeUndefined();
  });

  it('does not request access to unrelated pages', () => {
    expect(isSupportedTaobaoTmallURL('https://example.com/item/1')).toBe(false);
    expect(isSupportedTaobaoTmallURL('http://detail.tmall.com/item.htm?id=1')).toBe(false);
    expect(adapterForURL('chrome://extensions')).toBeUndefined();
  });

  it('exposes packaged adapters', () => {
    expect(supportedAdapters()).toEqual([
      { id: 'taobao_tmall', label: '淘宝 / 天猫' },
      { id: '1688', label: '1688' },
    ]);
  });
});
