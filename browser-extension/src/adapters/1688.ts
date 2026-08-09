import type { BrowserCollectAdapter, ProductSku } from './types.js';
import type { NormalizedProduct } from '../types.js';

/** Pure helpers (unit-tested). Page collect re-implements equivalents inside the body. */

function isSupported1688Host(hostname: string): boolean {
  const host = hostname.toLowerCase();
  return host === 'detail.1688.com' || host === 'm.1688.com';
}

export function isSupported1688URL(raw: string): boolean {
  return extract1688OfferId(raw) !== undefined;
}

export function extract1688OfferId(raw: string): string | undefined {
  try {
    const url = new URL(raw);
    if (url.protocol !== 'https:' || !isSupported1688Host(url.hostname)) return undefined;
    const m = url.pathname.match(/^\/offer\/(\d+)\.html\/?$/i);
    if (m?.[1]) return m[1];
    const q = url.searchParams.get('offerId') || url.searchParams.get('offerid') || url.searchParams.get('object_id');
    if (q && /^\d+$/.test(q)) return q;
  } catch {
    /* ignore */
  }
  return undefined;
}

export type PriceTier = {
  beginAmount: number;
  endAmount?: number;
  price: number;
};

export function parse1688Price(raw: unknown): number | undefined {
  if (typeof raw === 'number' && Number.isFinite(raw) && raw > 0 && raw < 1_000_000) return raw;
  const m = String(raw ?? '')
    .replace(/,/g, '')
    .match(/(\d+(?:\.\d{1,4})?)/);
  if (!m) return undefined;
  const n = Number(m[1]);
  return Number.isFinite(n) && n > 0 && n < 1_000_000 ? n : undefined;
}

export function parse1688Quantity(raw: unknown): number | undefined {
  if (raw === undefined || raw === null) return undefined;
  if (typeof raw === 'number') return Number.isFinite(raw) ? Math.max(0, Math.floor(raw)) : undefined;
  const t = String(raw).replace(/,/g, '').trim();
  if (!/^\d+(?:\.\d+)?$/.test(t)) return undefined;
  const n = Number(t);
  return Number.isFinite(n) ? Math.max(0, Math.floor(n)) : undefined;
}

function parse1688MoneyValue(raw: unknown): number | undefined {
  if (raw && typeof raw === 'object' && !Array.isArray(raw)) {
    const money = raw as Record<string, unknown>;
    for (const key of ['value', 'number', 'priceText', 'amount']) {
      const parsed = parse1688Price(money[key]);
      if (parsed !== undefined) return parsed;
    }
    return undefined;
  }
  return parse1688Price(raw);
}

/** Extract a SKU-specific price before callers consider an offer-level fallback. */
export function extract1688SkuPriceFromBucket(bucket: Record<string, unknown>): number | undefined {
  for (const key of [
    'price',
    'discountPrice',
    'salePrice',
    'priceDisplay',
    'consignPrice',
    'originPrice',
    'retailPrice',
    'priceMoney',
    'salePriceMoney',
  ]) {
    const parsed = parse1688MoneyValue(bucket[key]);
    if (parsed !== undefined) return parsed;
  }
  const promotion = bucket.promotionPrices;
  if (promotion && typeof promotion === 'object' && !Array.isArray(promotion)) {
    const prices = promotion as Record<string, unknown>;
    for (const key of ['finalPrice', 'salePriceMoney', 'salePrice', 'discountPrice']) {
      const parsed = parse1688MoneyValue(prices[key]);
      if (parsed !== undefined) return parsed;
    }
  }
  return undefined;
}

/** Read only explicitly named price fields; arbitrary numbers such as weights are not prices. */
export function extract1688OfferPriceFromUnknown(root: unknown): number | undefined {
  const priceKeys = [
    'price',
    'discountPrice',
    'priceDisplay',
    'salePrice',
    'offerPrice',
    'finalPrice',
    'consignPrice',
    'originPrice',
    'retailPrice',
    'mainPrice',
  ];
  const moneyKeys = ['priceMoney', 'salePriceMoney'];
  const walk = (value: unknown, depth: number): number | undefined => {
    if (depth > 14 || !value || typeof value !== 'object') return undefined;
    if (Array.isArray(value)) {
      for (const item of value) {
        const hit = walk(item, depth + 1);
        if (hit !== undefined) return hit;
      }
      return undefined;
    }
    const object = value as Record<string, unknown>;
    for (const key of priceKeys) {
      const hit = parse1688Price(object[key]);
      if (hit !== undefined) return hit;
    }
    for (const key of moneyKeys) {
      const money = object[key];
      if (!money || typeof money !== 'object') continue;
      const bucket = money as Record<string, unknown>;
      const hit = parse1688Price(bucket.value) ?? parse1688Price(bucket.number);
      if (hit !== undefined) return hit;
    }
    for (const nested of Object.values(object)) {
      if (!nested || typeof nested !== 'object') continue;
      const hit = walk(nested, depth + 1);
      if (hit !== undefined) return hit;
    }
    return undefined;
  };
  return walk(root, 0);
}

export function isValid1688SkuDimensionValue(raw: string, dimensionName: string): boolean {
  const value = raw.replace(/\s+/g, ' ').trim();
  const name = dimensionName.replace(/[:：\s]+$/u, '').trim();
  if (!value || value.length > 100 || value === name) return false;
  if (/^(颜色|尺寸|尺码|规格|库存|价格|数量|厚度)$/.test(value)) return false;
  if (/¥|￥/.test(value) || /库存\s*\d+/.test(value) || /^库存\d+/.test(value)) return false;
  if (/\d+(?:\.\d+)?\s*mm.*(?:¥|￥|库存)/i.test(value)) return false;
  return true;
}

