import { describe, expect, it } from 'vitest';
import { adapterForURL, supportedAdapters } from './registry.js';
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

  it('does not request access to unrelated pages', () => {
    expect(isSupportedTaobaoTmallURL('https://example.com/item/1')).toBe(false);
    expect(isSupportedTaobaoTmallURL('http://detail.tmall.com/item.htm?id=1')).toBe(false);
    expect(adapterForURL('chrome://extensions')).toBeUndefined();
  });

  it('exposes only packaged adapters', () => {
    expect(supportedAdapters()).toEqual([{ id: 'taobao_tmall', label: '淘宝 / 天猫' }]);
  });
});
