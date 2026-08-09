export type ProductSku = {
  id?: string;
  properties?: Record<string, string>;
  price?: number;
  /** 优惠前原价（新版天猫 SSR 的 price） */
  originalPrice?: number;
  stock?: number;
  /** 库存状态文案：有货 / 即将售罄 / 无货 等 */
  stockStatus?: string;
  /** 预计发货/送达时间 */
  logisticsTime?: string;
  skuCode?: string;
  image?: string;
  raw?: Record<string, unknown>;
};

export type BrowserCollectSource = 'taobao_tmall' | '1688';

export type NormalizedProduct = {
  source: BrowserCollectSource;
  sourceUrl: string;
  title: string;
  currency: 'CNY' | string;
  mainDescription?: string;
  mainImages: string[];
  descriptionImages: string[];
  attributes: Record<string, string | number | boolean>;
  skus: ProductSku[];
  raw: Record<string, unknown>;
};

export type PageCollectResult =
  | { ok: true; product: NormalizedProduct }
  | { ok: false; errorCode: string; message: string };

export type ExtensionDevice = {
  id: string;
  name: string;
  status: string;
  expiresAt: string;
  lastUsedAt?: string;
  createdAt: string;
};

export type CollectTask = {
  id: string;
  source: string;
  sourceUrl: string;
  status: string;
  resultProductId?: string;
};
