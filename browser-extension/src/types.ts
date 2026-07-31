export type ProductSku = {
  id?: string;
  properties?: Record<string, string>;
  price?: number;
  stock?: number;
  skuCode?: string;
  image?: string;
  raw?: Record<string, unknown>;
};

export type NormalizedProduct = {
  source: 'taobao_tmall';
  sourceUrl: string;
  title: string;
  currency: 'CNY';
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
