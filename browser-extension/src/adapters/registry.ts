import { taobaoTmallAdapter } from './taobao-tmall.js';
import type { BrowserCollectAdapter } from './types.js';

const adapters: BrowserCollectAdapter[] = [taobaoTmallAdapter];

export function adapterForURL(url: string): BrowserCollectAdapter | undefined {
  return adapters.find((adapter) => adapter.supports(url));
}

export function supportedAdapters(): ReadonlyArray<Pick<BrowserCollectAdapter, 'id' | 'label'>> {
  return adapters.map(({ id, label }) => ({ id, label }));
}