export function parse1688SkuComboKey(
  raw: string,
  dimensionHints: Array<{ name: string; values: string[] }> = [],
): Record<string, string> {
  const trim = (value: unknown) => String(value ?? '').replace(/\s+/g, ' ').trim();
  const normalized = raw.replace(/&gt;/gi, '>').replace(/&amp;/gi, '&').trim();
  const properties: Record<string, string> = {};
  const namedSegments = normalized
    .split(/[;；#]/)
    .map((segment) => segment.trim())
    .filter(Boolean);
  for (const segment of namedSegments) {
    const match = segment.match(/^([^:：#\s]{1,24})\s*[#:：]\s*(.+)$/);
    if (!match) continue;
    const key = trim(match[1]);
    const value = trim(match[2]);
    if (key && value) properties[key] = value;
  }
  if (Object.keys(properties).length) return properties;

  const valueSegments = normalized
    .split(/>|»/)
    .map((segment) => trim(segment))
    .filter(Boolean);
  if (valueSegments.length === 2) {
    let names: [string, string];
    if (dimensionHints.length >= 2) {
      names = [dimensionHints[0]!.name, dimensionHints[1]!.name];
    } else if (/颜色|蓝|粉|黄|绿|米白|红|黑|白|灰|卡其|藏青/.test(valueSegments[0]!)) {
      names = ['颜色', '尺码'];
    } else if (/内长|尺码|尺寸|cm|码|mm|厚度/i.test(valueSegments[0]!)) {
      names = ['尺码', '颜色'];
    } else {
      names = ['颜色', '尺码'];
    }
    return { [names[0]]: valueSegments[0]!, [names[1]]: valueSegments[1]! };
  }
  if (valueSegments.length >= 4 && valueSegments.length % 2 === 0) {
    for (let index = 0; index < valueSegments.length; index += 2) {
      const key = valueSegments[index];
      const value = valueSegments[index + 1];
      if (key && value) properties[key] = value;
    }
    return properties;
  }
  if (dimensionHints.length === 1 && normalized) {
    return { [dimensionHints[0]!.name]: normalized };
  }
  return properties;
}

/** Extract wholesale ladder from common 1688 JSON shapes without inventing tiers. */
export function extractPriceTiersFromUnknown(root: unknown, depth = 0): PriceTier[] {
  if (depth > 16 || root == null) return [];
  if (Array.isArray(root)) {
    const tiers: PriceTier[] = [];
    for (const item of root) {
      if (!item || typeof item !== 'object') continue;
      const o = item as Record<string, unknown>;
      const price =
        parse1688Price(o.price) ??
        parse1688Price(o.value) ??
        parse1688Price(o.discountPrice) ??
        parse1688Price(o.offerPrice);
      const begin =
        parse1688Quantity(o.beginAmount) ??
        parse1688Quantity(o.startQuantity) ??
        parse1688Quantity(o.minQuantity) ??
        parse1688Quantity(o.begin) ??
        parse1688Quantity(o.start);
      if (price === undefined || begin === undefined) continue;
      const end =
        parse1688Quantity(o.endAmount) ?? parse1688Quantity(o.endQuantity) ?? parse1688Quantity(o.end);
      tiers.push(end !== undefined ? { beginAmount: begin, endAmount: end, price } : { beginAmount: begin, price });
    }
    if (tiers.length) return tiers.sort((a, b) => a.beginAmount - b.beginAmount);
    for (const item of root) {
      const hit = extractPriceTiersFromUnknown(item, depth + 1);
      if (hit.length) return hit;
    }
    return [];
  }
  if (typeof root !== 'object') return [];
  const o = root as Record<string, unknown>;
  for (const key of ['priceRange', 'priceRanges', 'priceTiers', 'priceRangeList', 'priceList', 'ladderPrice']) {
    if (key in o) {
      const hit = extractPriceTiersFromUnknown(o[key], depth + 1);
      if (hit.length) return hit;
    }
  }
  for (const v of Object.values(o)) {
    const hit = extractPriceTiersFromUnknown(v, depth + 1);
    if (hit.length) return hit;
  }
  return [];
}

export function extractMinOrderFromUnknown(root: unknown, depth = 0): number | undefined {
  if (depth > 14 || root == null) return undefined;
  if (typeof root === 'number') return undefined;
  if (typeof root === 'string') return undefined;
  if (Array.isArray(root)) {
    for (const i of root) {
      const hit = extractMinOrderFromUnknown(i, depth + 1);
      if (hit !== undefined) return hit;
    }
    return undefined;
  }
  const o = root as Record<string, unknown>;
  for (const key of ['minOrderQuantity', 'minOrder', 'moq', 'beginAmount', 'startQuantity', 'orderMinAmount']) {
    if (key in o) {
      const n = parse1688Quantity(o[key]);
      if (n !== undefined && n > 0) return n;
    }
  }
  for (const v of Object.values(o)) {
    const hit = extractMinOrderFromUnknown(v, depth + 1);
    if (hit !== undefined) return hit;
  }
  return undefined;
}

// Serialized by chrome.scripting.executeScript — every helper must live inside the body.
export async function collect1688Page(_options?: { maxPriceProbes?: number }): Promise<NormalizedProduct> {
  const sleep = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));
  const fail = (code: string, message: string): never => {
    throw new Error(`${code}: ${message}`);
  };
  const text = (el: Element | null | undefined) => (el?.textContent ?? '').replace(/\s+/g, ' ').trim();
  const parsePrice = (raw: unknown): number | undefined => {
    if (typeof raw === 'number' && Number.isFinite(raw) && raw > 0 && raw < 1_000_000) return raw;
    const m = String(raw ?? '')
      .replace(/,/g, '')
      .match(/(\d+(?:\.\d{1,4})?)/);
    if (!m) return undefined;
    const n = Number(m[1]);
    return Number.isFinite(n) && n > 0 && n < 1_000_000 ? n : undefined;
  };
  const parseQty = (raw: unknown): number | undefined => {
    if (raw === undefined || raw === null) return undefined;
    if (typeof raw === 'number') return Number.isFinite(raw) ? Math.max(0, Math.floor(raw)) : undefined;
    const t = String(raw).replace(/,/g, '').trim();
    if (!/^\d+(?:\.\d+)?$/.test(t)) return undefined;
    const n = Number(t);
    return Number.isFinite(n) ? Math.max(0, Math.floor(n)) : undefined;
  };
  const normalizeImage = (raw: string | null | undefined): string => {
    let value = String(raw ?? '').trim();
    if (!value || value.startsWith('data:')) return '';
    if (value.startsWith('//')) value = `https:${value}`;
    if (!/^https?:\/\//i.test(value)) return '';
    if (!/\.(jpg|jpeg|png|webp|gif)(\?|$)/i.test(value) && !/alicdn|cbu01|imgextra/i.test(value)) {
      if (/\/img\//i.test(value) && !/\.(jpg|jpeg|png|webp|gif)/i.test(value)) {
        value = `${value.replace(/[_.]+$/, '')}.jpg`;
      }
    }
    return value;
  };
  const uniqueImages = (values: Array<string | null | undefined>, max = 30) => {
    const seen = new Set<string>();
    const out: string[] = [];
    for (const raw of values) {
      const value = normalizeImage(raw);
      if (!value) continue;
      if (/(?:spaceball|\/s\.gif|icon|logo|avatar|ww-|wangwang|promise|badge)/i.test(value)) continue;
      const key = (value.split('?')[0] ?? value).toLowerCase();
      if (seen.has(key)) continue;
      seen.add(key);
      out.push(value);
      if (out.length >= max) break;
    }
    return out;
  };
  const trim = (s: unknown) => String(s ?? '').replace(/\s+/g, ' ').trim();
  const isValidSkuValue = (raw: string, dimensionName: string): boolean => {
    const value = trim(raw);
    const name = trim(dimensionName).replace(/[:：\s]+$/u, '');
    if (!value || value.length > 100 || value === name) return false;
    if (/^(颜色|尺寸|尺码|规格|库存|价格|数量|厚度)$/.test(value)) return false;
    if (/¥|￥/.test(value) || /库存\s*\d+/.test(value) || /^库存\d+/.test(value)) return false;
    if (/\d+(?:\.\d+)?\s*mm.*(?:¥|￥|库存)/i.test(value)) return false;
    return true;
  };

  const pageURL = location.href;
  const host = location.hostname.toLowerCase();
  if (location.protocol !== 'https:' || (host !== 'detail.1688.com' && host !== 'm.1688.com')) {
    fail('UNSUPPORTED_PAGE', '当前标签页不是 1688 商品详情页');
  }
  const offerFromPath = location.pathname.match(/^\/offer\/(\d+)\.html\/?$/i)?.[1];
  const offerFromQuery =
    new URL(pageURL).searchParams.get('offerId') ||
    new URL(pageURL).searchParams.get('offerid') ||
    new URL(pageURL).searchParams.get('object_id');
  const offerId = offerFromPath || (offerFromQuery && /^\d+$/.test(offerFromQuery) ? offerFromQuery : '');
  if (!offerId) {
    fail('UNSUPPORTED_PAGE', '当前页面不是 1688 offer 详情页');
  }
  const sourceUrl = `https://detail.1688.com/offer/${offerId}.html`;

  const originalScrollY = window.scrollY;
  const waitSelectors = [
    'h1.d-title',
    'h1',
    '[class*="title"]',
    '.detail-gallery',
    '[class*="gallery"]',
    '[class*="sku"]',
    '.obj-sku',
  ];
  const waitStarted = Date.now();
  while (!waitSelectors.some((sel) => document.querySelector(sel)) && Date.now() - waitStarted < 12_000) {
    await sleep(250);
  }

  // Lazy-load detail images with segmented scroll; restore later.
  const maxY = Math.max(document.body?.scrollHeight ?? 0, document.documentElement?.scrollHeight ?? 0);
  for (let y = 0; y < Math.min(maxY, 4500); y += 700) {
    window.scrollTo(0, y);
    await sleep(180);
  }
  window.scrollTo(0, originalScrollY);

  const bodyPeek = text(document.body).slice(0, 4000);
  const titlePeek = document.title.trim();
  const hasProductChrome = !!(
    document.querySelector('h1.d-title, h1, [class*="title"]') &&
    (document.querySelector('.detail-gallery, [class*="gallery"], [class*="offer-img"]') ||
      document.querySelector('[class*="sku"], [class*="obj-sku"]'))
  );

  if (/punish|x5secdata|captcha|_____tmd_____/i.test(pageURL) || /sec\.1688\.com/i.test(host)) {
    fail('VERIFY_REQUIRED', '页面触发安全验证或风控，请完成验证后重试');
  }
  if (!hasProductChrome && /安全验证|请完成验证|滑块验证|人机验证|访问过于频繁|拖动.*验证/i.test(bodyPeek)) {
    fail('VERIFY_REQUIRED', '页面需要安全验证，请完成验证后重试');
  }
  if (!hasProductChrome && (/请登录|账号登录|登录后查看/i.test(bodyPeek) || /passport\.1688\.com|login\.1688\.com/i.test(host))) {
    fail('LOGIN_REQUIRED', '需要登录 1688 后才能采集该商品，请登录后重试');
  }
  if (/商品不存在|该商品已下架|找不到该商品|offer不存在|页面不存在/i.test(`${titlePeek} ${bodyPeek}`)) {
    fail('PRODUCT_NOT_FOUND', '商品不存在或已下架');
  }

  type Tier = { beginAmount: number; endAmount?: number; price: number };
  type Dim = { name: string; values: string[] };

  const roots: unknown[] = [];
  const pushRoot = (v: unknown) => {
    if (v && typeof v === 'object') roots.push(v);
  };
  const win = window as unknown as Record<string, unknown>;
  for (const key of [
    'context',
    '__INIT_DATA',
    '__INITIAL_STATE__',
    'detailData',
    'offerDetailData',
    'iDetailConfig',
    'OFFER_DETAIL',
    'iDetailData',
  ]) {
    try {
      pushRoot(win[key]);
    } catch {
      /* ignore */
    }
  }
  for (const script of Array.from(document.scripts)) {
    const t = script.textContent ?? '';
    if (t.length < 120) continue;
    if (!/skuMap|skuModel|tradeModel|gallery|subject|priceRange|offerId|skuProps|saleProp/i.test(t)) continue;
    const candidates: string[] = [];
    if (t.trim().startsWith('{') || t.trim().startsWith('[')) candidates.push(t.trim().slice(0, 120_000));
    const assign = t.match(/(?:window\.)?(?:context|__INIT_DATA|detailData)\s*=\s*(\{[\s\S]*?\});?\s*(?:\n|$)/);
    if (assign?.[1]) candidates.push(assign[1].slice(0, 120_000));
    for (const c of candidates) {
      try {
        pushRoot(JSON.parse(c));
      } catch {
        /* ignore bad fragment */
      }
    }
    if (roots.length >= 12) break;
  }
  document.querySelectorAll('script[type="application/ld+json"]').forEach((el) => {
    try {
      pushRoot(JSON.parse(el.textContent || ''));
    } catch {
      /* ignore */
    }
  });

  const findResultData = (list: unknown[]): Record<string, unknown> | null => {
    const walk = (x: unknown, depth: number): Record<string, unknown> | null => {
      if (depth > 10 || !x || typeof x !== 'object') return null;
      const o = x as Record<string, unknown>;
      if (o.result && typeof o.result === 'object') {
        const data = (o.result as Record<string, unknown>).data;
        if (data && typeof data === 'object') return data as Record<string, unknown>;
      }
      if (o.gallery && typeof o.gallery === 'object' && (o.skuModel || o.tradeModel || o.tempModel)) {
        return o;
      }
      if (Array.isArray(x)) {
        for (const i of x) {
          const hit = walk(i, depth + 1);
          if (hit) return hit;
        }
        return null;
      }
      for (const v of Object.values(o)) {
        const hit = walk(v, depth + 1);
        if (hit) return hit;
      }
      return null;
    };
    for (const r of list) {
      const hit = walk(r, 0);
      if (hit) return hit;
    }
    return null;
  };

  const data = findResultData(roots);

  const extractTiers = (root: unknown, depth = 0): Tier[] => {
    if (depth > 16 || root == null) return [];
    if (Array.isArray(root)) {
      const tiers: Tier[] = [];
      for (const item of root) {
        if (!item || typeof item !== 'object') continue;
        const o = item as Record<string, unknown>;
        const price =
          parsePrice(o.price) ?? parsePrice(o.value) ?? parsePrice(o.discountPrice) ?? parsePrice(o.offerPrice);
        const begin =
          parseQty(o.beginAmount) ??
          parseQty(o.startQuantity) ??
          parseQty(o.minQuantity) ??
          parseQty(o.begin) ??
          parseQty(o.start);
        if (price === undefined || begin === undefined) continue;
        const end = parseQty(o.endAmount) ?? parseQty(o.endQuantity) ?? parseQty(o.end);
        tiers.push(end !== undefined ? { beginAmount: begin, endAmount: end, price } : { beginAmount: begin, price });
      }
      if (tiers.length) return tiers.sort((a, b) => a.beginAmount - b.beginAmount);
      for (const item of root) {
        const hit = extractTiers(item, depth + 1);
        if (hit.length) return hit;
      }
      return [];
    }
    if (typeof root !== 'object') return [];
    const o = root as Record<string, unknown>;
    for (const key of ['priceRange', 'priceRanges', 'priceTiers', 'priceRangeList', 'priceList', 'ladderPrice']) {
      if (key in o) {
        const hit = extractTiers(o[key], depth + 1);
        if (hit.length) return hit;
      }
    }
    for (const v of Object.values(o)) {
      const hit = extractTiers(v, depth + 1);
      if (hit.length) return hit;
    }
    return [];
  };

  const extractMinOrder = (root: unknown, depth = 0): number | undefined => {
    if (depth > 14 || !root || typeof root !== 'object') return undefined;
    if (Array.isArray(root)) {
      for (const i of root) {
        const hit = extractMinOrder(i, depth + 1);
        if (hit !== undefined) return hit;
      }
      return undefined;
    }
    const o = root as Record<string, unknown>;
    for (const key of ['minOrderQuantity', 'minOrder', 'moq', 'beginAmount', 'startQuantity', 'orderMinAmount']) {
      const n = parseQty(o[key]);
      if (n !== undefined && n > 0) return n;
    }
    for (const v of Object.values(o)) {
      const hit = extractMinOrder(v, depth + 1);
      if (hit !== undefined) return hit;
    }
    return undefined;
  };

  const extractUnit = (root: unknown, depth = 0): string | undefined => {
    if (depth > 12 || !root || typeof root !== 'object') return undefined;
    if (Array.isArray(root)) {
      for (const i of root) {
        const hit = extractUnit(i, depth + 1);
        if (hit) return hit;
      }
      return undefined;
    }
    const o = root as Record<string, unknown>;
    for (const key of ['unit', 'saleUnit', 'offerUnit', 'unitName', 'quantityUnit']) {
      const u = trim(o[key]);
      if (u && u.length <= 20) return u;
    }
    for (const v of Object.values(o)) {
      const hit = extractUnit(v, depth + 1);
      if (hit) return hit;
    }
    return undefined;
  };

  const extractTitle = (): string => {
    const preferredKeys = ['subject', 'offerTitle', 'productTitle', 'title'];
    let best = '';
    const walk = (x: unknown, depth: number) => {
      if (depth > 18 || !x || typeof x !== 'object') return;
      if (Array.isArray(x)) {
        for (const i of x) walk(i, depth + 1);
        return;
      }
      const o = x as Record<string, unknown>;
      for (const k of preferredKeys) {
        const t = trim(o[k]);
        if (t.length >= 4 && t.length <= 300 && !best) best = t;
      }
      for (const v of Object.values(o)) walk(v, depth + 1);
    };
    for (const r of roots) walk(r, 0);
    if (best) return best;
    for (const sel of ['h1.d-title', '.offer-title .title-text', '.title-content h1', 'h1[class*="title"]', 'h1']) {
      const t = text(document.querySelector(sel));
      if (t.length >= 4 && t.length <= 300) return t;
    }
    const og = document.querySelector('meta[property="og:title"]')?.getAttribute('content')?.trim() || '';
    if (og.length >= 4) return og;
    return titlePeek.replace(/[-_|].*1688.*$/i, '').trim();
  };

  const extractShopName = (): string | undefined => {
    const keys = ['companyName', 'sellerLoginId', 'loginId', 'shopName', 'memberName', 'winportName'];
    const walk = (x: unknown, depth: number): string | undefined => {
      if (depth > 14 || !x || typeof x !== 'object') return undefined;
      if (Array.isArray(x)) {
        for (const i of x) {
          const hit = walk(i, depth + 1);
          if (hit) return hit;
        }
        return undefined;
      }
      const o = x as Record<string, unknown>;
      for (const k of keys) {
        const v = trim(o[k]);
        if (v && v.length >= 2 && v.length <= 120) return v;
      }
      for (const v of Object.values(o)) {
        const hit = walk(v, depth + 1);
        if (hit) return hit;
      }
      return undefined;
    };
    for (const r of roots) {
      const hit = walk(r, 0);
      if (hit) return hit;
    }
    for (const sel of [
      '[class*="company-name"]',
      '[class*="shop-name"]',
      '.shop-company-name',
      'a[href*="winport"]',
    ]) {
      const t = text(document.querySelector(sel));
      if (t.length >= 2 && t.length <= 80) return t;
    }
    return undefined;
  };

  const collectDomImages = (selectors: string[]) => {
    const urls: string[] = [];
    for (const sel of selectors) {
      document.querySelectorAll(sel).forEach((node) => {
        const img = node as HTMLImageElement;
        const order = ['data-lazy-src', 'data-src', 'data-original', 'data-img', 'src'];
        for (const attr of order) {
          const v = img.getAttribute?.(attr) || (attr === 'src' ? img.src || img.currentSrc : '') || '';
          if (v && !v.startsWith('data:')) {
            urls.push(v);
            break;
          }
        }
      });
    }
    return urls;
  };

  const mainFromDom = collectDomImages([
    '.vertical-img img',
    '.detail-gallery-preview img',
    '.detail-gallery img',
    '[class*="offer-gallery"] img',
    '[class*="main-image"] img',
    '.swiper-slide img',
    '.obj-header-image img',
  ]);
  const detailFromDom = collectDomImages([
    '#offer-template-0 img',
    '.offer-description img',
    '.offer-detail img',
    '.detail-desc-module img',
    '[class*="detail-description"] img',
    '[module-title="商品详情"] img',
  ]);

  const mainFromData: string[] = [];
  const detailFromData: string[] = [];
  if (data) {
    const gallery = data.gallery;
    if (gallery && typeof gallery === 'object') {
      const fields = (gallery as Record<string, unknown>).fields;
      if (fields && typeof fields === 'object') {
        const f = fields as Record<string, unknown>;
        for (const list of [f.mainImage, f.offerImgList, f.imageList]) {
          if (!Array.isArray(list)) continue;
          for (const item of list) {
            if (typeof item === 'string') mainFromData.push(item);
          }
        }
      }
    }
    const walkDetail = (x: unknown, depth: number, keyHint: string) => {
      if (depth > 14 || x == null) return;
      if (typeof x === 'string') {
        if (!/detail|desc|content/i.test(keyHint)) return;
        const re = /(https?:\/\/[^\s"'<>]+)/gi;
        let m: RegExpExecArray | null;
        while ((m = re.exec(x))) {
          if (/\/img\/ibank\//i.test(m[1]) || /\.(jpg|jpeg|png|webp)/i.test(m[1])) detailFromData.push(m[1]);
        }
        return;
      }
      if (typeof x !== 'object') return;
      if (Array.isArray(x)) {
        for (const i of x) walkDetail(i, depth + 1, keyHint);
        return;
      }
      for (const [k, v] of Object.entries(x as Record<string, unknown>)) {
        walkDetail(v, depth + 1, `${keyHint}.${k}`);
      }
    };
    for (const key of Object.keys(data)) {
      if (/detail|desc|content|template/i.test(key)) walkDetail(data[key], 0, key);
    }
  }
  const ogImage = document.querySelector('meta[property="og:image"]')?.getAttribute('content') || '';
  const mainImages = uniqueImages([...mainFromData, ...mainFromDom, ogImage], 12);
  const descriptionImages = uniqueImages(
    [...detailFromData, ...detailFromDom].filter((u) => !mainImages.some((m) => (m.split('?')[0] ?? m) === (u.split('?')[0] ?? u))),
    30,
  );

  const attributes: Record<string, string | number | boolean> = {};
  const setAttr = (k: string, v: string | number | boolean) => {
    const key = trim(k).replace(/[:：\s]+$/u, '');
    if (!key || key.length > 40) return;
    if (attributes[key] !== undefined) return;
    if (typeof v === 'string') {
      const val = trim(v);
      if (!val || val.length > 260) return;
      attributes[key] = val;
    } else {
      attributes[key] = v;
    }
  };

  // DOM attributes
  for (const sel of [
    '.offer-attr-item',
    '.offer-attrprogram .de-feature-item',
    '[class*="param-table"] tr',
    '.obj-content-table tr',
    '.offer-params tr',
    '[module-title="商品属性"] tr',
  ]) {
    document.querySelectorAll(sel).forEach((node) => {
      const dt = node.querySelector('dt, th, [class*="label"], [class*="name"]');
      const dd = node.querySelector('dd, td:last-child, [class*="value"]');
      if (dt && dd) {
        setAttr(text(dt), text(dd));
        return;
      }
      const blob = text(node);
      const m = /^(.{2,30})[:：]\s*(.+)$/.exec(blob);
      if (m) setAttr(m[1], m[2]);
    });
  }
  // JSON attributes
  const walkAttrs = (x: unknown, depth: number) => {
    if (depth > 16 || !x || typeof x !== 'object') return;
    if (Array.isArray(x)) {
      for (const row of x) {
        if (!row || typeof row !== 'object') continue;
        const r = row as Record<string, unknown>;
        const k = trim(r.name ?? r.attributeName ?? r.fname ?? r.label ?? '');
        const v = trim(r.value ?? r.attributeValue ?? r.text ?? r.vname ?? '');
        if (k && v) setAttr(k, v);
      }
      return;
    }
    const o = x as Record<string, unknown>;
    for (const key of ['offerAttr', 'productAttribute', 'productAttributes', 'attributes']) {
      if (key in o) walkAttrs(o[key], depth + 1);
    }
    for (const v of Object.values(o)) {
      if (v && typeof v === 'object') walkAttrs(v, depth + 1);
    }
  };
  for (const r of roots.slice(0, 8)) walkAttrs(r, 0);

  const priceTiers = data ? extractTiers(data) : extractTiers(roots);
  let minOrder = data ? extractMinOrder(data) : extractMinOrder(roots);
  if (minOrder === undefined && priceTiers.length) minOrder = priceTiers[0]!.beginAmount;
  const unit = (data ? extractUnit(data) : extractUnit(roots)) || undefined;
  if (unit) setAttr('单位', unit);
  if (minOrder !== undefined) setAttr('最小起订量', minOrder);
  if (priceTiers.length) {
    setAttr(
      '阶梯价摘要',
      priceTiers.map((t) => `${t.beginAmount}${unit || '件'}起￥${t.price}`).join('；'),
    );
  }

  // Product-level price (first tier or explicit price fields — not inventing)
  let productPrice: number | undefined = priceTiers[0]?.price;
  const priceKeys = [
    'price',
    'discountPrice',
    'priceDisplay',
    'salePrice',
    'offerPrice',
    'finalPrice',
    'consignPrice',
    'originPrice',
    'retailPrice',
    'mainPrice',
  ];
  const moneyKeys = ['priceMoney', 'salePriceMoney'];
  const walkPrice = (x: unknown, depth: number): number | undefined => {
    if (depth > 14 || !x || typeof x !== 'object') return undefined;
    if (Array.isArray(x)) {
      for (const i of x) {
        const hit = walkPrice(i, depth + 1);
        if (hit !== undefined) return hit;
      }
      return undefined;
    }
    const o = x as Record<string, unknown>;
    for (const k of priceKeys) {
      const n = parsePrice(o[k]);
      if (n !== undefined) return n;
    }
    for (const k of moneyKeys) {
      const money = o[k];
      if (!money || typeof money !== 'object') continue;
      const bucket = money as Record<string, unknown>;
      const n = parsePrice(bucket.value) ?? parsePrice(bucket.number);
      if (n !== undefined) return n;
    }
    for (const v of Object.values(o)) {
      if (!v || typeof v !== 'object') continue;
      const hit = walkPrice(v, depth + 1);
      if (hit !== undefined) return hit;
    }
    return undefined;
  };
  if (productPrice === undefined && data) productPrice = walkPrice(data, 0);
  if (productPrice === undefined) productPrice = walkPrice(roots, 0);
  // DOM price texts
  if (productPrice === undefined) {
    const candidates: number[] = [];
    document
      .querySelectorAll('[class*="price"], [class*="obj-price"], [class*="price-range"], [class*="wholesale"]')
      .forEach((el) => {
        const t = text(el);
        if (!t || t.length > 200) return;
        for (const m of t.matchAll(/(?:¥|￥)\s*([\d,]+(?:\.\d{1,2})?)/g)) {
          const n = parsePrice(m[1]);
          if (n !== undefined) candidates.push(n);
        }
      });
    if (candidates.length) productPrice = Math.min(...candidates);
  }

  // SKU mining from skuMap / skuModel
  const skus: ProductSku[] = [];
  const skuMaps: Record<string, unknown>[] = [];
  const propArrays: unknown[] = [];
  const collectSkuStructures = (x: unknown, depth: number) => {
    if (depth > 22 || !x || typeof x !== 'object') return;
    if (Array.isArray(x)) {
      for (const i of x) collectSkuStructures(i, depth + 1);
      return;
    }
    const o = x as Record<string, unknown>;
    for (const [k, v] of Object.entries(o)) {
      if (
        (k === 'skuMap' || k === 'skuInfoMap' || k === 'skuPriceMap') &&
        v &&
        typeof v === 'object' &&
        !Array.isArray(v)
      ) {
        skuMaps.push(v as Record<string, unknown>);
      }
      if (k === 'skuModel' && v && typeof v === 'object') {
        const sm = v as Record<string, unknown>;
        if (sm.skuMap && typeof sm.skuMap === 'object') skuMaps.push(sm.skuMap as Record<string, unknown>);
        if (sm.skuInfoMap && typeof sm.skuInfoMap === 'object') skuMaps.push(sm.skuInfoMap as Record<string, unknown>);
      }
      if (/^sku_props$/i.test(k) || k === 'saleProp' || k === 'saleProps' || k === 'skuProps' || k === 'skuPropList') {
        propArrays.push(v);
      }
    }
    for (const v of Object.values(o)) collectSkuStructures(v, depth + 1);
  };
  for (const r of data ? [data, ...roots] : roots) collectSkuStructures(r, 0);

  const dims: Dim[] = [];
  const pushDim = (name: string, values: string[]) => {
    const n = trim(name).replace(/[:：\s]+$/u, '');
    const vs = [...new Set(values.map((v) => trim(v)).filter((v) => isValidSkuValue(v, n)))];
    if (!n || !vs.length) return;
    const existing = dims.find((d) => d.name === n);
    if (existing) {
      for (const v of vs) if (!existing.values.includes(v)) existing.values.push(v);
    } else dims.push({ name: n, values: vs });
  };
  for (const sp of propArrays) {
    if (!Array.isArray(sp)) continue;
    for (const row of sp) {
      if (!row || typeof row !== 'object') continue;
      const o = row as Record<string, unknown>;
      const name = trim(o.prop ?? o.name ?? o.fname ?? o.label ?? '');
      const valuesRaw = o.value ?? o.values ?? o.vlist ?? o.skus;
      const parts = Array.isArray(valuesRaw)
        ? valuesRaw
        : valuesRaw && typeof valuesRaw === 'object' && Array.isArray((valuesRaw as { list?: unknown[] }).list)
          ? ((valuesRaw as { list: unknown[] }).list ?? [])
          : [];
      const labels: string[] = [];
      for (const p of parts) {
        if (!p || typeof p !== 'object') continue;
        const po = p as Record<string, unknown>;
        const label = trim(po.name ?? po.value ?? po.text ?? po.vname ?? '');
        if (label) labels.push(label);
      }
      if (name && labels.length) pushDim(name, labels);
    }
  }
  // DOM sku dimensions
  const dimLabelRe = /^(颜色|尺码|尺寸|规格|型号|款式|容量|套餐|版本|内长|厚度)/;
  document
    .querySelectorAll(
      '[class*="sku-item-wrapper"], [class*="sku-selector"], [class*="obj-sku"], [class*="sale-prop"], [class*="spec-item"], .module-od-sku-selection',
    )
    .forEach((wrap) => {
      const labelNode = wrap.querySelector(
        '[class*="label"], [class*="title"], dt, .name, [class*="prop-name"], [class*="sku-item-label"]',
      );
      let label = text(labelNode).replace(/[:：\s]+$/u, '');
      if (!label || (!dimLabelRe.test(label) && label.length > 16)) return;
      const values: string[] = [];
      wrap
        .querySelectorAll(
          '[class*="sku-item"]:not([class*="sku-item-wrapper"]):not([class*="sku-item-label"]), [class*="select-item"], [class*="prop-item"], [class*="value-item"], button[class*="sku"]',
        )
        .forEach((el) => {
          const imgAlt = el.querySelector('img')?.getAttribute('alt')?.trim();
          const t = imgAlt || el.getAttribute('title')?.trim() || text(el);
          if (t && isValidSkuValue(t, label)) values.push(t);
        });
      if (values.length) pushDim(label, values);
    });

  const parseComboKey = (key: string): Record<string, string> => {
    const props: Record<string, string> = {};
    const normalized = key.replace(/&gt;/gi, '>').replace(/&amp;/gi, '&').trim();
    for (const part of normalized.split(/[;；#]/).map((segment) => segment.trim()).filter(Boolean)) {
      const m = part.match(/^([^:：#\s]{1,24})\s*[#:：]\s*(.+)$/);
      if (m) props[trim(m[1])] = trim(m[2]);
    }
    if (Object.keys(props).length) return props;
    const valueSegments = normalized
      .split(/>|»/)
      .map((segment) => trim(segment))
      .filter(Boolean);
    if (valueSegments.length === 2) {
      let names: [string, string];
      if (dims.length >= 2) names = [dims[0]!.name, dims[1]!.name];
      else if (/颜色|蓝|粉|黄|绿|米白|红|黑|白|灰|卡其|藏青/.test(valueSegments[0]!)) names = ['颜色', '尺码'];
      else if (/内长|尺码|尺寸|cm|码|mm|厚度/i.test(valueSegments[0]!)) names = ['尺码', '颜色'];
      else names = ['颜色', '尺码'];
      return { [names[0]]: valueSegments[0]!, [names[1]]: valueSegments[1]! };
    }
    if (valueSegments.length >= 4 && valueSegments.length % 2 === 0) {
      for (let index = 0; index < valueSegments.length; index += 2) {
        const name = valueSegments[index];
        const value = valueSegments[index + 1];
        if (name && value) props[name] = value;
      }
      return props;
    }
    if (dims.length === 1 && normalized) props[dims[0]!.name] = normalized;
    return props;
  };
  const parseMoneyValue = (raw: unknown): number | undefined => {
    if (raw && typeof raw === 'object' && !Array.isArray(raw)) {
      const money = raw as Record<string, unknown>;
      for (const key of ['value', 'number', 'priceText', 'amount']) {
        const parsed = parsePrice(money[key]);
        if (parsed !== undefined) return parsed;
      }
      return undefined;
    }
    return parsePrice(raw);
  };
  const skuPriceFromBucket = (bucket: Record<string, unknown>): number | undefined => {
    for (const key of [
      'price',
      'discountPrice',
      'salePrice',
      'priceDisplay',
      'consignPrice',
      'originPrice',
      'retailPrice',
      'priceMoney',
      'salePriceMoney',
    ]) {
      const parsed = parseMoneyValue(bucket[key]);
      if (parsed !== undefined) return parsed;
    }
    const promotion = bucket.promotionPrices;
    if (promotion && typeof promotion === 'object' && !Array.isArray(promotion)) {
      const prices = promotion as Record<string, unknown>;
      for (const key of ['finalPrice', 'salePriceMoney', 'salePrice', 'discountPrice']) {
        const parsed = parseMoneyValue(prices[key]);
        if (parsed !== undefined) return parsed;
      }
    }
    return undefined;
  };
  const skuStockFromBucket = (bucket: Record<string, unknown>): number | undefined =>
    parseQty(bucket.canBookCount) ??
    parseQty(bucket.amountOnSale) ??
    parseQty(bucket.stock) ??
    parseQty(bucket.quantity) ??
    parseQty(bucket.sellableQuantity) ??
    parseQty(bucket.saleCount) ??
    parseQty(bucket.amount) ??
    parseQty(bucket.bookCount);

  const mergedMap: Record<string, unknown> = Object.assign({}, ...skuMaps);
  const mapKeys = Object.keys(mergedMap);
  if (mapKeys.length) {
    for (const rawKey of mapKeys.slice(0, 120)) {
      const bucket = (mergedMap[rawKey] ?? {}) as Record<string, unknown>;
      let props = parseComboKey(rawKey);
      if (!Object.keys(props).length && rawKey.length > 0 && rawKey.length < 80) props = { 规格: trim(rawKey) };
      if (!Object.keys(props).length) continue;
      const price = skuPriceFromBucket(bucket) ?? productPrice;
      const stock = skuStockFromBucket(bucket);
      const imgRaw =
        (typeof bucket.pic === 'string' && bucket.pic) ||
        (typeof bucket.skuPicture === 'string' && bucket.skuPicture) ||
        (typeof bucket.skuPictureUrl === 'string' && bucket.skuPictureUrl) ||
        (typeof bucket.imageUrl === 'string' && bucket.imageUrl) ||
        '';
      const image = normalizeImage(imgRaw) || undefined;
      const skuCode = trim(bucket.specId ?? bucket.skuId ?? bucket.skuVid ?? bucket.skuCode ?? '');
      skus.push({
        skuCode: skuCode || undefined,
        properties: props,
        price,
        stock: stock !== undefined ? stock : undefined,
        image,
        raw: {
          source: 'skuMap',
          skuMapKey: rawKey.slice(0, 120),
          priceKnown: price !== undefined,
          stockKnown: stock !== undefined,
        },
      });
    }
  }

  // Cartesian fallback from dimensions when no skuMap
  if (!skus.length && dims.length) {
    let combos: Record<string, string>[] = [{}];
    for (const dim of dims.slice(0, 4)) {
      const next: Record<string, string>[] = [];
      for (const c of combos) {
        for (const v of dim.values.slice(0, 40)) next.push({ ...c, [dim.name]: v });
      }
      combos = next.slice(0, 120);
    }
    for (const properties of combos) {
      skus.push({
        properties,
        price: productPrice,
        raw: {
          source: 'cartesian-from-dimensions',
          priceKnown: productPrice !== undefined,
          stockKnown: false,
          warning: productPrice === undefined ? 'price_unknown' : undefined,
        },
      });
    }
  }

  // Single-SKU product without variants
  if (!skus.length) {
    skus.push({
      properties: { 规格: '默认' },
      price: productPrice,
      raw: {
        source: 'single-offer',
        priceKnown: productPrice !== undefined,
        stockKnown: false,
        warning: productPrice === undefined ? 'price_unknown' : undefined,
      },
    });
  }

  // DOM table enrichment for size/stock rows
  document.querySelectorAll('[class*="sku-table"] tr, [class*="table-sku"] tr').forEach((row) => {
    const blob = text(row);
    if (blob.length < 4 || blob.length > 400) return;
    const priceM = /(?:¥|￥)\s*([\d.]+)/.exec(blob);
    const stockM = /库存\s*(\d+)/.exec(blob);
    if (!priceM && !stockM) return;
    const label =
      (/(内长\d+[^¥￥]*)/.exec(blob)?.[1] ||
        /([\d.]+\s*mm)/i.exec(blob)?.[1] ||
        blob.split(/¥|￥/)[0] ||
        '').trim().slice(0, 80);
    if (!label) return;
    const existing = skus.find((s) => Object.values(s.properties ?? {}).some((v) => v.includes(label) || label.includes(v)));
    if (existing) {
      if (existing.price === undefined && priceM) existing.price = parsePrice(priceM[1]);
      if (existing.stock === undefined && stockM) existing.stock = parseQty(stockM[1]);
    }
  });

  const title = extractTitle();
  if (!title || title.length < 4) {
    fail('PARSE_FAILED', '无法读取商品标题，页面结构可能已变化或未加载完成');
  }
  if (!mainImages.length) {
    fail('PARSE_FAILED', '无法读取商品主图，请确认页面已完整加载且未触发验证');
  }

  const warnings: string[] = [];
  const missingPrice = skus.every((s) => s.price === undefined) && productPrice === undefined;
  const missingStock = skus.every((s) => s.stock === undefined);
  if (missingPrice) warnings.push('price_unknown');
  if (missingStock) warnings.push('stock_unknown');
  if (priceTiers.length) {
    warnings.push('price_tiers_preserved_in_raw');
  }
  if (!data) warnings.push('page_json_context_missing_used_dom_fallback');

  const shopName = extractShopName();
  if (shopName) setAttr('供应商', shopName);

  const priceMin = priceTiers.length
    ? Math.min(...priceTiers.map((t) => t.price))
    : productPrice ?? (skus.map((s) => s.price).filter((p): p is number => p !== undefined).length
        ? Math.min(...skus.map((s) => s.price!).filter((p) => p > 0))
        : undefined);
  const priceMax = priceTiers.length
    ? Math.max(...priceTiers.map((t) => t.price))
    : productPrice ?? (skus.map((s) => s.price).filter((p): p is number => p !== undefined).length
        ? Math.max(...skus.map((s) => s.price!).filter((p) => p > 0))
        : undefined);

  try {
    window.scrollTo(0, originalScrollY);
  } catch {
    /* ignore */
  }

  return {
    source: '1688',
    sourceUrl,
    title,
    currency: 'CNY',
    mainDescription: document.querySelector('meta[name="description"]')?.getAttribute('content')?.trim() || undefined,
    mainImages,
    descriptionImages,
    attributes,
    skus,
    raw: {
      provider: 'browser_extension',
      extractProvider: '1688',
      offerId,
      finalUrl: pageURL,
      shopName: shopName || undefined,
      productPrice: productPrice ?? undefined,
      priceMin: priceMin ?? undefined,
      priceMax: priceMax ?? undefined,
      priceRange:
        priceMin !== undefined && priceMax !== undefined
          ? priceMin === priceMax
            ? `¥${priceMin}`
            : `¥${priceMin} - ¥${priceMax}`
          : undefined,
      priceTiers,
      priceTiersWarning: priceTiers.length
        ? '阶梯价已完整保存在 raw.priceTiers；SKU.price 为首档或 SKU 单价，不是唯一成交价'
        : undefined,
      minOrderQuantity: minOrder ?? undefined,
      unit: unit || undefined,
      qualityWarnings: warnings,
      skuGroupCount: dims.length,
      skuCount: skus.length,
      stockStatus: missingStock ? 'unknown' : 'partial_or_known',
      priceStatus: missingPrice ? 'unknown' : priceTiers.length ? 'tiered' : 'known',
    },
  };
}

export const alibaba1688Adapter: BrowserCollectAdapter = {
  id: '1688',
  label: '1688',
  supports: isSupported1688URL,
  collect: collect1688Page,
};
