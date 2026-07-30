import { history } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import { appendSourceToUrl, parsePositiveInt, readQueryState, writeQueryState } from '../urlState';

const historyMock = vi.mocked(history);

describe('urlState helpers', () => {
  it('parses positive integers and falls back for invalid values', () => {
    expect(parsePositiveInt('3.8')).toBe(3);
    expect(parsePositiveInt('0', 7)).toBe(7);
    expect(parsePositiveInt('abc', 2)).toBe(2);
  });

  it('reads only requested query keys', () => {
    expect(readQueryState('?page=2&tab=publish&empty=', ['page', 'tab', 'empty'] as const)).toEqual({
      page: '2',
      tab: 'publish',
      empty: undefined,
    });
  });

  it('appends default navigation source without overwriting existing source', () => {
    expect(appendSourceToUrl('/products/p1')).toBe('/products/p1?source=dashboard');
    expect(appendSourceToUrl('/products/p1?source=taskcenter', 'manual')).toBe('/products/p1?source=taskcenter');
  });

  it('writes only allowlisted query keys', () => {
    historyMock.location.pathname = '/products';
    historyMock.location.search = '?page=1&keyword=old';

    writeQueryState({ page: 2, keyword: '', dangerous: 'x' }, { replace: true });

    expect(historyMock.replace).toHaveBeenCalledWith('/products?page=2');
    expect(historyMock.push).not.toHaveBeenCalled();
  });
});
