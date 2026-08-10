import type { BrowserCollectAdapter, ProductSku } from './types.js';
import type { NormalizedProduct, ProductPackagingInfo, ProductPackagingRow } from '../types.js';

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

type TitleFallbacks = {
  domHeadings?: unknown[];
  ogTitle?: unknown;
  documentTitle?: unknown;
};

/** Extract a product title without coercing structured values to "[object Object]". */
export function extract1688TitleFromUnknown(
  roots: unknown[],
  fallbacks: TitleFallbacks = {},
): string | undefined {
  const normalizeCandidate = (raw: unknown): string | undefined => {
    if (typeof raw !== 'string') return undefined;
    const value = raw.replace(/\s+/g, ' ').trim();
    if (value.length < 4 || value.length > 300 || /^\d+$/.test(value)) return undefined;
    return value;
  };
  const preferredKeys = ['subject', 'offerTitle', 'productTitle', 'title'];
  let best: string | undefined;
  const walk = (value: unknown, depth: number): void => {
    if (best || depth > 18 || !value || typeof value !== 'object') return;
    if (Array.isArray(value)) {
      for (const item of value) walk(item, depth + 1);
      return;
    }
    const object = value as Record<string, unknown>;
    for (const key of preferredKeys) {
      const candidate = normalizeCandidate(object[key]);
      if (candidate) {
        best = candidate;
        return;
      }
    }
    for (const nested of Object.values(object)) walk(nested, depth + 1);
  };
  for (const root of roots) walk(root, 0);
  if (best) return best;

  for (const heading of fallbacks.domHeadings ?? []) {
    const candidate = normalizeCandidate(heading);
    if (candidate) return candidate;
  }
  const ogTitle = normalizeCandidate(fallbacks.ogTitle);
  if (ogTitle) return ogTitle;
  if (typeof fallbacks.documentTitle !== 'string') return undefined;
  return normalizeCandidate(fallbacks.documentTitle.replace(/[-_|].*1688.*$/i, ''));
}

export type DetailImageCandidate = {
  attributes?: Record<string, unknown>;
  currentSrc?: unknown;
  backgroundImage?: unknown;
  ancestorHint?: unknown;
  naturalWidth?: unknown;
  naturalHeight?: unknown;
};

type DetailImageInput = {
  baseUrl?: string;
  domCandidates?: DetailImageCandidate[];
  structuredRoots?: unknown[];
  mainImages?: string[];
  skuImages?: string[];
};

