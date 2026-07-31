import type { NormalizedProduct } from '../types.js';
export type { ProductSku } from '../types.js';

export type BrowserCollectAdapter = {
  id: string;
  label: string;
  supports(url: string): boolean;
  collect(options?: { maxPriceProbes?: number }): Promise<NormalizedProduct>;
};
