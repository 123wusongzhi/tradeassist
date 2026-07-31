import type { NormalizedProduct } from '../types.js';

export type BrowserCollectAdapter = {
  id: string;
  label: string;
  supports(url: string): boolean;
  collect(): Promise<NormalizedProduct>;
};