/** Extract only detail-scoped product images from offline DOM/JSON-shaped input. */
export function extract1688DescriptionImagesFromUnknown(input: DetailImageInput): string[] {
  const baseUrl = input.baseUrl ?? 'https://detail.1688.com/';
  const normalize = (raw: unknown): string | undefined => {
    if (typeof raw !== 'string') return undefined;
    const value = raw
      .trim()
      .replace(/&amp;/gi, '&')
      .replace(/\\u002f/gi, '/')
      .replace(/\\\//g, '/');
    if (!value || /[<>"']/.test(value) || /^(?:data|blob):/i.test(value)) return undefined;
    try {
      const url = value.startsWith('//') ? new URL(`https:${value}`) : new URL(value, baseUrl);
      if (!/^https?:$/.test(url.protocol)) return undefined;
      const href = url.href;
      if (!/\.(?:jpe?g|png|webp|gif)(?:[?#]|$)/i.test(href) && !/(?:alicdn|tbcdn|\/img\/ibank\/)/i.test(href)) {
        return undefined;
      }
      if (/(?:logo|icon|placeholder|loading|spacer|avatar|qr|ewm|wangwang|promise|badge|service|security|verify)/i.test(href)) {
        return undefined;
      }
      if (/[_-](?:16|20|24|30|32|40|48|50|60|64)x(?:16|20|24|30|32|40|48|50|60|64)(?:[x_.-]|$)/i.test(href)) {
        return undefined;
      }
      return href;
    } catch {
      return undefined;
    }
  };
  const canonical = (raw: string): string => (normalize(raw)?.split(/[?#]/)[0] ?? raw).toLowerCase();
  const excluded = new Set(
    [...(input.mainImages ?? []), ...(input.skuImages ?? [])]
      .map(canonical)
      .filter(Boolean),
  );
  const out: string[] = [];
  const seen = new Set<string>();
  const push = (raw: unknown): boolean => {
    const url = normalize(raw);
    if (!url) return false;
    const key = canonical(url);
    if (!key || excluded.has(key)) return false;
    if (!seen.has(key)) {
      seen.add(key);
      out.push(url);
    }
    return true;
  };
  const parseSrcset = (raw: unknown): string[] => {
    if (typeof raw !== 'string') return [];
    return raw.split(',').map((part) => part.trim().split(/\s+/)[0]).filter(Boolean).reverse();
  };
  const parseBackground = (raw: unknown): string[] => {
    if (typeof raw !== 'string') return [];
    return Array.from(raw.matchAll(/url\(\s*["']?([^"')]+)["']?\s*\)/gi), (match) => match[1] ?? '');
  };

  for (const candidate of input.domCandidates ?? []) {
    const hint = typeof candidate.ancestorHint === 'string' ? candidate.ancestorHint.toLowerCase() : '';
    if (/(?:main[-_ ]?image|offer[-_ ]?gallery|detail-gallery|sku|sale[-_ ]?prop|service|icon|badge|toolbar|footer)/i.test(hint)) {
      continue;
    }
    const width = typeof candidate.naturalWidth === 'number' ? candidate.naturalWidth : 0;
    const height = typeof candidate.naturalHeight === 'number' ? candidate.naturalHeight : 0;
    if (width > 0 && height > 0 && width < 72 && height < 72) continue;
    const attrs = candidate.attributes ?? {};
    const sourceCandidates: unknown[] = [];
    for (const name of ['data-lazy-src', 'data-src', 'data-original', 'data-img', 'data-zoom', 'src']) {
      if (typeof attrs[name] === 'string' && attrs[name]) sourceCandidates.push(attrs[name]);
    }
    sourceCandidates.push(candidate.currentSrc, ...parseSrcset(attrs.srcset));
    for (const source of sourceCandidates) {
      if (push(source)) break;
    }
    for (const background of parseBackground(candidate.backgroundImage ?? attrs.style)) push(background);
  }

  const walk = (
    value: unknown,
    depth: number,
    keyHint: string,
    visited: WeakSet<object>,
    budget: { remaining: number },
  ): void => {
    if (depth > 22 || value == null || budget.remaining-- <= 0) return;
    const hint = keyHint.toLowerCase();
    if (typeof value === 'string') {
      if (!/(?:detail|desc|description|content|wireless|template)/i.test(hint)) return;
      if (/(?:sku|spec|variant|saleprop|mainimage|gallery|album|thumb)/i.test(hint)) return;
      const direct = normalize(value);
      if (direct) push(direct);
      for (const match of value.matchAll(/(?:(?:https?:)?\\?\/\\?\/)[^\s"'<>]+/gi)) push(match[0]);
      return;
    }
    if (typeof value !== 'object' || visited.has(value)) return;
    visited.add(value);
    if (Array.isArray(value)) {
      for (const item of value) walk(item, depth + 1, keyHint, visited, budget);
      return;
    }
    for (const [key, nested] of Object.entries(value as Record<string, unknown>)) {
      walk(nested, depth + 1, `${keyHint}.${key}`, visited, budget);
    }
  };
  for (const root of input.structuredRoots ?? []) {
    walk(root, 0, '', new WeakSet<object>(), { remaining: 10_000 });
  }
  return out.slice(0, 30);
}

export type ProductDescriptionDomCandidate = {
  contextText?: unknown;
  sourceHint?: unknown;
  text?: unknown;
  html?: unknown;
};

type ProductDescriptionInput = {
  domCandidates?: ProductDescriptionDomCandidate[];
  structuredRoots?: unknown[];
  metaDescription?: unknown;
  productTitle?: unknown;
  nodeBudgetPerRoot?: number;
};

/** Extract trustworthy product copy without falling back to whole-page or commerce-table text. */
export function extract1688MainDescriptionFromUnknown(
  input: ProductDescriptionInput,
): string | undefined {
  const comparable = (raw: unknown): string =>
    typeof raw === 'string' ? raw.replace(/\u00a0/g, ' ').replace(/\s+/g, ' ').trim() : '';
  const decodeEntities = (raw: string): string =>
    raw
      .replace(/&nbsp;|&#160;/gi, ' ')
      .replace(/&amp;/gi, '&')
      .replace(/&lt;/gi, '<')
      .replace(/&gt;/gi, '>')
      .replace(/&quot;/gi, '"')
      .replace(/&#39;|&apos;/gi, "'")
      .replace(/&#(\d+);/g, (_match, digits: string) => {
        const codePoint = Number(digits);
        return Number.isInteger(codePoint) && codePoint > 0 && codePoint <= 0x10ffff
          ? String.fromCodePoint(codePoint)
          : '';
      });
  const sanitize = (raw: unknown): string => {
    if (typeof raw !== 'string') return '';
    const decoded = decodeEntities(raw);
    const withoutCommerceTables = decoded.replace(/<table\b[^>]*>[\s\S]*?<\/table\s*>/gi, (table) =>
      /产品规格|商品规格|产品尺寸|包装信息|商品件重尺|价格|库存|起批/i.test(table) ? '' : table,
    );
    return withoutCommerceTables
      .replace(/<(?:script|style|noscript)\b[^>]*>[\s\S]*?<\/(?:script|style|noscript)\s*>/gi, '')
      .replace(/<!--[\s\S]*?-->/g, '')
      .replace(/<(?:br|hr)\b[^>]*\/?\s*>/gi, '\n')
      .replace(/<\/(?:address|article|aside|blockquote|div|dl|fieldset|figcaption|figure|footer|form|h[1-6]|header|li|main|nav|ol|p|pre|section|table|tbody|td|tfoot|th|thead|tr|ul)\s*>/gi, '\n')
      .replace(/<[^>]+>/g, ' ')
      .split(/\r?\n/)
      .map((line) => line.replace(/\s+/g, ' ').trim())
      .filter(Boolean)
      .join('\n');
  };
  const explicitDescriptionContext = (raw: unknown): boolean =>
    /(?:商品|产品|图文)(?:详情|描述)|卖点|(?:offer|product|item|wireless)[-_ ]?(?:description|desc|detail)|(?:detail|desc)[-_ ]?(?:content|description|desc)|offer[-_ ]?template|desc[-_ ]?lazyload|selling[-_ ]?points?|highlights?/i.test(
      comparable(raw),
    );
  const excludedContext = (raw: unknown): boolean =>
    /(?:^|[._/\s-])(?:sku(?:model|map|props?)?|saleprop|sale[-_ ]?prop|variant|specification|spec|productPack(?:Info)?|package|packaging|logistic|freight|price(?:model|range)?|inventory|stock(?:model)?|quantity|trade(?:model)?|tier)(?:$|[._/\s-])|产品规格|商品规格|包装|物流|价格|库存|阶梯/i.test(
      comparable(raw),
    );
  const descriptionKey = (raw: string): boolean =>
    /^(?:description|desc|productDescription|offerDescription|itemDescription|detailDescription|detailDesc|detailContent|descContent|descriptionContent|wirelessDescription|wirelessDesc|mobileDescription|productDetailContent|sellingPoint|sellingPoints|sellingPointList|sellingFeature|sellingFeatures|productHighlights|highlights|featureDescription)$/i.test(
      raw,
    );
  const title = comparable(input.productTitle);
  const isAssetOrUrlFragment = (raw: string): boolean =>
    /^(?:(?:https?:)?\/\/\S+|(?:data|blob):\S+|(?:\.{0,2}\/)?[^\s<>]+\.(?:jpe?g|png|webp|gif|svg|avif)(?:[?#]\S*)?)$/i.test(
      raw,
    );
  const meaningful = (raw: string): boolean => {
    const value = comparable(raw);
    if (value.length < 12) return false;
    if (!/[\p{L}\p{N}]/u.test(value) || /^(?:https?:)?\/\//i.test(value)) return false;
    if (/^(?:商品详情|产品详情|图文详情|商品描述|产品描述|卖点|暂无(?:商品)?描述|详情加载中|查看全部|登录后查看)[。！!：:\s]*$/i.test(value)) {
      return false;
    }
    return !title || value.toLocaleLowerCase() !== title.toLocaleLowerCase();
  };
  const build = (rawCandidates: unknown[]): string | undefined => {
    const fragments: string[] = [];
    const seen = new Set<string>();
    for (const raw of rawCandidates) {
      const cleaned = sanitize(raw);
      if (!cleaned) continue;
      for (const fragment of cleaned.split('\n')) {
        const value = comparable(fragment);
        if (!value || /^(?:商品详情|产品详情|图文详情|商品描述|产品描述|卖点)[：:\s]*$/i.test(value)) continue;
        if (title && value.toLocaleLowerCase() === title.toLocaleLowerCase()) continue;
        if (isAssetOrUrlFragment(value)) continue;
        const key = value.toLocaleLowerCase();
        if (seen.has(key)) continue;
        seen.add(key);
        fragments.push(value);
      }
    }
    const result = fragments.join('\n').slice(0, 5000).trim();
    return meaningful(result) ? result : undefined;
  };

  const domDescription = build(
    (input.domCandidates ?? [])
      .filter((candidate) => {
        const context = `${comparable(candidate.contextText)} ${comparable(candidate.sourceHint)}`;
        return explicitDescriptionContext(context) && !excludedContext(context);
      })
      .flatMap((candidate) => [candidate.html, candidate.text]),
  );
  if (domDescription) return domDescription;

  const structuredCandidates: unknown[] = [];
  const walk = (
    value: unknown,
    depth: number,
    path: string,
    trusted: boolean,
    visited: WeakSet<object>,
    budget: { remaining: number },
  ): void => {
    if (depth > 20 || value == null || budget.remaining-- <= 0 || excludedContext(path)) return;
    if (typeof value === 'string') {
      if (trusted) structuredCandidates.push(value);
      return;
    }
    if (typeof value !== 'object' || visited.has(value)) return;
    visited.add(value);
    if (Array.isArray(value)) {
      for (const item of value) {
        if (budget.remaining <= 0) break;
        walk(item, depth + 1, path, trusted, visited, budget);
      }
      return;
    }
    for (const [key, nested] of Object.entries(value as Record<string, unknown>)) {
      if (budget.remaining <= 0) break;
      const nextPath = path ? `${path}.${key}` : key;
      if (excludedContext(nextPath)) continue;
      const nextTrusted =
        trusted ||
        descriptionKey(key) ||
        (/^(?:detail|productDetail|offerDetail)$/i.test(key) && typeof nested === 'string');
      if (nextTrusted && /^(?:id|url|link|image|imageUrl|imageList|video|status|success|code|count|total)$/i.test(key)) {
        continue;
      }
      walk(nested, depth + 1, nextPath, nextTrusted, visited, budget);
    }
  };
  const nodeBudget = Math.max(1, Math.min(10_000, input.nodeBudgetPerRoot ?? 10_000));
  for (const root of input.structuredRoots ?? []) {
    walk(root, 0, '', false, new WeakSet<object>(), { remaining: nodeBudget });
  }
  const structuredDescription = build(structuredCandidates);
  if (structuredDescription) return structuredDescription;
  return build([input.metaDescription]);
}

export type ProductAttributeDomCandidate = {
  contextText?: unknown;
  sourceHint?: unknown;
  name?: unknown;
  value?: unknown;
};

type ProductAttributeInput = {
  domCandidates?: ProductAttributeDomCandidate[];
  structuredRoots?: unknown[];
  nodeBudgetPerRoot?: number;
};

/** Extract product-level attributes only from explicit attribute/parameter semantics. */
export function extract1688ProductAttributesFromUnknown(
  input: ProductAttributeInput,
): Record<string, string | number | boolean> {
  const attributes: Record<string, string | number | boolean> = {};
  let attributeCount = 0;
  const normalizeText = (raw: unknown): string =>
    typeof raw === 'string' || typeof raw === 'number' || typeof raw === 'boolean'
      ? String(raw).replace(/\u00a0/g, ' ').replace(/\s+/g, ' ').trim()
      : '';
  const setAttribute = (rawName: unknown, rawValue: unknown): void => {
    const name = normalizeText(rawName).replace(/[:：\s]+$/u, '');
    const value = normalizeText(rawValue);
    if (
      !name ||
      name.length > 40 ||
      !value ||
      value.length > 260 ||
      attributes[name] !== undefined ||
      attributeCount >= 200
    ) {
      return;
    }
    attributes[name] = typeof rawValue === 'number' || typeof rawValue === 'boolean' ? rawValue : value;
    attributeCount += 1;
  };
  const isExplicitAttributeContext = (raw: unknown): boolean =>
    /(?:商品|产品)?(?:属性|参数)|(?:offer|product)[-_ ]?(?:attr|param|feature)|feature/i.test(
      normalizeText(raw),
    );
  const isExcludedContext = (raw: unknown): boolean =>
    /(?:sku|sale[-_ ]?prop|variant|specification|产品规格|商品规格|包装|packag|物流|logistic|freight|价格|price|库存|inventory|stock|quantity|阶梯|ladder|tier)/i.test(
      normalizeText(raw),
    );

  for (const candidate of input.domCandidates ?? []) {
    const labelledContext = normalizeText(candidate.contextText);
    const sourceHint = normalizeText(candidate.sourceHint);
    const requiresLabel = /param[-_ ]?table/i.test(sourceHint) && !/(?:offer|product)[-_ ]?param/i.test(sourceHint);
    if (requiresLabel && !isExplicitAttributeContext(labelledContext)) continue;
    const context = `${labelledContext} ${sourceHint}`;
    if (!isExplicitAttributeContext(context) || isExcludedContext(context)) continue;
    setAttribute(candidate.name, candidate.value);
  }

  const semanticContainerKey = /^(?:(?:offer|product)?(?:attrs?|attributes?|features?|params?|parameters?)(?:list)?)$/i;
  const strongNameKeys = ['attributeName', 'attrName', 'featureName', 'paramName', 'propertyName'] as const;
  const strongValueKeys = ['attributeValue', 'attrValue', 'featureValue', 'paramValue', 'propertyValue'] as const;
  const genericNameKeys = ['name', 'fname', 'label'] as const;
  const genericValueKeys = ['value', 'text', 'vname'] as const;
  const metadataKey = /^(?:id|key|type|code|index|order|sort|visible|required|unit|units|display|title|status|success|message|count|total|page|pagesize|hasmore)$/i;

  const readFirst = (object: Record<string, unknown>, keys: readonly string[]): unknown => {
    for (const key of keys) {
      if (object[key] !== undefined && object[key] !== null) return object[key];
    }
    return undefined;
  };
  const scanRoot = (root: unknown): void => {
    const visited = new WeakSet<object>();
    const budget = { remaining: Math.max(1, Math.min(input.nodeBudgetPerRoot ?? 12_000, 50_000)) };
    const walkContainer = (value: unknown, depth: number, path: string): void => {
      if (depth > 18 || value == null || budget.remaining-- <= 0 || isExcludedContext(path)) return;
      if (typeof value !== 'object') return;
      if (visited.has(value)) return;
      visited.add(value);
      if (Array.isArray(value)) {
        for (const row of value) walkContainer(row, depth + 1, path);
        return;
      }
      const object = value as Record<string, unknown>;
      const strongName = readFirst(object, strongNameKeys);
      const strongValue = readFirst(object, strongValueKeys) ?? object.value;
      const genericName = readFirst(object, genericNameKeys);
      const genericValue = readFirst(object, genericValueKeys);
      const rowName = strongName ?? genericName;
      const rowValue = strongValue ?? genericValue;
      if (rowName !== undefined && rowValue !== undefined) {
        setAttribute(rowName, rowValue);
      } else {
        const entries = Object.entries(object);
        if (entries.length <= 80) {
          for (const [key, nested] of entries) {
            if (metadataKey.test(key) || nested == null || typeof nested === 'object') continue;
            setAttribute(key, nested);
          }
        }
      }
      for (const [key, nested] of Object.entries(object)) {
        if (nested && typeof nested === 'object' && !isExcludedContext(key)) {
          walkContainer(nested, depth + 1, `${path}.${key}`);
        }
      }
    };
    const walk = (value: unknown, depth: number, path: string): void => {
      if (depth > 18 || value == null || budget.remaining-- <= 0 || isExcludedContext(path)) return;
      if (typeof value !== 'object') return;
      if (visited.has(value)) return;
      visited.add(value);
      if (Array.isArray(value)) {
        for (const item of value) walk(item, depth + 1, path);
        return;
      }
      const object = value as Record<string, unknown>;
      const strongName = readFirst(object, strongNameKeys);
      const strongValue = readFirst(object, strongValueKeys) ?? object.value;
      if (strongName !== undefined && strongValue !== undefined) setAttribute(strongName, strongValue);
      for (const [key, nested] of Object.entries(object)) {
        if (!nested || typeof nested !== 'object') continue;
        const nextPath = `${path}.${key}`;
        if (isExcludedContext(nextPath)) continue;
        if (semanticContainerKey.test(key)) walkContainer(nested, depth + 1, nextPath);
        else walk(nested, depth + 1, nextPath);
      }
    };
    walk(root, 0, 'root');
  };
  for (const root of input.structuredRoots ?? []) scanRoot(root);
  return attributes;
}

export type PackagingTableCandidate = {
  contextText?: unknown;
  headers?: unknown[];
  rows?: unknown[][];
};

export type ProductDimensionTableCandidate = PackagingTableCandidate;

export type ProductDimensionRow = {
  specification: string;
  productDimension: string;
  productName: string;
};

function normalizeProductDimensionCell(raw: unknown): string {
  return typeof raw === 'string' || typeof raw === 'number'
    ? String(raw).replace(/\u00a0/g, ' ').replace(/\s+/g, ' ').trim()
    : '';
}

function productDimensionHeaderRole(raw: unknown): keyof ProductDimensionRow | undefined {
  const value = normalizeProductDimensionCell(raw).replace(/\s+/g, '');
  if (/^(?:产品规格|商品规格|规格)$/u.test(value)) return 'specification';
  if (/^(?:产品尺寸|商品尺寸)$/u.test(value)) return 'productDimension';
  if (/^(?:品名|产品名称|商品名称)$/u.test(value)) return 'productName';
  return undefined;
}

function normalizeProductDimensionValue(raw: unknown): string {
  const value = normalizeProductDimensionCell(raw);
  if (!value || value.length > 120 || /^(?:—|–|-|--|暂无|无|N\/?A)$/iu.test(value)) return '';
  const normalized = value.replace(/\s*(?:[×*＊]|[xX])\s*/g, '×');
  const numberWithOptionalUnit = String.raw`\d+(?:\.\d+)?(?:\s*(?:mm|cm|m|毫米|厘米|米|寸|英寸))?`;
  const dimensionPattern = new RegExp(
    String.raw`^${numberWithOptionalUnit}(?:×${numberWithOptionalUnit}){1,3}(?:\s*[（(](?:mm|cm|m|毫米|厘米|米|寸|英寸)[）)])?$`,
    'iu',
  );
  return dimensionPattern.test(normalized) ? normalized : '';
}

/** Parse only an explicitly labelled 产品规格 / 产品尺寸 / 品名 table. */
export function extract1688ProductDimensionsFromUnknown(
  candidates: ProductDimensionTableCandidate[],
): ProductDimensionRow[] {
  const rows: ProductDimensionRow[] = [];
  const seen = new Set<string>();
  for (const candidate of candidates) {
    const context = normalizeProductDimensionCell(candidate.contextText);
    if (/(?:包装信息|商品件重尺|包装件重尺)/u.test(context)) continue;
    const indexes = new Map<keyof ProductDimensionRow, number>();
    for (const [index, header] of (candidate.headers ?? []).entries()) {
      const role = productDimensionHeaderRole(header);
      if (role != null && !indexes.has(role)) indexes.set(role, index);
    }
    const required: Array<keyof ProductDimensionRow> = ['specification', 'productDimension', 'productName'];
    if (!required.every((role) => indexes.has(role))) continue;
    for (const cells of candidate.rows ?? []) {
      if (!Array.isArray(cells)) continue;
      const specification = normalizeProductDimensionCell(cells[indexes.get('specification')!]);
      const productDimension = normalizeProductDimensionValue(cells[indexes.get('productDimension')!]);
      const productName = normalizeProductDimensionCell(cells[indexes.get('productName')!]);
      if (
        !specification ||
        specification.length > 200 ||
        /^(?:产品规格|商品规格|规格)$/u.test(specification) ||
        !productDimension ||
        productName.length > 200 ||
        /^(?:品名|产品名称|商品名称)$/u.test(productName)
      ) {
        continue;
      }
      const row = { specification, productDimension, productName };
      const key = JSON.stringify(row);
      if (!seen.has(key)) {
        seen.add(key);
        rows.push(row);
      }
      if (rows.length >= 200) return rows;
    }
  }
  return rows;
}

/** Add 产品尺寸 only when a source row resolves to exactly one SKU by an exact value match. */
export function merge1688ProductDimensionsIntoSkus(
  skus: ProductSku[],
  rows: ProductDimensionRow[],
): ProductSku[] {
  const normalizedSkuValues = skus.map((sku) =>
    Object.entries(sku.properties ?? {})
      .filter(([key]) => key !== '产品尺寸')
      .map(([, value]) => normalizeProductDimensionCell(value)),
  );
  const assignments = new Map<number, Set<string>>();
  for (const row of rows) {
    const specification = normalizeProductDimensionCell(row.specification);
    const productName = normalizeProductDimensionCell(row.productName);
    const productDimension = normalizeProductDimensionValue(row.productDimension);
    if (!specification || !productDimension) continue;
    const specificationMatches = normalizedSkuValues
      .map((values, index) => (values.includes(specification) ? index : -1))
      .filter((index) => index >= 0);
    let skuIndex = specificationMatches.length === 1 ? specificationMatches[0] : undefined;
    if (specificationMatches.length === 0 && productName) {
      const productNameMatches = normalizedSkuValues
        .map((values, index) => (values.includes(productName) ? index : -1))
        .filter((index) => index >= 0);
      if (productNameMatches.length === 1) skuIndex = productNameMatches[0];
    }
    if (skuIndex === undefined) continue;
    const dimensions = assignments.get(skuIndex) ?? new Set<string>();
    dimensions.add(productDimension);
    assignments.set(skuIndex, dimensions);
  }
  return skus.map((sku, index) => {
    if (sku.properties?.['产品尺寸']) return sku;
    const dimensions = assignments.get(index);
    if (!dimensions || dimensions.size !== 1) return sku;
    return { ...sku, properties: { ...(sku.properties ?? {}), 产品尺寸: [...dimensions][0]! } };
  });
}

export type SkuImageTableCandidate = {
  headers?: unknown[];
  rows?: Array<{ cells?: unknown[]; image?: unknown; imageCandidates?: unknown[] }>;
};

export type SkuImageRow = {
  specification: string;
  image: string;
};

function normalize1688SkuImage(raw: unknown): string {
  if (typeof raw !== 'string') return '';
  const value = raw
    .trim()
    .replace(/&amp;/gi, '&')
    .replace(/\\u002f/gi, '/')
    .replace(/\\\//g, '/');
  if (!value || /^(?:data|blob):/i.test(value) || /[<>"']/.test(value)) return '';
  try {
    const url = value.startsWith('//') ? new URL(`https:${value}`) : new URL(value, 'https://detail.1688.com/');
    if (!/^https?:$/.test(url.protocol)) return '';
    const href = url.href;
    if (!/\.(?:jpe?g|png|webp|gif)(?:[?#]|$)/i.test(href) && !/(?:alicdn|tbcdn|\/img\/ibank\/)/i.test(href)) {
      return '';
    }
    if (/(?:spaceball|\/s\.gif|placeholder|loading|spacer|icon|logo|avatar|wangwang|promise|badge)/i.test(href)) {
      return '';
    }
    return href;
  } catch {
    return '';
  }
}

function read1688SkuImageFromObject(value: unknown): string {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return '';
  const object = value as Record<string, unknown>;
  for (const key of [
    'image',
    'img',
    'pic',
    'picUrl',
    'imageUrl',
    'imageURL',
    'skuPicture',
    'skuPictureUrl',
  ]) {
    const direct = normalize1688SkuImage(object[key]);
    if (direct) return direct;
    const nested = object[key];
    if (nested && typeof nested === 'object' && !Array.isArray(nested)) {
      const nestedObject = nested as Record<string, unknown>;
      for (const nestedKey of ['url', 'src', 'value']) {
        const image = normalize1688SkuImage(nestedObject[nestedKey]);
        if (image) return image;
      }
    }
  }
  return '';
}

/** Read the complete SKU value-to-image metadata carried by skuProps / saleProps JSON. */
export function extract1688StructuredSkuImageRowsFromUnknown(
  roots: unknown[],
  maxRows = 200,
): SkuImageRow[] {
  const limit = Number.isFinite(maxRows) ? Math.max(1, Math.min(200, Math.floor(maxRows))) : 200;
  const rows: SkuImageRow[] = [];
  const seen = new Set<string>();
  const scanPropArray = (
    value: unknown,
    visited: WeakSet<object>,
    budget: { remaining: number },
  ): void => {
    if (!Array.isArray(value) || budget.remaining-- <= 0 || visited.has(value)) return;
    visited.add(value);
    for (const group of value) {
      if (budget.remaining-- <= 0) return;
      if (!group || typeof group !== 'object' || Array.isArray(group)) continue;
      if (visited.has(group)) continue;
      visited.add(group);
      const object = group as Record<string, unknown>;
      const valuesRaw = object.value ?? object.values ?? object.vlist ?? object.skus ?? object.list;
      const parts = Array.isArray(valuesRaw)
        ? valuesRaw
        : valuesRaw && typeof valuesRaw === 'object' && Array.isArray((valuesRaw as { list?: unknown[] }).list)
          ? (valuesRaw as { list: unknown[] }).list
          : [];
      if (parts !== value && budget.remaining-- <= 0) return;
      for (const part of parts) {
        if (budget.remaining-- <= 0) return;
        if (!part || typeof part !== 'object' || Array.isArray(part)) continue;
        if (visited.has(part)) continue;
        visited.add(part);
        const partObject = part as Record<string, unknown>;
        const specification = normalizeProductDimensionCell(
          partObject.name ?? partObject.value ?? partObject.text ?? partObject.vname ?? partObject.label,
        );
        const image = read1688SkuImageFromObject(partObject);
        if (!specification || specification.length > 200 || !image) continue;
        const key = `${specification}\u0000${image.split(/[?#]/)[0]!.toLowerCase()}`;
        if (!seen.has(key)) {
          seen.add(key);
          rows.push({ specification, image });
        }
        if (rows.length >= limit) return;
      }
    }
  };
  const containerKey = /^(?:sku_props|skuProps|skuPropList|saleProp|saleProps)$/i;
  for (const root of roots) {
    const visited = new WeakSet<object>();
    const budget = { remaining: 10_000 };
    const walk = (value: unknown, depth: number): void => {
      if (rows.length >= limit || depth > 22 || value == null || budget.remaining-- <= 0) return;
      if (typeof value !== 'object') return;
      if (visited.has(value)) return;
      visited.add(value);
      if (Array.isArray(value)) {
        for (const item of value) {
          if (budget.remaining <= 0) break;
          walk(item, depth + 1);
        }
        return;
      }
      const object = value as Record<string, unknown>;
      for (const key in object) {
        if (!Object.prototype.hasOwnProperty.call(object, key)) continue;
        if (budget.remaining-- <= 0) break;
        const nested = object[key];
        if (containerKey.test(key)) {
          scanPropArray(nested, visited, budget);
        } else if (nested && typeof nested === 'object') {
          walk(nested, depth + 1);
        }
      }
    };
    walk(root, 0);
    if (rows.length >= limit) break;
  }
  return rows;
}

/** Read row thumbnails only from an explicit 1688 SKU purchase table. */
export function extract1688SkuImageRowsFromUnknown(candidates: SkuImageTableCandidate[]): SkuImageRow[] {
  const rows: SkuImageRow[] = [];
  const seen = new Set<string>();
  for (const candidate of candidates) {
    const headers = (candidate.headers ?? []).map(normalizeProductDimensionCell);
    const specificationIndex = headers.findIndex((header) => /^(?:产品规格|商品规格|规格)$/u.test(header));
    const isPurchaseTable = headers.some((header) =>
      /(?:价格.*库存|库存.*价格|进货数量|采购数量|订购数量)/u.test(header.replace(/\s+/g, '')),
    );
    if (specificationIndex < 0 || !isPurchaseTable) continue;
    for (const candidateRow of candidate.rows ?? []) {
      const specification = normalizeProductDimensionCell(candidateRow.cells?.[specificationIndex]);
      const image = [candidateRow.image, ...(candidateRow.imageCandidates ?? [])]
        .map(normalize1688SkuImage)
        .find(Boolean) ?? '';
      if (!specification || specification.length > 200 || !image) continue;
      const key = `${specification}\u0000${image.split(/[?#]/)[0]!.toLowerCase()}`;
      if (!seen.has(key)) {
        seen.add(key);
        rows.push({ specification, image });
      }
      if (rows.length >= 200) return rows;
    }
  }
  return rows;
}

/** Fill a missing SKU image only when one table-row specification resolves to exactly one SKU. */
export function merge1688SkuImagesIntoSkus(
  skus: ProductSku[],
  structuredRows: SkuImageRow[],
  domFallbackRows: SkuImageRow[] = [],
): ProductSku[] {
  const normalizedSkuValues = skus.map((sku) =>
    Object.entries(sku.properties ?? {})
      .filter(([key]) => key !== '产品尺寸')
      .map(([, value]) => normalizeProductDimensionCell(value)),
  );
  const collectAssignments = (rows: SkuImageRow[]) => {
    const assignments = new Map<number, Map<string, string>>();
    for (const row of rows) {
      const specification = normalizeProductDimensionCell(row.specification);
      const image = normalize1688SkuImage(row.image);
      if (!specification || !image) continue;
      const matches = normalizedSkuValues
        .map((values, index) => (values.includes(specification) ? index : -1))
        .filter((index) => index >= 0);
      if (matches.length !== 1) continue;
      const images = assignments.get(matches[0]!) ?? new Map<string, string>();
      images.set(image.split(/[?#]/)[0]!.toLowerCase(), image);
      assignments.set(matches[0]!, images);
    }
    return assignments;
  };
  const structuredAssignments = collectAssignments(structuredRows);
  const domFallbackAssignments = collectAssignments(domFallbackRows);
  return skus.map((sku, index) => {
    if (normalize1688SkuImage(sku.image)) return sku;
    const structuredImages = structuredAssignments.get(index);
    const images = structuredImages ?? domFallbackAssignments.get(index);
    if (!images || images.size !== 1) return sku;
    return { ...sku, image: [...images.values()][0]! };
  });
}

export type SkuPropertyTableCandidate = {
  contextText?: unknown;
  headers?: unknown[];
  rows?: Array<{ cells?: unknown[] }>;
};

export type SkuPropertyRow = {
  specification: string;
  properties: Record<string, string>;
};

function normalize1688SkuPropertyHeader(raw: unknown): string {
  const value = normalizeProductDimensionCell(raw)
    .replace(/[（]/g, '(')
    .replace(/[）]/g, ')')
    .replace(/\s+/g, '')
    .replace(/[:：]+$/u, '');
  if (
    !value ||
    value.length > 40 ||
    /[<>\u0000-\u001F\u007F]/u.test(value) ||
    /^(?:__proto__|prototype|constructor)$/iu.test(value)
  ) {
    return '';
  }
  return value;
}

function is1688SkuPurchaseOperationHeader(header: string): boolean {
  return /(?:价格|库存|进货数量|采购数量|订购数量|购买数量|起订数量|操作|加购|数量)/u.test(header);
}

/** Read descriptive columns only from an explicit 1688 SKU purchase table. */
export function extract1688SkuPropertyRowsFromUnknown(
  candidates: SkuPropertyTableCandidate[],
): SkuPropertyRow[] {
  const rows: SkuPropertyRow[] = [];
  const seen = new Set<string>();
  let inspectedRows = 0;
  for (const candidate of candidates) {
    const context = normalizeProductDimensionCell(candidate.contextText);
    if (/(?:包装信息|商品件重尺|包装件重尺)/u.test(context)) continue;
    const rawHeaders = Array.isArray(candidate.headers) ? candidate.headers : [];
    if (rawHeaders.length < 3 || rawHeaders.length > 12) continue;
    const headers = rawHeaders.map(normalize1688SkuPropertyHeader);
    if (headers.some((header) => !header) || new Set(headers).size !== headers.length) continue;
    const specificationIndex = headers.findIndex((header) => /^(?:产品规格|商品规格|规格)$/u.test(header));
    const isPurchaseTable = headers.some((header) =>
      /(?:价格.*库存|库存.*价格|进货数量|采购数量|订购数量|购买数量|起订数量)/u.test(header),
    );
    if (specificationIndex < 0 || !isPurchaseTable) continue;
    const descriptiveIndexes = headers
      .map((header, index) => (!is1688SkuPurchaseOperationHeader(header) ? index : -1))
      .filter((index) => index >= 0);
    if (descriptiveIndexes.length < 2) continue;
    const candidateRows = Array.isArray(candidate.rows) ? candidate.rows : [];
    for (const candidateRow of candidateRows) {
      if (inspectedRows >= 200) return rows;
      inspectedRows += 1;
      if (!candidateRow || typeof candidateRow !== 'object' || Array.isArray(candidateRow)) continue;
      const cells = candidateRow.cells;
      if (!Array.isArray(cells) || cells.length > 12) continue;
      const specification = normalizeProductDimensionCell(cells[specificationIndex]);
      if (!specification || specification.length > 200) continue;
      const properties: Record<string, string> = {};
      let valid = true;
      for (const index of descriptiveIndexes) {
        if (index === specificationIndex) {
          properties[headers[index]!] = specification;
          continue;
        }
        const rawValue = cells[index];
        const value = normalizeProductDimensionCell(cells[index]);
        if (!value || /^(?:—|–|-|--|暂无|无|N\/?A)$/iu.test(value)) {
          if (rawValue == null || typeof rawValue === 'string' || typeof rawValue === 'number') continue;
          valid = false;
          break;
        }
        if (value.length > 200 || /[<>\u0000-\u0008\u000B\u000C\u000E-\u001F]/u.test(value)) {
          valid = false;
          break;
        }
        properties[headers[index]!] = value;
      }
      if (!valid || Object.keys(properties).length < 2) continue;
      const key = `${specification}\u0000${JSON.stringify(properties)}`;
      if (!seen.has(key)) {
        seen.add(key);
        rows.push({ specification, properties });
      }
    }
  }
  return rows;
}

/** Merge one unambiguous purchase-table row by an exact existing SKU property match. */
export function merge1688SkuPropertiesIntoSkus(
  skus: ProductSku[],
  rows: SkuPropertyRow[],
): ProductSku[] {
  const normalizedSkuValues = skus.map((sku) =>
    Object.entries(sku.properties ?? {})
      .filter(([name]) => name !== '产品尺寸')
      .map(([, value]) => normalizeProductDimensionCell(value)),
  );
  const assignments = new Map<number, Map<string, Record<string, string>>>();
  for (const row of rows.slice(0, 200)) {
    const specification = normalizeProductDimensionCell(row.specification);
    if (!specification) continue;
    const matches = normalizedSkuValues
      .map((values, index) => (values.includes(specification) ? index : -1))
      .filter((index) => index >= 0);
    if (matches.length !== 1) continue;
    const normalizedEntries =
      Object.entries(row.properties ?? {})
        .map(([name, value]) => [normalize1688SkuPropertyHeader(name), normalizeProductDimensionCell(value)] as const)
        .filter(([name, value]) => name && value && value.length <= 200 && !is1688SkuPurchaseOperationHeader(name));
    if (new Set(normalizedEntries.map(([name]) => name)).size !== normalizedEntries.length) continue;
    const specificationEntries = normalizedEntries.filter(([name]) => /^(?:产品规格|商品规格|规格)$/u.test(name));
    if (specificationEntries.length !== 1 || specificationEntries[0]![1] !== specification) continue;
    const properties = Object.fromEntries(normalizedEntries);
    if (Object.keys(properties).length < 2) continue;
    const signature = JSON.stringify(Object.entries(properties).sort(([left], [right]) => left.localeCompare(right)));
    const skuAssignments = assignments.get(matches[0]!) ?? new Map<string, Record<string, string>>();
    skuAssignments.set(signature, properties);
    assignments.set(matches[0]!, skuAssignments);
  }
  return skus.map((sku, index) => {
    const skuAssignments = assignments.get(index);
    if (!skuAssignments || skuAssignments.size !== 1) return sku;
    const properties = [...skuAssignments.values()][0]!;
    const merged = { ...(sku.properties ?? {}) };
    const existingByNormalizedName = new Map<string, Array<{ rawName: string; value: string }>>();
    for (const [rawName, rawValue] of Object.entries(merged)) {
      const normalizedName = normalize1688SkuPropertyHeader(rawName);
      if (!normalizedName) continue;
      const existing = existingByNormalizedName.get(normalizedName) ?? [];
      existing.push({ rawName, value: normalizeProductDimensionCell(rawValue) });
      existingByNormalizedName.set(normalizedName, existing);
    }
    for (const [name, value] of Object.entries(properties)) {
      const existing = existingByNormalizedName.get(name) ?? [];
      if (existing.length > 1 || (existing.length === 1 && existing[0]!.value !== value)) return sku;
    }
    const sourceSpecificationName = Object.keys(properties).find((name) => /^(?:产品规格|商品规格)$/u.test(name));
    const genericSpecifications = existingByNormalizedName.get('规格') ?? [];
    if (sourceSpecificationName && genericSpecifications.length) {
      if (
        genericSpecifications.length !== 1 ||
        genericSpecifications[0]!.value !== properties[sourceSpecificationName]
      ) return sku;
      delete merged[genericSpecifications[0]!.rawName];
    }
    for (const [name, value] of Object.entries(properties)) {
      const existing = existingByNormalizedName.get(name);
      if (existing?.length === 1 && existing[0]!.rawName !== name) delete merged[existing[0]!.rawName];
      merged[name] = value;
    }
    return { ...sku, properties: merged };
  });
}

const PACKAGING_HEADER_ROLES = {
  specification: /^(?:产品规格|商品规格|规格)$/u,
  lengthCm: /^长(?:度)?(?:\(cm\)|cm)$/iu,
  widthCm: /^宽(?:度)?(?:\(cm\)|cm)$/iu,
  heightCm: /^高(?:度)?(?:\(cm\)|cm)$/iu,
  volumeCm3: /^体积(?:\(cm(?:³|3)\)|cm(?:³|3))$/iu,
  weightG: /^重量(?:\(g\)|g)$/iu,
} as const;

function normalizePackagingText(raw: unknown): string {
  return typeof raw === 'string' || typeof raw === 'number'
    ? String(raw).replace(/\u00a0/g, ' ').replace(/\s+/g, ' ').trim()
    : '';
}

function packagingHeaderRole(raw: unknown): keyof ProductPackagingRow | undefined {
  const value = normalizePackagingText(raw)
    .replace(/[（]/g, '(')
    .replace(/[）]/g, ')')
    .replace(/\s+/g, '');
  for (const [role, pattern] of Object.entries(PACKAGING_HEADER_ROLES)) {
    if (pattern.test(value)) return role as keyof ProductPackagingRow;
  }
  return undefined;
}

function parsePackagingMeasurement(raw: unknown): number | null | undefined {
  const value = normalizePackagingText(raw);
  if (!value || /^(?:—|–|-|--|暂无|无|N\/?A)$/iu.test(value)) return null;
  if (!/^(?:\d{1,3}(?:,\d{3})+|\d+)(?:\.\d+)?$/.test(value)) return undefined;
  const parsed = Number(value.replace(/,/g, ''));
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : undefined;
}

/** Parse only a labelled 1688 商品件重尺 table; never infer or repair source values. */
export function extract1688PackagingFromUnknown(
  candidates: PackagingTableCandidate[],
): ProductPackagingInfo | undefined {
  const rows: ProductPackagingRow[] = [];
  const seen = new Set<string>();
  for (const candidate of candidates) {
    const context = normalizePackagingText(candidate.contextText);
    if (!/(?:包装信息|商品件重尺|包装件重尺)/u.test(context)) continue;
    const headers = candidate.headers ?? [];
    const indexes = new Map<keyof ProductPackagingRow, number>();
    headers.forEach((header, index) => {
      const role = packagingHeaderRole(header);
      if (role != null && !indexes.has(role)) indexes.set(role, index);
    });
    const required: Array<keyof ProductPackagingRow> = [
      'specification',
      'lengthCm',
      'widthCm',
      'heightCm',
      'volumeCm3',
      'weightG',
    ];
    if (!required.every((role) => indexes.has(role))) continue;
    for (const cells of candidate.rows ?? []) {
      if (!Array.isArray(cells)) continue;
      const specification = normalizePackagingText(cells[indexes.get('specification')!]);
      if (!specification || specification.length > 200 || /^(?:产品规格|商品规格|规格)$/u.test(specification)) continue;
      const lengthCm = parsePackagingMeasurement(cells[indexes.get('lengthCm')!]);
      const widthCm = parsePackagingMeasurement(cells[indexes.get('widthCm')!]);
      const heightCm = parsePackagingMeasurement(cells[indexes.get('heightCm')!]);
      const volumeCm3 = parsePackagingMeasurement(cells[indexes.get('volumeCm3')!]);
      const weightG = parsePackagingMeasurement(cells[indexes.get('weightG')!]);
      if ([lengthCm, widthCm, heightCm, volumeCm3, weightG].some((value) => value === undefined)) continue;
      const row: ProductPackagingRow = {
        specification,
        lengthCm: lengthCm!,
        widthCm: widthCm!,
        heightCm: heightCm!,
        volumeCm3: volumeCm3!,
        weightG: weightG!,
      };
      const key = JSON.stringify(row);
      if (!seen.has(key)) {
        seen.add(key);
        rows.push(row);
      }
      if (rows.length >= 200) break;
    }
    if (rows.length >= 200) break;
  }
  return rows.length ? { rows } : undefined;
}

export type PriceTier = {
  beginAmount: number;
  endAmount?: number;
  price: number;
};

const PRICE_TIER_CONTAINER_KEYS = [
  'priceRange',
  'priceRanges',
  'priceTiers',
  'priceRangeList',
  'priceList',
  'ladderPrice',
] as const;

const isPriceTierContainerKey = (key: string): boolean =>
  PRICE_TIER_CONTAINER_KEYS.some((candidate) => candidate.toLowerCase() === key.toLowerCase());

const normalize1688NodeBudget = (raw: number): number =>
  Number.isFinite(raw) ? Math.min(50_000, Math.max(1, Math.floor(raw))) : 10_000;

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
export function extract1688OfferPriceFromUnknown(root: unknown, maxNodesPerRoot = 10_000): number | undefined {
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
  const rootBudget = normalize1688NodeBudget(maxNodesPerRoot);
  const roots = Array.isArray(root) ? root : [root];
  for (const candidateRoot of roots) {
    const visited = new WeakSet<object>();
    let remaining = rootBudget;
    const walk = (value: unknown, depth: number): number | undefined => {
      if (depth > 14 || !value || typeof value !== 'object' || remaining <= 0) return undefined;
      const reference = value as object;
      if (visited.has(reference)) return undefined;
      visited.add(reference);
      remaining -= 1;
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
      for (const [key, nested] of Object.entries(object)) {
        if (isPriceTierContainerKey(key) || !nested || typeof nested !== 'object') continue;
        const hit = walk(nested, depth + 1);
        if (hit !== undefined) return hit;
      }
      return undefined;
    };
    const hit = walk(candidateRoot, 0);
    if (hit !== undefined) return hit;
  }
  return undefined;
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

/** Extract and validate one complete wholesale ladder from explicitly named 1688 JSON containers. */
export function extractPriceTiersFromUnknown(root: unknown, maxNodesPerRoot = 10_000): PriceTier[] {
  const beginKeys = ['beginAmount', 'startQuantity', 'minQuantity', 'begin', 'start'];
  const endKeys = ['endAmount', 'endQuantity', 'end'];
  const priceKeys = ['price', 'value', 'discountPrice', 'offerPrice'];
  const hasOwn = (value: Record<string, unknown>, key: string) => Object.prototype.hasOwnProperty.call(value, key);
  const parsePositiveInteger = (raw: unknown): number | undefined => {
    if (typeof raw === 'number') return Number.isSafeInteger(raw) && raw > 0 ? raw : undefined;
    const text = String(raw ?? '').replace(/,/g, '').trim();
    if (!/^\d+$/.test(text)) return undefined;
    const parsed = Number(text);
    return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : undefined;
  };
  const parseTierPrice = (raw: unknown): number | undefined => {
    if (typeof raw === 'number') {
      return Number.isFinite(raw) && raw > 0 && raw < 1_000_000 ? raw : undefined;
    }
    const match = String(raw ?? '')
      .replace(/,/g, '')
      .trim()
      .match(/^(?:[¥￥]\s*)?(\d+(?:\.\d{1,4})?)\s*(?:元)?$/);
    if (!match) return undefined;
    const parsed = Number(match[1]);
    return Number.isFinite(parsed) && parsed > 0 && parsed < 1_000_000 ? parsed : undefined;
  };
  const readConsistent = (
    row: Record<string, unknown>,
    keys: string[],
    parser: (raw: unknown) => number | undefined,
    emptyIsAbsent = false,
  ): { present: boolean; valid: boolean; value?: number } => {
    let present = false;
    let parsedValue: number | undefined;
    for (const key of keys) {
      if (!hasOwn(row, key)) continue;
      const rawValue = row[key];
      if (emptyIsAbsent && (rawValue == null || (typeof rawValue === 'string' && !rawValue.trim()))) continue;
      present = true;
      const parsed = parser(rawValue);
      if (parsed === undefined || (parsedValue !== undefined && parsedValue !== parsed)) {
        return { present: true, valid: false };
      }
      parsedValue = parsed;
    }
    return { present, valid: true, value: parsedValue };
  };
  const normalizeCandidate = (candidate: unknown, consume: (value: object) => boolean): PriceTier[] => {
    if (!Array.isArray(candidate) || candidate.length > 200 || !consume(candidate)) return [];
    const normalized: PriceTier[] = [];
    for (const item of candidate) {
      if (!item || typeof item !== 'object' || Array.isArray(item)) continue;
      if (!consume(item)) return [];
      const row = item as Record<string, unknown>;
      const tierShaped = [...beginKeys, ...endKeys, 'price', 'discountPrice', 'offerPrice'].some((key) =>
        hasOwn(row, key),
      );
      if (!tierShaped) continue;
      const begin = readConsistent(row, beginKeys, parsePositiveInteger);
      const price = readConsistent(row, priceKeys, parseTierPrice);
      const end = readConsistent(row, endKeys, parsePositiveInteger, true);
      if (!begin.present || !begin.valid || begin.value === undefined || !price.present || !price.valid || price.value === undefined) {
        return [];
      }
      if (end.present && (!end.valid || end.value === undefined || end.value < begin.value)) return [];
      normalized.push(
        end.value !== undefined
          ? { beginAmount: begin.value, endAmount: end.value, price: price.value }
          : { beginAmount: begin.value, price: price.value },
      );
    }
    if (!normalized.length) return [];
    normalized.sort((left, right) => left.beginAmount - right.beginAmount);
    const deduplicated: PriceTier[] = [];
    for (const tier of normalized) {
      const previous = deduplicated.at(-1);
      if (previous?.beginAmount === tier.beginAmount) {
        if (previous.price !== tier.price || previous.endAmount !== tier.endAmount) return [];
        continue;
      }
      deduplicated.push(tier);
    }
    for (let index = 0; index + 1 < deduplicated.length; index += 1) {
      const tier = deduplicated[index]!;
      if (tier.endAmount !== undefined && tier.endAmount >= deduplicated[index + 1]!.beginAmount) return [];
    }
    return deduplicated;
  };

  const rootBudget = normalize1688NodeBudget(maxNodesPerRoot);
  const roots = Array.isArray(root) ? root : [root];
  for (const candidateRoot of roots) {
    const visited = new WeakSet<object>();
    let remaining = rootBudget;
    const consume = (value: object): boolean => {
      if (remaining <= 0 || visited.has(value)) return false;
      visited.add(value);
      remaining -= 1;
      return true;
    };
    const walk = (value: unknown, depth: number): PriceTier[] => {
      if (depth > 16 || !value || typeof value !== 'object' || remaining <= 0) return [];
      const reference = value as object;
      if (!consume(reference)) return [];
      if (Array.isArray(value)) {
        for (const item of value) {
          const hit = walk(item, depth + 1);
          if (hit.length) return hit;
        }
        return [];
      }
      const object = value as Record<string, unknown>;
      for (const [key, candidate] of Object.entries(object)) {
        if (!isPriceTierContainerKey(key)) continue;
        const hit = normalizeCandidate(candidate, consume);
        if (hit.length) return hit;
      }
      for (const [key, nested] of Object.entries(object)) {
        if (isPriceTierContainerKey(key)) continue;
        const hit = walk(nested, depth + 1);
        if (hit.length) return hit;
      }
      return [];
    };
    const hit = walk(candidateRoot, 0);
    if (hit.length) return hit;
  }
  return [];
}

export function extractMinOrderFromUnknown(root: unknown, maxNodesPerRoot = 10_000): number | undefined {
  const minOrderKeys = ['minOrderQuantity', 'minOrder', 'moq', 'beginAmount', 'startQuantity', 'orderMinAmount'];
  const rootBudget = normalize1688NodeBudget(maxNodesPerRoot);
  const roots = Array.isArray(root) ? root : [root];
  for (const candidateRoot of roots) {
    const visited = new WeakSet<object>();
    let remaining = rootBudget;
    const walk = (value: unknown, depth: number): number | undefined => {
      if (depth > 14 || !value || typeof value !== 'object' || remaining <= 0) return undefined;
      const reference = value as object;
      if (visited.has(reference)) return undefined;
      visited.add(reference);
      remaining -= 1;
      if (Array.isArray(value)) {
        for (const item of value) {
          const hit = walk(item, depth + 1);
          if (hit !== undefined) return hit;
        }
        return undefined;
      }
      const object = value as Record<string, unknown>;
      for (const key of minOrderKeys) {
        const quantity = parse1688Quantity(object[key]);
        if (quantity !== undefined && quantity > 0) return quantity;
      }
      for (const [key, nested] of Object.entries(object)) {
        if (isPriceTierContainerKey(key)) continue;
        const hit = walk(nested, depth + 1);
        if (hit !== undefined) return hit;
      }
      return undefined;
    };
    const hit = walk(candidateRoot, 0);
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

  // Trigger long-page lazy loading, then target the detail body even when a large SKU table precedes it.
  const getDocumentHeight = () => Math.max(document.body?.scrollHeight ?? 0, document.documentElement?.scrollHeight ?? 0);
  const scrollStep = Math.max(520, Math.floor((window.innerHeight || 800) * 0.8));
  for (let y = 0, steps = 0; y < Math.min(getDocumentHeight(), 12_000) && steps < 28; y += scrollStep, steps++) {
    window.scrollTo(0, y);
    await sleep(220);
  }
  const detailRootSelectors = [
    '#offer-template-0',
    '#detail-content',
    '#detailContent',
    '#desc-lazyload-container',
    '.offer-description',
    '.offer-detail',
    '.detail-content',
    '.content-detail',
    '.detail-desc-module',
    '[class*="detail-description"]',
    '[class*="detailDescription"]',
    '[class*="offerDesc"]',
    '[module-title="商品详情"]',
    '.module-od-product-description',
    '.wireless-description',
  ];
  const detailRoots: Element[] = [];
  for (const candidate of Array.from(document.querySelectorAll(detailRootSelectors.join(',')))) {
    if (detailRoots.some((root) => root.contains(candidate))) continue;
    detailRoots.push(candidate);
    if (detailRoots.length >= 2) break;
  }
  if (!detailRoots.length) {
    const marker = Array.from(document.querySelectorAll('h2, h3, h4, [class*="module-title"], [class*="tab-title"]')).find(
      (node) => /^(?:商品详情|图文详情|产品详情)$/.test(text(node)),
    );
    if (marker) detailRoots.push(marker.parentElement ?? marker);
  }
  for (const root of detailRoots) {
    root.scrollIntoView({ block: 'start', behavior: 'auto' });
    await sleep(420);
    const start = Math.max(0, root.getBoundingClientRect().top + window.scrollY);
    const rootHeight = Math.max(root.scrollHeight, root.getBoundingClientRect().height);
    const end = Math.min(getDocumentHeight(), start + Math.min(rootHeight, 12_000));
    for (let y = start; y < end; y += scrollStep) {
      window.scrollTo(0, y);
      await sleep(180);
    }
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
    if (!/skuMap|skuModel|tradeModel|gallery|subject|priceRange|offerId|skuProps|saleProp|detail|description|wireless|template/i.test(t)) continue;
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

  const extractMainDescription = (productTitle?: string): string | undefined => {
    const comparable = (raw: unknown): string =>
      typeof raw === 'string' ? raw.replace(/\u00a0/g, ' ').replace(/\s+/g, ' ').trim() : '';
    const decodeEntities = (raw: string): string =>
      raw
        .replace(/&nbsp;|&#160;/gi, ' ')
        .replace(/&amp;/gi, '&')
        .replace(/&lt;/gi, '<')
        .replace(/&gt;/gi, '>')
        .replace(/&quot;/gi, '"')
        .replace(/&#39;|&apos;/gi, "'")
        .replace(/&#(\d+);/g, (_match, digits: string) => {
          const codePoint = Number(digits);
          return Number.isInteger(codePoint) && codePoint > 0 && codePoint <= 0x10ffff
            ? String.fromCodePoint(codePoint)
            : '';
        });
    const sanitize = (raw: unknown): string => {
      if (typeof raw !== 'string') return '';
      const decoded = decodeEntities(raw);
      const withoutCommerceTables = decoded.replace(/<table\b[^>]*>[\s\S]*?<\/table\s*>/gi, (table) =>
        /产品规格|商品规格|产品尺寸|包装信息|商品件重尺|价格|库存|起批/i.test(table) ? '' : table,
      );
      return withoutCommerceTables
        .replace(/<(?:script|style|noscript)\b[^>]*>[\s\S]*?<\/(?:script|style|noscript)\s*>/gi, '')
        .replace(/<!--[\s\S]*?-->/g, '')
        .replace(/<(?:br|hr)\b[^>]*\/?\s*>/gi, '\n')
        .replace(/<\/(?:address|article|aside|blockquote|div|dl|fieldset|figcaption|figure|footer|form|h[1-6]|header|li|main|nav|ol|p|pre|section|table|tbody|td|tfoot|th|thead|tr|ul)\s*>/gi, '\n')
        .replace(/<[^>]+>/g, ' ')
        .split(/\r?\n/)
        .map((line) => line.replace(/\s+/g, ' ').trim())
        .filter(Boolean)
        .join('\n');
    };
    const descriptionKey = (raw: string): boolean =>
      /^(?:description|desc|productDescription|offerDescription|itemDescription|detailDescription|detailDesc|detailContent|descContent|descriptionContent|wirelessDescription|wirelessDesc|mobileDescription|productDetailContent|sellingPoint|sellingPoints|sellingPointList|sellingFeature|sellingFeatures|productHighlights|highlights|featureDescription)$/i.test(
        raw,
      );
    const excludedContext = (raw: unknown): boolean =>
      /(?:^|[._/\s-])(?:sku(?:model|map|props?)?|saleprop|sale[-_ ]?prop|variant|specification|spec|productPack(?:Info)?|package|packaging|logistic|freight|price(?:model|range)?|inventory|stock(?:model)?|quantity|trade(?:model)?|tier)(?:$|[._/\s-])|产品规格|商品规格|包装|物流|价格|库存|阶梯/i.test(
        comparable(raw),
      );
    const normalizedTitle = comparable(productTitle);
    const isAssetOrUrlFragment = (raw: string): boolean =>
      /^(?:(?:https?:)?\/\/\S+|(?:data|blob):\S+|(?:\.{0,2}\/)?[^\s<>]+\.(?:jpe?g|png|webp|gif|svg|avif)(?:[?#]\S*)?)$/i.test(
        raw,
      );
    const meaningful = (raw: string): boolean => {
      const value = comparable(raw);
      if (value.length < 12) return false;
      if (!/[\p{L}\p{N}]/u.test(value) || /^(?:https?:)?\/\//i.test(value)) return false;
      if (/^(?:商品详情|产品详情|图文详情|商品描述|产品描述|卖点|暂无(?:商品)?描述|详情加载中|查看全部|登录后查看)[。！!：:\s]*$/i.test(value)) {
        return false;
      }
      return !normalizedTitle || value.toLocaleLowerCase() !== normalizedTitle.toLocaleLowerCase();
    };
    const build = (rawCandidates: unknown[]): string | undefined => {
      const fragments: string[] = [];
      const seen = new Set<string>();
      for (const raw of rawCandidates) {
        const cleaned = sanitize(raw);
        if (!cleaned) continue;
        for (const fragment of cleaned.split('\n')) {
          const value = comparable(fragment);
          if (!value || /^(?:商品详情|产品详情|图文详情|商品描述|产品描述|卖点)[：:\s]*$/i.test(value)) continue;
          if (normalizedTitle && value.toLocaleLowerCase() === normalizedTitle.toLocaleLowerCase()) continue;
          if (isAssetOrUrlFragment(value)) continue;
          const key = value.toLocaleLowerCase();
          if (seen.has(key)) continue;
          seen.add(key);
          fragments.push(value);
        }
      }
      const result = fragments.join('\n').slice(0, 5000).trim();
      return meaningful(result) ? result : undefined;
    };

    const descriptionRoots = [...detailRoots];
    for (const candidate of Array.from(
      document.querySelectorAll(
        '[class*="selling-point"], [class*="sellingPoint"], [class*="product-highlight"], [class*="productHighlight"], [module-title*="卖点"], [data-title*="卖点"]',
      ),
    )) {
      if (descriptionRoots.some((root) => root === candidate || root.contains(candidate))) continue;
      descriptionRoots.push(candidate);
      if (descriptionRoots.length >= 8) break;
    }
    const domCandidates: unknown[] = [];
    for (const root of descriptionRoots) {
      const clone = root.cloneNode(true) as Element;
      clone.querySelectorAll('script, style, noscript, nav, header, footer, h1').forEach((node) => node.remove());
      clone
        .querySelectorAll(
          '[class*="sku"], [id*="sku"], [class*="sale-prop"], [class*="saleProp"], [class*="packag"], [id*="packag"], [class*="price"], [id*="price"], [class*="inventory"], [id*="inventory"], [class*="stock"], [id*="stock"], [class*="specification"], [id*="specification"]',
        )
        .forEach((node) => node.remove());
      clone.querySelectorAll('table').forEach((table) => {
        if (/产品规格|商品规格|产品尺寸|包装信息|商品件重尺|价格|库存|起批/i.test(table.textContent ?? '')) {
          table.remove();
        }
      });
      domCandidates.push(clone.innerHTML || clone.textContent || '');
    }
    const domDescription = build(domCandidates);
    if (domDescription) return domDescription;

    const structuredCandidates: unknown[] = [];
    const walk = (
      value: unknown,
      depth: number,
      path: string,
      trusted: boolean,
      visited: WeakSet<object>,
      budget: { remaining: number },
    ): void => {
      if (depth > 20 || value == null || budget.remaining-- <= 0 || excludedContext(path)) return;
      if (typeof value === 'string') {
        if (trusted) structuredCandidates.push(value);
        return;
      }
      if (typeof value !== 'object' || visited.has(value)) return;
      visited.add(value);
      if (Array.isArray(value)) {
        for (const item of value) {
          if (budget.remaining <= 0) break;
          walk(item, depth + 1, path, trusted, visited, budget);
        }
        return;
      }
      for (const [key, nested] of Object.entries(value as Record<string, unknown>)) {
        if (budget.remaining <= 0) break;
        const nextPath = path ? `${path}.${key}` : key;
        if (excludedContext(nextPath)) continue;
        const nextTrusted =
          trusted ||
          descriptionKey(key) ||
          (/^(?:detail|productDetail|offerDetail)$/i.test(key) && typeof nested === 'string');
        if (nextTrusted && /^(?:id|url|link|image|imageUrl|imageList|video|status|success|code|count|total)$/i.test(key)) {
          continue;
        }
        walk(nested, depth + 1, nextPath, nextTrusted, visited, budget);
      }
    };
    for (const root of roots) {
      walk(root, 0, '', false, new WeakSet<object>(), { remaining: 10_000 });
    }
    const structuredDescription = build(structuredCandidates);
    if (structuredDescription) return structuredDescription;
    return build([document.querySelector('meta[name="description"]')?.getAttribute('content')]);
  };

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

  const tierContainerKeys = ['priceRange', 'priceRanges', 'priceTiers', 'priceRangeList', 'priceList', 'ladderPrice'];
  const isTierContainerKey = (key: string): boolean =>
    tierContainerKeys.some((candidate) => candidate.toLowerCase() === key.toLowerCase());
  const normalizeNodeBudget = (raw: number): number =>
    Number.isFinite(raw) ? Math.min(50_000, Math.max(1, Math.floor(raw))) : 10_000;
  const extractTiers = (root: unknown, maxNodesPerRoot = 10_000): Tier[] => {
    const beginKeys = ['beginAmount', 'startQuantity', 'minQuantity', 'begin', 'start'];
    const endKeys = ['endAmount', 'endQuantity', 'end'];
    const priceKeys = ['price', 'value', 'discountPrice', 'offerPrice'];
    const hasOwn = (value: Record<string, unknown>, key: string) => Object.prototype.hasOwnProperty.call(value, key);
    const parsePositiveInteger = (raw: unknown): number | undefined => {
      if (typeof raw === 'number') return Number.isSafeInteger(raw) && raw > 0 ? raw : undefined;
      const value = String(raw ?? '').replace(/,/g, '').trim();
      if (!/^\d+$/.test(value)) return undefined;
      const parsed = Number(value);
      return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : undefined;
    };
    const parseTierPrice = (raw: unknown): number | undefined => {
      if (typeof raw === 'number') {
        return Number.isFinite(raw) && raw > 0 && raw < 1_000_000 ? raw : undefined;
      }
      const match = String(raw ?? '')
        .replace(/,/g, '')
        .trim()
        .match(/^(?:[¥￥]\s*)?(\d+(?:\.\d{1,4})?)\s*(?:元)?$/);
      if (!match) return undefined;
      const parsed = Number(match[1]);
      return Number.isFinite(parsed) && parsed > 0 && parsed < 1_000_000 ? parsed : undefined;
    };
    const readConsistent = (
      row: Record<string, unknown>,
      keys: string[],
      parser: (raw: unknown) => number | undefined,
      emptyIsAbsent = false,
    ): { present: boolean; valid: boolean; value?: number } => {
      let present = false;
      let parsedValue: number | undefined;
      for (const key of keys) {
        if (!hasOwn(row, key)) continue;
        const rawValue = row[key];
        if (emptyIsAbsent && (rawValue == null || (typeof rawValue === 'string' && !rawValue.trim()))) continue;
        present = true;
        const parsed = parser(rawValue);
        if (parsed === undefined || (parsedValue !== undefined && parsedValue !== parsed)) {
          return { present: true, valid: false };
        }
        parsedValue = parsed;
      }
      return { present, valid: true, value: parsedValue };
    };
    const normalizeCandidate = (candidate: unknown, consume: (value: object) => boolean): Tier[] => {
      if (!Array.isArray(candidate) || candidate.length > 200 || !consume(candidate)) return [];
      const normalized: Tier[] = [];
      for (const item of candidate) {
        if (!item || typeof item !== 'object' || Array.isArray(item)) continue;
        if (!consume(item)) return [];
        const row = item as Record<string, unknown>;
        const tierShaped = [...beginKeys, ...endKeys, 'price', 'discountPrice', 'offerPrice'].some((key) =>
          hasOwn(row, key),
        );
        if (!tierShaped) continue;
        const begin = readConsistent(row, beginKeys, parsePositiveInteger);
        const price = readConsistent(row, priceKeys, parseTierPrice);
        const end = readConsistent(row, endKeys, parsePositiveInteger, true);
        if (!begin.present || !begin.valid || begin.value === undefined || !price.present || !price.valid || price.value === undefined) {
          return [];
        }
        if (end.present && (!end.valid || end.value === undefined || end.value < begin.value)) return [];
        normalized.push(
          end.value !== undefined
            ? { beginAmount: begin.value, endAmount: end.value, price: price.value }
            : { beginAmount: begin.value, price: price.value },
        );
      }
      if (!normalized.length) return [];
      normalized.sort((left, right) => left.beginAmount - right.beginAmount);
      const deduplicated: Tier[] = [];
      for (const tier of normalized) {
        const previous = deduplicated.at(-1);
        if (previous?.beginAmount === tier.beginAmount) {
          if (previous.price !== tier.price || previous.endAmount !== tier.endAmount) return [];
          continue;
        }
        deduplicated.push(tier);
      }
      for (let index = 0; index + 1 < deduplicated.length; index += 1) {
        const tier = deduplicated[index]!;
        if (tier.endAmount !== undefined && tier.endAmount >= deduplicated[index + 1]!.beginAmount) return [];
      }
      return deduplicated;
    };

    const rootBudget = normalizeNodeBudget(maxNodesPerRoot);
    const candidates = Array.isArray(root) ? root : [root];
    for (const candidateRoot of candidates) {
      const visited = new WeakSet<object>();
      let remaining = rootBudget;
      const consume = (value: object): boolean => {
        if (remaining <= 0 || visited.has(value)) return false;
        visited.add(value);
        remaining -= 1;
        return true;
      };
      const walk = (value: unknown, depth: number): Tier[] => {
        if (depth > 16 || !value || typeof value !== 'object' || remaining <= 0) return [];
        const reference = value as object;
        if (!consume(reference)) return [];
        if (Array.isArray(value)) {
          for (const item of value) {
            const hit = walk(item, depth + 1);
            if (hit.length) return hit;
          }
          return [];
        }
        const object = value as Record<string, unknown>;
        for (const [key, candidate] of Object.entries(object)) {
          if (!isTierContainerKey(key)) continue;
          const hit = normalizeCandidate(candidate, consume);
          if (hit.length) return hit;
        }
        for (const [key, nested] of Object.entries(object)) {
          if (isTierContainerKey(key)) continue;
          const hit = walk(nested, depth + 1);
          if (hit.length) return hit;
        }
        return [];
      };
      const hit = walk(candidateRoot, 0);
      if (hit.length) return hit;
    }
    return [];
  };

  const extractMinOrder = (root: unknown, maxNodesPerRoot = 10_000): number | undefined => {
    const minOrderKeys = ['minOrderQuantity', 'minOrder', 'moq', 'beginAmount', 'startQuantity', 'orderMinAmount'];
    const rootBudget = normalizeNodeBudget(maxNodesPerRoot);
    const candidates = Array.isArray(root) ? root : [root];
    for (const candidateRoot of candidates) {
      const visited = new WeakSet<object>();
      let remaining = rootBudget;
      const walk = (value: unknown, depth: number): number | undefined => {
        if (depth > 14 || !value || typeof value !== 'object' || remaining <= 0) return undefined;
        const reference = value as object;
        if (visited.has(reference)) return undefined;
        visited.add(reference);
        remaining -= 1;
        if (Array.isArray(value)) {
          for (const item of value) {
            const hit = walk(item, depth + 1);
            if (hit !== undefined) return hit;
          }
          return undefined;
        }
        const object = value as Record<string, unknown>;
        for (const key of minOrderKeys) {
          const quantity = parseQty(object[key]);
          if (quantity !== undefined && quantity > 0) return quantity;
        }
        for (const [key, nested] of Object.entries(object)) {
          if (isTierContainerKey(key)) continue;
          const hit = walk(nested, depth + 1);
          if (hit !== undefined) return hit;
        }
        return undefined;
      };
      const hit = walk(candidateRoot, 0);
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
    const normalizeCandidate = (raw: unknown): string => {
      if (typeof raw !== 'string') return '';
      const value = raw.replace(/\s+/g, ' ').trim();
      if (value.length < 4 || value.length > 300 || /^\d+$/.test(value)) return '';
      return value;
    };
    const walk = (x: unknown, depth: number) => {
      if (best || depth > 18 || !x || typeof x !== 'object') return;
      if (Array.isArray(x)) {
        for (const i of x) walk(i, depth + 1);
        return;
      }
      const o = x as Record<string, unknown>;
      for (const k of preferredKeys) {
        const candidate = normalizeCandidate(o[k]);
        if (candidate) {
          best = candidate;
          return;
        }
      }
      for (const v of Object.values(o)) walk(v, depth + 1);
    };
    for (const r of roots) walk(r, 0);
    if (best) return best;
    for (const sel of ['h1.d-title', '.offer-title .title-text', '.title-content h1', 'h1[class*="title"]', 'h1']) {
      const candidate = normalizeCandidate(text(document.querySelector(sel)));
      if (candidate) return candidate;
    }
    const og = normalizeCandidate(document.querySelector('meta[property="og:title"]')?.getAttribute('content'));
    if (og) return og;
    return normalizeCandidate(titlePeek.replace(/[-_|].*1688.*$/i, ''));
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

  const imageCanonical = (raw: string): string => (normalizeImage(raw).split(/[?#]/)[0] ?? '').toLowerCase();
  const normalizeSkuImage = (raw: unknown): string => {
    const image = typeof raw === 'string' ? normalizeImage(raw) : '';
    if (!image || /(?:spaceball|\/s\.gif|placeholder|loading|spacer|icon|logo|avatar|wangwang|promise|badge)/i.test(image)) {
      return '';
    }
    return image;
  };
  const excludedDetailAncestor = (node: Element): boolean =>
    !!node.closest(
      '.vertical-img, .detail-gallery-preview, .detail-gallery, [class*="offer-gallery"], [class*="main-image"], .swiper-slide, .obj-header-image, .obj-sku-img-item, [class*="sku-selector"], [class*="sale-prop"]',
    );
  const junkDetailAncestor = (node: Element, root: Element): boolean => {
    let current: Element | null = node;
    while (current && current !== root.parentElement) {
      const className = typeof current.className === 'string' ? current.className : '';
      const hint = `${className} ${current.id}`.toLowerCase();
      if (/(?:service|promise|guarantee|credit|badge|toolbar|icon|wangwang|footer|header-nav|trust)/i.test(hint)) {
        return true;
      }
      if (current === root) break;
      current = current.parentElement;
    }
    return false;
  };
  const collectDetailDomImages = (selectors: string[]): string[] => {
    const urls: string[] = [];
    const pushBackgrounds = (raw: string) => {
      for (const match of raw.matchAll(/url\(\s*["']?([^"')]+)["']?\s*\)/gi)) {
        const value = normalizeImage(match[1]);
        if (value && !/(?:spaceball|\/s\.gif|icon|logo|placeholder|loading|avatar|wangwang|promise|badge)/i.test(value)) {
          urls.push(value);
        }
      }
    };
    const pickValidSource = (raw: string | null | undefined): string => {
      const value = normalizeImage(raw);
      if (!value || /(?:spaceball|\/s\.gif|icon|logo|placeholder|loading|avatar|wangwang|promise|badge)/i.test(value)) {
        return '';
      }
      return value;
    };
    for (const selector of selectors) {
      document.querySelectorAll(selector).forEach((root) => {
        root.querySelectorAll('img, source, [style*="background"]').forEach((node) => {
          if (excludedDetailAncestor(node) || junkDetailAncestor(node, root)) return;
          if (node instanceof HTMLImageElement && node.naturalWidth > 0 && node.naturalHeight > 0 && node.naturalWidth < 72 && node.naturalHeight < 72) {
            return;
          }
          const attrs = ['data-lazy-src', 'data-src', 'data-original', 'data-img', 'data-zoom', 'src'];
          let picked = '';
          for (const attr of attrs) {
            picked = pickValidSource(node.getAttribute(attr));
            if (picked) break;
          }
          if (!picked && node instanceof HTMLImageElement) picked = pickValidSource(node.currentSrc);
          if (!picked) {
            const srcset = node.getAttribute('srcset') || '';
            const srcsetCandidates = srcset.split(',').map((part) => part.trim().split(/\s+/)[0] || '').filter(Boolean).reverse();
            for (const candidate of srcsetCandidates) {
              picked = pickValidSource(candidate);
              if (picked) break;
            }
          }
          if (picked) urls.push(picked);
          if (node instanceof HTMLElement) {
            pushBackgrounds(node.style.backgroundImage || window.getComputedStyle(node).backgroundImage || '');
          }
        });
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
  const detailFromDom = collectDetailDomImages([
    '#offer-template-0',
    '#detail-content',
    '#detailContent',
    '#desc-lazyload-container',
    '.offer-description',
    '.offer-detail',
    '.detail-content',
    '.content-detail',
    '.detail-desc-module',
    '[class*="detail-description"]',
    '[class*="detailDescription"]',
    '[class*="offerDesc"]',
    '[module-title="商品详情"]',
    '.module-od-product-description',
    '.wireless-description',
  ]);

  const tableRoots = Array.from(
    document.querySelectorAll('table, [role="table"], [role="grid"], .next-table, [class*="sku-table"], [class*="skuTable"]'),
  ).filter(
    (table, index, all) => !all.some((other, otherIndex) => otherIndex < index && other.contains(table)),
  );
  const packagingMarkers = Array.from(
    document.querySelectorAll('h1, h2, h3, h4, [class*="title"], [class*="label"], [class*="header"]'),
  ).filter((node) => /^(?:包装信息|商品件重尺|包装件重尺)$/.test(text(node)));
  const packagingContextFor = (table: Element): string => {
    for (const marker of packagingMarkers) {
      const common = marker.parentElement;
      if (common?.contains(table)) return text(marker);
      const markerRect = marker.getBoundingClientRect();
      const tableRect = table.getBoundingClientRect();
      if (tableRect.top >= markerRect.top && tableRect.top - markerRect.bottom < 900) return text(marker);
    }
    let current: Element | null = table;
    for (let depth = 0; current && depth < 4; depth++, current = current.parentElement) {
      let previous = current.previousElementSibling;
      for (let count = 0; previous && count < 4; count++, previous = previous.previousElementSibling) {
        const hint = text(previous);
        if (/(?:包装信息|商品件重尺|包装件重尺)/.test(hint)) return hint.slice(0, 200);
      }
    }
    return '';
  };
  const pickSkuRowImage = (cell: Element): string => {
    const nodes = Array.from(cell.querySelectorAll('img, source'));
    for (const node of nodes) {
      const sources: string[] = [];
      for (const attr of ['data-lazy-src', 'data-src', 'data-original', 'data-img', 'src']) {
        const raw = node.getAttribute(attr);
        if (raw) sources.push(raw);
      }
      if (node instanceof HTMLImageElement) sources.push(node.currentSrc || node.src || '');
      const srcset = node.getAttribute('srcset') || '';
      sources.push(...srcset.split(',').map((part) => part.trim().split(/\s+/)[0] || '').reverse());
      for (const source of sources) {
        const image = normalizeSkuImage(source);
        if (image) return image;
      }
    }
    if (cell instanceof HTMLElement) {
      const background = cell.style.backgroundImage || window.getComputedStyle(cell).backgroundImage || '';
      for (const match of background.matchAll(/url\(\s*["']?([^"')]+)["']?\s*\)/gi)) {
        const image = normalizeSkuImage(match[1]);
        if (image) return image;
      }
    }
    return '';
  };
  const normalizeSkuPropertyHeader = (raw: unknown): string => {
    const value = trim(raw)
      .replace(/[（]/g, '(')
      .replace(/[）]/g, ')')
      .replace(/\s+/g, '')
      .replace(/[:：]+$/u, '');
    if (
      !value ||
      value.length > 40 ||
      /[<>\u0000-\u001F\u007F]/u.test(value) ||
      /^(?:__proto__|prototype|constructor)$/iu.test(value)
    ) {
      return '';
    }
    return value;
  };
  const isSkuPurchaseOperationHeader = (header: string): boolean =>
    /(?:价格|库存|进货数量|采购数量|订购数量|购买数量|起订数量|操作|加购|数量)/u.test(header);
  const domSkuImageRows: Array<{ specification: string; image: string }> = [];
  const skuImageRowSeen = new Set<string>();
  const domSkuPropertyRows: Array<{ specification: string; properties: Record<string, string> }> = [];
  const skuPropertyRowSeen = new Set<string>();
  let inspectedSkuPurchaseRows = 0;
  for (const table of tableRoots) {
    let headerCells = Array.from(table.querySelectorAll('thead tr:last-child th, thead tr:last-child td, [role="columnheader"]'));
    let headerRow = headerCells[0]?.closest('tr, [role="row"], .next-table-row') ?? null;
    if (!headerCells.length) {
      headerRow = table.querySelector('tr, [role="row"], .next-table-row');
      headerCells = headerRow
        ? Array.from(headerRow.querySelectorAll(':scope > th, :scope > td, [role="columnheader"], .next-table-cell'))
        : [];
    }
    const headers = headerCells.map((cell) => normalizeSkuPropertyHeader(text(cell)));
    const specificationIndex = headers.findIndex((header) => /^(?:产品规格|商品规格|规格)$/.test(header));
    const isPurchaseTable = headers.some((header) =>
      /(?:价格.*库存|库存.*价格|进货数量|采购数量|订购数量|购买数量|起订数量)/.test(header),
    );
    if (specificationIndex < 0 || !isPurchaseTable) continue;
    if (/(?:包装信息|商品件重尺|包装件重尺)/.test(packagingContextFor(table))) continue;
    const canCollectProperties =
      headers.length >= 3 &&
      headers.length <= 12 &&
      headers.every(Boolean) &&
      new Set(headers).size === headers.length;
    const descriptiveIndexes = canCollectProperties
      ? headers
          .map((header, index) => (!isSkuPurchaseOperationHeader(header) ? index : -1))
          .filter((index) => index >= 0)
      : [];
    const dataRows = Array.from(
      table.querySelectorAll('tbody tr, [role="row"], .next-table-body .next-table-row'),
    ).filter((row) => row !== headerRow && !row.querySelector('[role="columnheader"]') && !row.closest('thead'));
    for (const row of dataRows) {
      if (inspectedSkuPurchaseRows >= 200) break;
      inspectedSkuPurchaseRows += 1;
      const cells = Array.from(row.querySelectorAll(':scope > th, :scope > td, [role="cell"], .next-table-cell'));
      const specificationCell = cells[specificationIndex];
      if (!specificationCell) continue;
      const imageNode = specificationCell.querySelector('img');
      const specification = trim(
        text(specificationCell) || imageNode?.getAttribute('alt') || imageNode?.getAttribute('title') || '',
      );
      if (
        domSkuPropertyRows.length < 200 &&
        descriptiveIndexes.length >= 2 &&
        cells.length <= 12 &&
        specification &&
        specification.length <= 200
      ) {
        const properties: Record<string, string> = {};
        let valid = true;
        for (const index of descriptiveIndexes) {
          if (index === specificationIndex) {
            properties[headers[index]!] = specification;
            continue;
          }
          const value = trim(text(cells[index]));
          if (!value || /^(?:—|–|-|--|暂无|无|N\/?A)$/i.test(value)) continue;
          if (value.length > 200 || /[<>\u0000-\u0008\u000B\u000C\u000E-\u001F]/.test(value)) {
            valid = false;
            break;
          }
          properties[headers[index]!] = value;
        }
        if (valid && Object.keys(properties).length >= 2) {
          const key = `${specification}\u0000${JSON.stringify(properties)}`;
          if (!skuPropertyRowSeen.has(key)) {
            skuPropertyRowSeen.add(key);
            domSkuPropertyRows.push({ specification, properties });
          }
        }
      }
      const image = pickSkuRowImage(specificationCell);
      if (!specification || specification.length > 200 || !image) continue;
      const key = `${specification}\u0000${imageCanonical(image)}`;
      if (skuImageRowSeen.has(key)) continue;
      skuImageRowSeen.add(key);
      domSkuImageRows.push({ specification, image });
      if (domSkuImageRows.length >= 200) break;
    }
    if (inspectedSkuPurchaseRows >= 200 || domSkuImageRows.length >= 200) break;
  }

  const skuFromDom = [
    ...collectDomImages([
    '.obj-sku-img-item img',
    '[class*="sku-selector"] img',
    '[class*="sale-prop"] img',
    '[class*="sku-item-wrapper"] img',
    ]),
    ...domSkuImageRows.map((row) => row.image),
  ];

  const mainFromData: string[] = [];
  const detailFromData: string[] = [];
  const walkDetail = (
    x: unknown,
    depth: number,
    keyHint: string,
    visited: WeakSet<object>,
    budget: { remaining: number },
  ): void => {
    if (depth > 22 || x == null || budget.remaining-- <= 0) return;
    if (typeof x === 'string') {
      if (!/detail|desc|description|content|wireless|template/i.test(keyHint)) return;
      if (/sku|spec|variant|saleprop|mainimage|gallery|album|thumb/i.test(keyHint)) return;
      const normalizedText = x.replace(/&amp;/gi, '&').replace(/\\u002f/gi, '/').replace(/\\\//g, '/');
      const re = /((?:https?:)?\/\/[^\s"'<>]+)/gi;
      let m: RegExpExecArray | null;
      while ((m = re.exec(normalizedText))) {
        if (/\/img\/ibank\//i.test(m[1]) || /\.(jpg|jpeg|png|webp)/i.test(m[1])) detailFromData.push(m[1]);
      }
      return;
    }
    if (typeof x !== 'object' || visited.has(x)) return;
    visited.add(x);
    if (Array.isArray(x)) {
      for (const i of x) walkDetail(i, depth + 1, keyHint, visited, budget);
      return;
    }
    for (const [k, v] of Object.entries(x as Record<string, unknown>)) {
      walkDetail(v, depth + 1, `${keyHint}.${k}`, visited, budget);
    }
  };
  const scanDetailRoot = (root: unknown): void => {
    walkDetail(root, 0, '', new WeakSet<object>(), { remaining: 10_000 });
  };
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
    scanDetailRoot(data);
  }
  for (const root of roots) scanDetailRoot(root);
  const ogImage = document.querySelector('meta[property="og:image"]')?.getAttribute('content') || '';
  const mainImages = uniqueImages([...mainFromData, ...mainFromDom, ogImage], 12);
  const excludedDescriptionKeys = new Set([...mainImages, ...skuFromDom].map(imageCanonical).filter(Boolean));
  const descriptionImages = uniqueImages(
    [...detailFromData, ...detailFromDom].filter((url) => !excludedDescriptionKeys.has(imageCanonical(url))),
    30,
  );

  // 包装信息（商品件重尺）与 SKU 产品尺寸是不同语义；仅采集带明确单位的包装表格。
  const normalizePackagingText = (raw: unknown): string =>
    typeof raw === 'string' || typeof raw === 'number'
      ? String(raw).replace(/\u00a0/g, ' ').replace(/\s+/g, ' ').trim()
      : '';
  const packagingHeaderRole = (raw: unknown): string => {
    const value = normalizePackagingText(raw)
      .replace(/[（]/g, '(')
      .replace(/[）]/g, ')')
      .replace(/\s+/g, '');
    const roles: Array<[string, RegExp]> = [
      ['specification', /^(?:产品规格|商品规格|规格)$/u],
      ['lengthCm', /^长(?:度)?(?:\(cm\)|cm)$/iu],
      ['widthCm', /^宽(?:度)?(?:\(cm\)|cm)$/iu],
      ['heightCm', /^高(?:度)?(?:\(cm\)|cm)$/iu],
      ['volumeCm3', /^体积(?:\(cm(?:³|3)\)|cm(?:³|3))$/iu],
      ['weightG', /^重量(?:\(g\)|g)$/iu],
    ];
    return roles.find(([, pattern]) => pattern.test(value))?.[0] ?? '';
  };
  const parsePackagingMeasurement = (raw: unknown): number | null | undefined => {
    const value = normalizePackagingText(raw);
    if (!value || /^(?:—|–|-|--|暂无|无|N\/?A)$/iu.test(value)) return null;
    if (!/^(?:\d{1,3}(?:,\d{3})+|\d+)(?:\.\d+)?$/.test(value)) return undefined;
    const parsed = Number(value.replace(/,/g, ''));
    return Number.isFinite(parsed) && parsed >= 0 ? parsed : undefined;
  };
  const packagingRows: Array<{
    specification: string;
    lengthCm: number | null;
    widthCm: number | null;
    heightCm: number | null;
    volumeCm3: number | null;
    weightG: number | null;
  }> = [];
  const packagingSeen = new Set<string>();
  for (const table of tableRoots) {
    const context = packagingContextFor(table);
    if (!/(?:包装信息|商品件重尺|包装件重尺)/.test(context)) continue;
    let headerCells = Array.from(table.querySelectorAll('thead tr:last-child th, thead tr:last-child td, [role="columnheader"]'));
    if (!headerCells.length) {
      const firstRow = table.querySelector('tr, [role="row"], .next-table-row');
      headerCells = firstRow
        ? Array.from(firstRow.querySelectorAll(':scope > th, :scope > td, [role="columnheader"], .next-table-cell'))
        : [];
    }
    const indexes = new Map<string, number>();
    headerCells.forEach((cell, index) => {
      const role = packagingHeaderRole(text(cell));
      if (role && !indexes.has(role)) indexes.set(role, index);
    });
    const required = ['specification', 'lengthCm', 'widthCm', 'heightCm', 'volumeCm3', 'weightG'];
    if (!required.every((role) => indexes.has(role))) continue;
    const dataRows = Array.from(
      table.querySelectorAll('tbody tr, [role="row"], .next-table-body .next-table-row'),
    ).filter((row) => !row.querySelector('[role="columnheader"]') && !row.closest('thead'));
    for (const row of dataRows) {
      const cells = Array.from(row.querySelectorAll(':scope > th, :scope > td, [role="cell"], .next-table-cell'));
      const values = cells.map((cell) => text(cell));
      const specification = normalizePackagingText(values[indexes.get('specification')!]);
      if (!specification || specification.length > 200 || /^(?:产品规格|商品规格|规格)$/.test(specification)) continue;
      const lengthCm = parsePackagingMeasurement(values[indexes.get('lengthCm')!]);
      const widthCm = parsePackagingMeasurement(values[indexes.get('widthCm')!]);
      const heightCm = parsePackagingMeasurement(values[indexes.get('heightCm')!]);
      const volumeCm3 = parsePackagingMeasurement(values[indexes.get('volumeCm3')!]);
      const weightG = parsePackagingMeasurement(values[indexes.get('weightG')!]);
      if ([lengthCm, widthCm, heightCm, volumeCm3, weightG].some((value) => value === undefined)) continue;
      const collected = {
        specification,
        lengthCm: lengthCm!,
        widthCm: widthCm!,
        heightCm: heightCm!,
        volumeCm3: volumeCm3!,
        weightG: weightG!,
      };
      const key = JSON.stringify(collected);
      if (!packagingSeen.has(key)) {
        packagingSeen.add(key);
        packagingRows.push(collected);
      }
      if (packagingRows.length >= 200) break;
    }
    if (packagingRows.length >= 200) break;
  }
  const packaging = packagingRows.length ? { rows: packagingRows } : undefined;

  // 产品规格表中的“产品尺寸”属于 SKU 属性，不得与包装件重尺互相推断。
  const normalizeProductDimensionCell = (raw: unknown): string =>
    typeof raw === 'string' || typeof raw === 'number'
      ? String(raw).replace(/\u00a0/g, ' ').replace(/\s+/g, ' ').trim()
      : '';
  const productDimensionHeaderRole = (raw: unknown): string => {
    const value = normalizeProductDimensionCell(raw).replace(/\s+/g, '');
    if (/^(?:产品规格|商品规格|规格)$/.test(value)) return 'specification';
    if (/^(?:产品尺寸|商品尺寸)$/.test(value)) return 'productDimension';
    if (/^(?:品名|产品名称|商品名称)$/.test(value)) return 'productName';
    return '';
  };
  const normalizeProductDimensionValue = (raw: unknown): string => {
    const value = normalizeProductDimensionCell(raw);
    if (!value || value.length > 120 || /^(?:—|–|-|--|暂无|无|N\/?A)$/i.test(value)) return '';
    const normalized = value.replace(/\s*(?:[×*＊]|[xX])\s*/g, '×');
    const numberWithOptionalUnit = String.raw`\d+(?:\.\d+)?(?:\s*(?:mm|cm|m|毫米|厘米|米|寸|英寸))?`;
    const pattern = new RegExp(
      String.raw`^${numberWithOptionalUnit}(?:×${numberWithOptionalUnit}){1,3}(?:\s*[（(](?:mm|cm|m|毫米|厘米|米|寸|英寸)[）)])?$`,
      'i',
    );
    return pattern.test(normalized) ? normalized : '';
  };
  const productDimensionRows: Array<{
    specification: string;
    productDimension: string;
    productName: string;
  }> = [];
  const productDimensionSeen = new Set<string>();
  for (const table of tableRoots) {
    const context = packagingContextFor(table);
    if (/(?:包装信息|商品件重尺|包装件重尺)/.test(context)) continue;
    let headerCells = Array.from(table.querySelectorAll('thead tr:last-child th, thead tr:last-child td, [role="columnheader"]'));
    let headerRow = headerCells[0]?.closest('tr, [role="row"], .next-table-row') ?? null;
    if (!headerCells.length) {
      headerRow = table.querySelector('tr, [role="row"], .next-table-row');
      headerCells = headerRow
        ? Array.from(headerRow.querySelectorAll(':scope > th, :scope > td, [role="columnheader"], .next-table-cell'))
        : [];
    }
    const indexes = new Map<string, number>();
    headerCells.forEach((cell, index) => {
      const role = productDimensionHeaderRole(text(cell));
      if (role && !indexes.has(role)) indexes.set(role, index);
    });
    if (!['specification', 'productDimension', 'productName'].every((role) => indexes.has(role))) continue;
    const dataRows = Array.from(
      table.querySelectorAll('tbody tr, [role="row"], .next-table-body .next-table-row'),
    ).filter((row) => row !== headerRow && !row.querySelector('[role="columnheader"]') && !row.closest('thead'));
    for (const row of dataRows) {
      const cells = Array.from(row.querySelectorAll(':scope > th, :scope > td, [role="cell"], .next-table-cell'));
      const values = cells.map((cell) => text(cell));
      const specification = normalizeProductDimensionCell(values[indexes.get('specification')!]);
      const productDimension = normalizeProductDimensionValue(values[indexes.get('productDimension')!]);
      const productName = normalizeProductDimensionCell(values[indexes.get('productName')!]);
      if (
        !specification ||
        specification.length > 200 ||
        /^(?:产品规格|商品规格|规格)$/.test(specification) ||
        !productDimension ||
        productName.length > 200 ||
        /^(?:品名|产品名称|商品名称)$/.test(productName)
      ) {
        continue;
      }
      const collected = { specification, productDimension, productName };
      const key = JSON.stringify(collected);
      if (!productDimensionSeen.has(key)) {
        productDimensionSeen.add(key);
        productDimensionRows.push(collected);
      }
      if (productDimensionRows.length >= 200) break;
    }
    if (productDimensionRows.length >= 200) break;
  }

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

  const normalizeAttributeText = (raw: unknown): string =>
    typeof raw === 'string' || typeof raw === 'number' || typeof raw === 'boolean'
      ? String(raw).replace(/\u00a0/g, ' ').replace(/\s+/g, ' ').trim()
      : '';
  let semanticAttributeCount = 0;
  const setSemanticAttr = (rawName: unknown, rawValue: unknown): void => {
    const name = normalizeAttributeText(rawName);
    const value = normalizeAttributeText(rawValue);
    const normalizedName = name.replace(/[:：\s]+$/u, '');
    if (!normalizedName || !value || attributes[normalizedName] !== undefined || semanticAttributeCount >= 200) return;
    setAttr(name, typeof rawValue === 'number' || typeof rawValue === 'boolean' ? rawValue : value);
    if (attributes[normalizedName] !== undefined) semanticAttributeCount += 1;
  };
  const isExplicitAttributeContext = (raw: unknown): boolean =>
    /(?:商品|产品)?(?:属性|参数)|(?:offer|product)[-_ ]?(?:attr|param|feature)|feature/i.test(
      normalizeAttributeText(raw),
    );
  const isExcludedAttributeContext = (raw: unknown): boolean =>
    /(?:sku|sale[-_ ]?prop|variant|specification|产品规格|商品规格|包装|packag|物流|logistic|freight|价格|price|库存|inventory|stock|quantity|阶梯|ladder|tier)/i.test(
      normalizeAttributeText(raw),
    );

  // DOM product attributes: only explicitly labelled attribute/parameter containers.
  const attributeNodes = new Map<Element, { selector: string; requiresLabel: boolean }>();
  for (const [selector, requiresLabel] of [
    ['.offer-attr-item', false],
    ['.offer-attrprogram .de-feature-item', false],
    ['.offer-attrprogram li', false],
    ['.offer-params tr', false],
    ['[module-title="商品属性"] tr', false],
    ['[module-title="产品属性"] tr', false],
    ['[module-title="商品参数"] tr', false],
    ['[module-title="产品参数"] tr', false],
    ['[data-title="商品属性"] tr', false],
    ['[data-title="产品属性"] tr', false],
    ['[data-title="商品参数"] tr', false],
    ['[data-title="产品参数"] tr', false],
    ['[class*="offer-attr"] [class*="item"]', false],
    ['[class*="product-attr"] tr', false],
    ['[class*="product-param"] tr', false],
    ['[class*="param-table"] tr', true],
  ] as const) {
    document.querySelectorAll(selector).forEach((node) => {
      const current = attributeNodes.get(node);
      if (!current || (current.requiresLabel && !requiresLabel)) attributeNodes.set(node, { selector, requiresLabel });
    });
  }
  attributeNodes.forEach(({ selector, requiresLabel }, node) => {
    const labelledScope = node.closest('[module-title], [data-title], [aria-label]');
    const structuralScope = node.closest(
      '.offer-attrprogram, .offer-params, [class*="offer-attr"], [class*="product-attr"], [class*="product-param"], [class*="param-table"]',
    );
    const labelledContext = [
      labelledScope?.getAttribute('module-title'),
      labelledScope?.getAttribute('data-title'),
      labelledScope?.getAttribute('aria-label'),
    ].join(' ');
    const context = [
      requiresLabel ? '' : selector,
      labelledContext,
      structuralScope?.className,
    ].join(' ');
    if (requiresLabel && !isExplicitAttributeContext(labelledContext)) return;
    if (!isExplicitAttributeContext(context) || isExcludedAttributeContext(context)) return;
    const nameNode = node.querySelector(
      'dt, th:first-child, td:first-child, [class*="label"], [class*="name"]',
    );
    const valueNode = node.querySelector('dd, td:last-child, [class*="value"]');
    if (nameNode && valueNode && nameNode !== valueNode) {
      setSemanticAttr(text(nameNode), text(valueNode));
      return;
    }
    const blob = text(node);
    const match = /^(.{2,30})[:：]\s*(.+)$/.exec(blob);
    if (match) setSemanticAttr(match[1], match[2]);
  });

  // JSON product attributes: discover semantic containers, never reinterpret arbitrary arrays as attributes.
  const semanticAttributeContainerKey = /^(?:(?:offer|product)?(?:attrs?|attributes?|features?|params?|parameters?)(?:list)?)$/i;
  const strongAttributeNameKeys = ['attributeName', 'attrName', 'featureName', 'paramName', 'propertyName'] as const;
  const strongAttributeValueKeys = ['attributeValue', 'attrValue', 'featureValue', 'paramValue', 'propertyValue'] as const;
  const genericAttributeNameKeys = ['name', 'fname', 'label'] as const;
  const genericAttributeValueKeys = ['value', 'text', 'vname'] as const;
  const attributeMetadataKey = /^(?:id|key|type|code|index|order|sort|visible|required|unit|units|display|title|status|success|message|count|total|page|pagesize|hasmore)$/i;
  const readFirstAttributeField = (object: Record<string, unknown>, keys: readonly string[]): unknown => {
    for (const key of keys) {
      if (object[key] !== undefined && object[key] !== null) return object[key];
    }
    return undefined;
  };
  const scanAttributeRoot = (root: unknown): void => {
    const visited = new WeakSet<object>();
    const budget = { remaining: 12_000 };
    const walkAttributeContainer = (value: unknown, depth: number, path: string): void => {
      if (depth > 18 || value == null || budget.remaining-- <= 0 || isExcludedAttributeContext(path)) return;
      if (typeof value !== 'object' || visited.has(value)) return;
      visited.add(value);
      if (Array.isArray(value)) {
        for (const row of value) walkAttributeContainer(row, depth + 1, path);
        return;
      }
      const object = value as Record<string, unknown>;
      const strongName = readFirstAttributeField(object, strongAttributeNameKeys);
      const strongValue = readFirstAttributeField(object, strongAttributeValueKeys) ?? object.value;
      const genericName = readFirstAttributeField(object, genericAttributeNameKeys);
      const genericValue = readFirstAttributeField(object, genericAttributeValueKeys);
      const rowName = strongName ?? genericName;
      const rowValue = strongValue ?? genericValue;
      if (rowName !== undefined && rowValue !== undefined) {
        setSemanticAttr(rowName, rowValue);
      } else {
        const entries = Object.entries(object);
        if (entries.length <= 80) {
          for (const [key, nested] of entries) {
            if (attributeMetadataKey.test(key) || nested == null || typeof nested === 'object') continue;
            setSemanticAttr(key, nested);
          }
        }
      }
      for (const [key, nested] of Object.entries(object)) {
        if (nested && typeof nested === 'object' && !isExcludedAttributeContext(key)) {
          walkAttributeContainer(nested, depth + 1, `${path}.${key}`);
        }
      }
    };
    const walk = (value: unknown, depth: number, path: string): void => {
      if (depth > 18 || value == null || budget.remaining-- <= 0 || isExcludedAttributeContext(path)) return;
      if (typeof value !== 'object' || visited.has(value)) return;
      visited.add(value);
      if (Array.isArray(value)) {
        for (const item of value) walk(item, depth + 1, path);
        return;
      }
      const object = value as Record<string, unknown>;
      const strongName = readFirstAttributeField(object, strongAttributeNameKeys);
      const strongValue = readFirstAttributeField(object, strongAttributeValueKeys) ?? object.value;
      if (strongName !== undefined && strongValue !== undefined) setSemanticAttr(strongName, strongValue);
      for (const [key, nested] of Object.entries(object)) {
        if (!nested || typeof nested !== 'object') continue;
        const nextPath = `${path}.${key}`;
        if (isExcludedAttributeContext(nextPath)) continue;
        if (semanticAttributeContainerKey.test(key)) walkAttributeContainer(nested, depth + 1, nextPath);
        else walk(nested, depth + 1, nextPath);
      }
    };
    walk(root, 0, 'root');
  };
  for (const root of roots.slice(0, 12)) scanAttributeRoot(root);

  const extractionRoots = data ? [data, ...roots] : roots;
  const priceTiers = extractTiers(extractionRoots);
  let minOrder = extractMinOrder(extractionRoots);
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
  const extractOfferPrice = (root: unknown, maxNodesPerRoot = 10_000): number | undefined => {
    const rootBudget = normalizeNodeBudget(maxNodesPerRoot);
    const candidates = Array.isArray(root) ? root : [root];
    for (const candidateRoot of candidates) {
      const visited = new WeakSet<object>();
      let remaining = rootBudget;
      const walk = (value: unknown, depth: number): number | undefined => {
        if (depth > 14 || !value || typeof value !== 'object' || remaining <= 0) return undefined;
        const reference = value as object;
        if (visited.has(reference)) return undefined;
        visited.add(reference);
        remaining -= 1;
        if (Array.isArray(value)) {
          for (const item of value) {
            const hit = walk(item, depth + 1);
            if (hit !== undefined) return hit;
          }
          return undefined;
        }
        const object = value as Record<string, unknown>;
        for (const key of priceKeys) {
          const price = parsePrice(object[key]);
          if (price !== undefined) return price;
        }
        for (const key of moneyKeys) {
          const money = object[key];
          if (!money || typeof money !== 'object') continue;
          const bucket = money as Record<string, unknown>;
          const price = parsePrice(bucket.value) ?? parsePrice(bucket.number);
          if (price !== undefined) return price;
        }
        for (const [key, nested] of Object.entries(object)) {
          if (isTierContainerKey(key) || !nested || typeof nested !== 'object') continue;
          const hit = walk(nested, depth + 1);
          if (hit !== undefined) return hit;
        }
        return undefined;
      };
      const hit = walk(candidateRoot, 0);
      if (hit !== undefined) return hit;
    }
    return undefined;
  };
  if (productPrice === undefined) productPrice = extractOfferPrice(extractionRoots);
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
  const propArrayRoots: unknown[][] = [];
  const collectSkuStructures = (root: unknown): unknown[] => {
    const propArrays: unknown[] = [];
    const visited = new WeakSet<object>();
    let remaining = 15_000;
    const walk = (value: unknown, depth: number): void => {
      if (depth > 22 || value == null || remaining-- <= 0 || typeof value !== 'object') return;
      if (visited.has(value)) return;
      visited.add(value);
      if (Array.isArray(value)) {
        for (const item of value) {
          if (remaining <= 0) break;
          walk(item, depth + 1);
        }
        return;
      }
      const object = value as Record<string, unknown>;
      for (const key in object) {
        if (!Object.prototype.hasOwnProperty.call(object, key)) continue;
        if (remaining-- <= 0) break;
        const nested = object[key];
        const isSkuMapContainer = key === 'skuMap' || key === 'skuInfoMap' || key === 'skuPriceMap';
        const isSkuPropContainer = /^(?:sku_props|skuProps|skuPropList|saleProp|saleProps)$/i.test(key);
        if (
          isSkuMapContainer &&
          nested &&
          typeof nested === 'object' &&
          !Array.isArray(nested)
        ) {
          skuMaps.push(nested as Record<string, unknown>);
        }
        if (key === 'skuModel' && nested && typeof nested === 'object' && !Array.isArray(nested)) {
          const skuModel = nested as Record<string, unknown>;
          if (skuModel.skuMap && typeof skuModel.skuMap === 'object' && !Array.isArray(skuModel.skuMap)) {
            skuMaps.push(skuModel.skuMap as Record<string, unknown>);
          }
          if (skuModel.skuInfoMap && typeof skuModel.skuInfoMap === 'object' && !Array.isArray(skuModel.skuInfoMap)) {
            skuMaps.push(skuModel.skuInfoMap as Record<string, unknown>);
          }
        }
        if (isSkuPropContainer) propArrays.push(nested);
        if (!isSkuMapContainer && !isSkuPropContainer) walk(nested, depth + 1);
      }
    };
    walk(root, 0);
    return propArrays;
  };
  for (const root of data ? [data, ...roots] : roots) propArrayRoots.push(collectSkuStructures(root));

  const dims: Dim[] = [];
  const structuredSkuImageRows: Array<{ specification: string; image: string }> = [];
  const structuredSkuImageSeen = new Set<string>();
  const skuImageFromObject = (value: unknown): string => {
    if (!value || typeof value !== 'object' || Array.isArray(value)) return '';
    const object = value as Record<string, unknown>;
    for (const key of [
      'image',
      'img',
      'pic',
      'picUrl',
      'imageUrl',
      'imageURL',
      'skuPicture',
      'skuPictureUrl',
    ]) {
      const direct = normalizeSkuImage(object[key]);
      if (direct) return direct;
      const nested = object[key];
      if (nested && typeof nested === 'object' && !Array.isArray(nested)) {
        const nestedObject = nested as Record<string, unknown>;
        for (const nestedKey of ['url', 'src', 'value']) {
          const image = normalizeSkuImage(nestedObject[nestedKey]);
          if (image) return image;
        }
      }
    }
    return '';
  };
  const pushStructuredSkuImage = (specificationRaw: unknown, value: unknown): void => {
    if (structuredSkuImageRows.length >= 200) return;
    const specification = trim(specificationRaw);
    const image = skuImageFromObject(value);
    if (!specification || specification.length > 200 || !image) return;
    const key = `${specification}\u0000${imageCanonical(image)}`;
    if (structuredSkuImageSeen.has(key)) return;
    structuredSkuImageSeen.add(key);
    structuredSkuImageRows.push({ specification, image });
  };
  const pushDim = (name: string, values: string[]) => {
    const n = trim(name).replace(/[:：\s]+$/u, '');
    const vs = [...new Set(values.map((v) => trim(v)).filter((v) => isValidSkuValue(v, n)))];
    if (!n || !vs.length) return;
    const existing = dims.find((d) => d.name === n);
    if (existing) {
      for (const v of vs) if (!existing.values.includes(v)) existing.values.push(v);
    } else dims.push({ name: n, values: vs });
  };
  for (const propArrays of propArrayRoots) {
    const visited = new WeakSet<object>();
    let remaining = 10_000;
    for (const sp of propArrays) {
      if (remaining-- <= 0) break;
      if (!Array.isArray(sp) || visited.has(sp)) continue;
      visited.add(sp);
      for (const row of sp) {
        if (remaining-- <= 0) break;
        if (!row || typeof row !== 'object' || Array.isArray(row) || visited.has(row)) continue;
        visited.add(row);
        const object = row as Record<string, unknown>;
        const name = trim(object.prop ?? object.name ?? object.fname ?? object.label ?? '');
        const valuesRaw = object.value ?? object.values ?? object.vlist ?? object.skus ?? object.list;
        const parts = Array.isArray(valuesRaw)
          ? valuesRaw
          : valuesRaw && typeof valuesRaw === 'object' && Array.isArray((valuesRaw as { list?: unknown[] }).list)
            ? ((valuesRaw as { list: unknown[] }).list ?? [])
            : [];
        if (remaining-- <= 0) break;
        const labels: string[] = [];
        for (const part of parts) {
          if (remaining-- <= 0) break;
          if (!part || typeof part !== 'object' || Array.isArray(part) || visited.has(part)) continue;
          visited.add(part);
          const partObject = part as Record<string, unknown>;
          const label = trim(
            partObject.name ?? partObject.value ?? partObject.text ?? partObject.vname ?? partObject.label ?? '',
          );
          if (label) {
            labels.push(label);
            pushStructuredSkuImage(label, partObject);
          }
        }
        if (name && labels.length) pushDim(name, labels);
      }
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
      const image = skuImageFromObject(bucket) || undefined;
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

  const skuValuesForPurchaseProperties = skus.map((sku) =>
    Object.entries(sku.properties ?? {})
      .filter(([name]) => name !== '产品尺寸')
      .map(([, value]) => normalizeProductDimensionCell(value)),
  );
  const skuPropertyAssignments = new Map<number, Map<string, Record<string, string>>>();
  for (const row of domSkuPropertyRows) {
    const specification = normalizeProductDimensionCell(row.specification);
    if (!specification) continue;
    const matches = skuValuesForPurchaseProperties
      .map((values, index) => (values.includes(specification) ? index : -1))
      .filter((index) => index >= 0);
    if (matches.length !== 1) continue;
    const normalizedEntries =
      Object.entries(row.properties)
        .map(([name, value]) => [normalizeSkuPropertyHeader(name), normalizeProductDimensionCell(value)] as const)
        .filter(([name, value]) => name && value && value.length <= 200 && !isSkuPurchaseOperationHeader(name));
    if (new Set(normalizedEntries.map(([name]) => name)).size !== normalizedEntries.length) continue;
    const specificationEntries = normalizedEntries.filter(([name]) => /^(?:产品规格|商品规格|规格)$/.test(name));
    if (specificationEntries.length !== 1 || specificationEntries[0]![1] !== specification) continue;
    const properties = Object.fromEntries(normalizedEntries);
    if (Object.keys(properties).length < 2) continue;
    const signature = JSON.stringify(Object.entries(properties).sort(([left], [right]) => left.localeCompare(right)));
    const assignments = skuPropertyAssignments.get(matches[0]!) ?? new Map<string, Record<string, string>>();
    assignments.set(signature, properties);
    skuPropertyAssignments.set(matches[0]!, assignments);
  }
  skus.forEach((sku, index) => {
    const assignments = skuPropertyAssignments.get(index);
    if (!assignments || assignments.size !== 1) return;
    const properties = [...assignments.values()][0]!;
    const merged = { ...(sku.properties ?? {}) };
    const existingByNormalizedName = new Map<string, Array<{ rawName: string; value: string }>>();
    for (const [rawName, rawValue] of Object.entries(merged)) {
      const normalizedName = normalizeSkuPropertyHeader(rawName);
      if (!normalizedName) continue;
      const existing = existingByNormalizedName.get(normalizedName) ?? [];
      existing.push({ rawName, value: normalizeProductDimensionCell(rawValue) });
      existingByNormalizedName.set(normalizedName, existing);
    }
    for (const [name, value] of Object.entries(properties)) {
      const existing = existingByNormalizedName.get(name) ?? [];
      if (existing.length > 1 || (existing.length === 1 && existing[0]!.value !== value)) return;
    }
    const sourceSpecificationName = Object.keys(properties).find((name) => /^(?:产品规格|商品规格)$/.test(name));
    const genericSpecifications = existingByNormalizedName.get('规格') ?? [];
    if (sourceSpecificationName && genericSpecifications.length) {
      if (
        genericSpecifications.length !== 1 ||
        genericSpecifications[0]!.value !== properties[sourceSpecificationName]
      ) return;
      delete merged[genericSpecifications[0]!.rawName];
    }
    for (const [name, value] of Object.entries(properties)) {
      const existing = existingByNormalizedName.get(name);
      if (existing?.length === 1 && existing[0]!.rawName !== name) delete merged[existing[0]!.rawName];
      merged[name] = value;
    }
    sku.properties = merged;
  });

  const skuValuesForProductDimension = skus.map((sku) =>
    Object.entries(sku.properties ?? {})
      .filter(([key]) => key !== '产品尺寸')
      .map(([, value]) => normalizeProductDimensionCell(value)),
  );
  const productDimensionAssignments = new Map<number, Set<string>>();
  for (const row of productDimensionRows) {
    const specificationMatches = skuValuesForProductDimension
      .map((values, index) => (values.includes(row.specification) ? index : -1))
      .filter((index) => index >= 0);
    let skuIndex = specificationMatches.length === 1 ? specificationMatches[0] : undefined;
    if (specificationMatches.length === 0 && row.productName) {
      const productNameMatches = skuValuesForProductDimension
        .map((values, index) => (values.includes(row.productName) ? index : -1))
        .filter((index) => index >= 0);
      if (productNameMatches.length === 1) skuIndex = productNameMatches[0];
    }
    if (skuIndex === undefined) continue;
    const dimensions = productDimensionAssignments.get(skuIndex) ?? new Set<string>();
    dimensions.add(row.productDimension);
    productDimensionAssignments.set(skuIndex, dimensions);
  }
  skus.forEach((sku, index) => {
    if (sku.properties?.['产品尺寸']) return;
    const dimensions = productDimensionAssignments.get(index);
    if (!dimensions || dimensions.size !== 1) return;
    sku.properties = { ...(sku.properties ?? {}), 产品尺寸: [...dimensions][0]! };
  });

  const skuValuesForImage = skus.map((sku) =>
    Object.entries(sku.properties ?? {})
      .filter(([key]) => key !== '产品尺寸')
      .map(([, value]) => normalizeProductDimensionCell(value)),
  );
  const collectSkuImageAssignments = (rows: Array<{ specification: string; image: string }>) => {
    const assignments = new Map<number, Map<string, string>>();
    for (const row of rows) {
      const specification = normalizeProductDimensionCell(row.specification);
      const image = normalizeSkuImage(row.image);
      if (!specification || !image) continue;
      const matches = skuValuesForImage
        .map((values, index) => (values.includes(specification) ? index : -1))
        .filter((index) => index >= 0);
      if (matches.length !== 1) continue;
      const images = assignments.get(matches[0]!) ?? new Map<string, string>();
      images.set(imageCanonical(image), image);
      assignments.set(matches[0]!, images);
    }
    return assignments;
  };
  const structuredSkuImageAssignments = collectSkuImageAssignments(structuredSkuImageRows);
  const domSkuImageAssignments = collectSkuImageAssignments(domSkuImageRows);
  skus.forEach((sku, index) => {
    if (normalizeSkuImage(sku.image)) return;
    const structuredImages = structuredSkuImageAssignments.get(index);
    const images = structuredImages ?? domSkuImageAssignments.get(index);
    if (!images || images.size !== 1) return;
    sku.image = [...images.values()][0]!;
  });

  const title = extractTitle();
  if (!title || title.length < 4) {
    fail('PARSE_FAILED', '无法读取商品标题，页面结构可能已变化或未加载完成');
  }
  if (!mainImages.length) {
    fail('PARSE_FAILED', '无法读取商品主图，请确认页面已完整加载且未触发验证');
  }
  const mainDescription = extractMainDescription(title);

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
    mainDescription,
    mainImages,
    descriptionImages,
    packaging,
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
