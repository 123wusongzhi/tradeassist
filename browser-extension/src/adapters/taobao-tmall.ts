import type { BrowserCollectAdapter, ProductSku } from './types.js';
import type { NormalizedProduct } from '../types.js';

const SUPPORTED_HOSTS = new Set([
  'detail.tmall.com',
  'chaoshi.tmall.com',
  'detail.tmall.hk',
  'item.taobao.com',
  'ju.taobao.com',
  'world.taobao.com',
]);

export const MAX_SKU_PRICE_PROBES = 200;
export const DEFAULT_SKU_PRICE_PROBES = 24;

export function isSupportedTaobaoTmallURL(raw: string): boolean {
  try {
    const url = new URL(raw);
    return url.protocol === 'https:' && SUPPORTED_HOSTS.has(url.hostname.toLowerCase());
  } catch {
    return false;
  }
}

export type TaobaoTmallSkuGroup = {
  name: string;
  options: { label: string; selected: boolean; disabled: boolean }[];
};

// ---------- 纯函数：价格/数量解析（0 = 缺货） ----------
export function parseTaobaoPrice(raw: unknown): number | undefined {
  if (typeof raw === 'number' && Number.isFinite(raw) && raw > 0) return raw;
  const m = String(raw ?? '')
    .replace(/,/g, '')
    .match(/(\d+(?:\.\d{1,2})?)/);
  if (!m) return undefined;
  const n = Number(m[1]);
  return Number.isFinite(n) && n > 0 ? n : undefined;
}

export function parseTaobaoQuantity(raw: unknown): number | undefined {
  if (raw === undefined || raw === null) return undefined;
  if (typeof raw === 'number') return Number.isFinite(raw) ? Math.max(0, Math.floor(raw)) : undefined;
  const t = String(raw).replace(/,/g, '').trim();
  if (!/^\d+(?:\.\d+)?$/.test(t)) return undefined;
  const n = Number(t);
  return Number.isFinite(n) ? Math.max(0, Math.floor(n)) : undefined;
}

function priceFromInfo(info: Record<string, unknown>): number | undefined {
  const sub = info.subPrice && typeof info.subPrice === 'object' ? (info.subPrice as Record<string, unknown>) : undefined;
  const price = info.price && typeof info.price === 'object' ? (info.price as Record<string, unknown>) : undefined;
  // 注意：priceMoney 以“分”为单位，不能直接当“元”解析；统一使用 priceText。
  return parseTaobaoPrice(info.price) ?? parseTaobaoPrice(sub?.priceText) ?? parseTaobaoPrice(price?.priceText);
}

function originalPriceFromInfo(info: Record<string, unknown>): number | undefined {
  const price = info.price && typeof info.price === 'object' ? (info.price as Record<string, unknown>) : undefined;
  return parseTaobaoPrice(price?.priceText);
}

// ---------- 纯函数：由新版天猫 SSR 的 skuBase + skuCore.sku2info 构建 SKU ----------
export function buildSkusFromTaobaoSkuBase(
  skuBase: unknown,
  sku2info: unknown,
): { skuGroups: TaobaoTmallSkuGroup[]; skus: ProductSku[] } {
  const base = skuBase as { props?: unknown[]; skus?: unknown[]; skuMap?: Record<string, unknown> } | null;
  if (!base || !Array.isArray(base.props)) return { skuGroups: [], skus: [] };
  const propsRaw = base.props;

  const propMeta = new Map<
    string,
    { name: string; values: Map<string, { name: string; image?: string }> }
  >();
  for (const p of propsRaw) {
    if (!p || typeof p !== 'object') continue;
    const po = p as Record<string, unknown>;
    const pid = String(po.pid ?? po.propertyId ?? po.propId ?? po.id ?? '').trim();
    const name = String(po.name ?? po.propName ?? po.propertyName ?? '规格').trim() || '规格';
    const valuesRaw = po.values ?? po.value;
    if (!Array.isArray(valuesRaw)) continue;
    const values = new Map<string, { name: string; image?: string }>();
    for (const v of valuesRaw) {
      if (!v || typeof v !== 'object') continue;
      const vo = v as Record<string, unknown>;
      const vid = String(vo.vid ?? vo.valueId ?? vo.id ?? vo.name ?? '').trim();
      const label = String(vo.name ?? vo.valueName ?? vo.text ?? '').trim();
      if (!vid || !label) continue;
      const imgRaw = String(vo.image ?? vo.img ?? vo.pic ?? '').trim();
      values.set(vid, { name: label, image: imgRaw || undefined });
    }
    if (values.size) propMeta.set(pid || name, { name, values });
  }

  const skuGroups: TaobaoTmallSkuGroup[] = [];
  for (const meta of propMeta.values()) {
    skuGroups.push({
      name: meta.name,
      options: [...meta.values.values()].map((v) => ({
        label: v.name,
        selected: false,
        disabled: false,
      })),
    });
  }

  const infoMap =
    sku2info && typeof sku2info === 'object' ? (sku2info as Record<string, unknown>) : undefined;
  const skusRaw = Array.isArray(base.skus)
    ? base.skus
    : base.skuMap && typeof base.skuMap === 'object'
      ? Object.values(base.skuMap)
      : [];
  const skus: ProductSku[] = [];

  for (const s of skusRaw) {
    if (!s || typeof s !== 'object') continue;
    const so = s as Record<string, unknown>;
    const propPath = String(so.propPath ?? so.propPathStr ?? so.specId ?? '').trim();
    const properties: Record<string, string> = {};
    let image = '';
    if (propPath) {
      for (const seg of propPath.split(/[;；]/)) {
        const [pid, vid] = seg.split(':');
        if (!pid || !vid) continue;
        const meta = propMeta.get(pid.trim());
        const val = meta?.values.get(vid.trim());
        if (meta && val) {
          properties[meta.name] = val.name;
          if (val.image) image = val.image;
        }
      }
    }
    const skuId = String(so.skuId ?? so.skuid ?? so.id ?? '').trim();
    const info = skuId && infoMap ? (infoMap[skuId] as Record<string, unknown> | undefined) : undefined;
    const price =
      priceFromInfo(so as Record<string, unknown>) ?? (info ? priceFromInfo(info) : undefined);
    const originalPrice = info ? originalPriceFromInfo(info) : undefined;
    const stock =
      parseTaobaoQuantity(so.quantity ?? so.stock ?? so.amount) ??
      (info ? parseTaobaoQuantity(info.quantity ?? info.stock ?? info.amount) : undefined);
    if (Object.keys(properties).length === 0 && skuGroups.length === 1 && skuGroups[0]!.options.length === 1) {
      properties[skuGroups[0]!.name] = skuGroups[0]!.options[0]!.label;
    }
    if (Object.keys(properties).length === 0) continue;
    skus.push({
      properties,
      price,
      originalPrice,
      stock,
      stockStatus: typeof info?.quantityText === 'string' ? info.quantityText : undefined,
      logisticsTime: typeof info?.logisticsTime === 'string' ? info.logisticsTime : undefined,
      skuCode: skuId || undefined,
      image: image || undefined,
      raw: {
        source: 'skuBase',
        propPath,
        sku2info: info
          ? {
              quantity: info.quantity,
              quantityText: info.quantityText ?? '',
              logisticsTime: info.logisticsTime ?? '',
              moreQuantity: info.moreQuantity ?? '',
            }
          : undefined,
      },
    });
  }

  if (!skus.length && skuGroups.length) {
    // 组合兜底：无 skuId 明细时按规格组生成笛卡尔组合（无价格/库存信息）
    let combos: Record<string, string>[] = [{}];
    for (const g of skuGroups) {
      const opts = g.options.filter((o) => o.label && !o.disabled);
      if (!opts.length) continue;
      const next: Record<string, string>[] = [];
      for (const combo of combos) {
        for (const opt of opts) {
          next.push({ ...combo, [g.name]: opt.label });
        }
      }
      combos = next.length ? next : combos;
    }
    return {
      skuGroups,
      skus: combos.slice(0, 200).map((properties) => ({
        properties,
        raw: { fromDomGroups: true },
      })),
    };
  }
  return { skuGroups, skus };
}

// ---------- 纯函数：把 skuId 价格探测结果合并进 skus（探测结果优先） ----------
export function mergeSkuPriceProbeResults(
  skus: ProductSku[],
  probes: Record<string, unknown>,
): ProductSku[] {
  const hasAny = Object.keys(probes).some(
    (key) =>
      probes[key] &&
      typeof probes[key] === 'object' &&
      !(probes[key] as { error?: string }).error &&
      ((probes[key] as { priceText?: string }).priceText ||
        (probes[key] as { originalPriceText?: string }).originalPriceText ||
        (probes[key] as { quantity?: number }).quantity != null),
  );
  if (!hasAny) return skus;
  return skus.map((s) => {
    const key = String(s.skuCode ?? s.id ?? '').trim();
    const p = key ? (probes[key] as Record<string, unknown> | undefined) : undefined;
    if (!p || typeof p !== 'object' || (p as { error?: string }).error) return s;
    const probePrice = parseTaobaoPrice(p.priceText) ?? parseTaobaoPrice(p.originalPriceText);
    const probeOriginalPrice = parseTaobaoPrice(p.originalPriceText);
    const probeStock = parseTaobaoQuantity(p.quantity);
    return {
      ...s,
      price: probePrice && probePrice > 0 ? probePrice : s.price,
      originalPrice: probeOriginalPrice && probeOriginalPrice > 0 ? probeOriginalPrice : s.originalPrice,
      stock: probeStock !== undefined ? probeStock : s.stock,
      stockStatus: typeof p.quantityText === 'string' ? p.quantityText : s.stockStatus,
      logisticsTime: typeof p.logisticsTime === 'string' ? p.logisticsTime : s.logisticsTime,
      raw: {
        ...(s.raw ?? {}),
        skuPriceProbe: {
          priceText: p.priceText ?? '',
          originalPriceText: p.originalPriceText ?? '',
          quantity: p.quantity,
          quantityText: p.quantityText ?? '',
          logisticsTime: p.logisticsTime ?? '',
        },
      },
    };
  });
}

// This function is serialized by chrome.scripting.executeScript. Keep every
// page helper inside the function body; it must not close over extension state.
// 内部内联逻辑与上方 buildSkusFromTaobaoSkuBase / mergeSkuPriceProbeResults /
// parseTaobaoPrice / parseTaobaoQuantity 保持一致（executeScript 无法引用模块闭包）。
export async function collectTaobaoTmallPage(options?: { maxPriceProbes?: number }): Promise<NormalizedProduct> {
  const sleep = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));
  const text = (element: Element | null | undefined) =>
    (element?.textContent ?? '').replace(/\s+/g, ' ').trim();
  const firstText = (selectors: string[]) => {
    for (const selector of selectors) {
      const value = text(document.querySelector(selector));
      if (value) return value;
    }
    return '';
  };
  const parsePrice = (raw: unknown): number | undefined => {
    if (typeof raw === 'number' && Number.isFinite(raw) && raw > 0) return raw;
    const m = String(raw ?? '')
      .replace(/,/g, '')
      .match(/(\d+(?:\.\d{1,2})?)/);
    if (!m) return undefined;
    const value = Number(m[1]);
    return Number.isFinite(value) && value > 0 ? value : undefined;
  };
  const parseQuantity = (raw: unknown): number | undefined => {
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
    value = value.replace(/_(?:\d+x\d+)?\.(?:jpg|jpeg|png|webp)(?=$|\?)/i, '');
    const queryIndex = value.indexOf('?');
    if (queryIndex >= 0) value = value.slice(0, queryIndex);
    return value;
  };
  const uniqueImages = (values: Array<string | null | undefined>) => {
    const seen = new Set<string>();
    const result: string[] = [];
    for (const raw of values) {
      const value = normalizeImage(raw);
      const key = value.toLowerCase();
      if (!value || /(?:spaceball|\/s\.gif)/i.test(value) || seen.has(key)) continue;
      seen.add(key);
      result.push(value);
    }
    return result;
  };
  const fail = (code: string, message: string): never => {
    throw new Error(`${code}: ${message}`);
  };

  const pageURL = location.href;
  const pageHost = location.hostname.toLowerCase();
  if (
    ![
      'detail.tmall.com',
      'chaoshi.tmall.com',
      'detail.tmall.hk',
      'item.taobao.com',
      'ju.taobao.com',
      'world.taobao.com',
    ].includes(pageHost)
  ) {
    fail('UNSUPPORTED_PAGE', '当前标签页不是支持的淘宝或天猫商品详情页');
  }

  const originalScrollY = window.scrollY;
  const productSelectors = [
    '[class*="ItemHeader"]',
    '[class*="MainTitle"]',
    '#J_Title',
    '[class*="PicGallery"]',
    '#J_UlThumb',
    'h1',
  ];
  const waitStarted = Date.now();
  while (!productSelectors.some((selector) => document.querySelector(selector)) && Date.now() - waitStarted < 12_000) {
    await sleep(250);
  }

  const pageText = text(document.body).slice(0, 30_000);
  const documentTitle = document.title.trim();
  if (/验证码|安全验证|滑块|访问受限|verify|captcha/i.test(`${documentTitle} ${pageText.slice(0, 5000)}`)) {
    fail('VERIFY_REQUIRED', '页面需要安全验证，请完成验证后重试');
  }
  if (
    /亲，请登录|密码登录|短信登录|login\.taobao\.com/i.test(`${location.href} ${documentTitle} ${pageText.slice(0, 5000)}`)
  ) {
    fail('LOGIN_REQUIRED', '页面需要登录，请在当前浏览器登录淘宝后重试');
  }
  if (/商品不存在|宝贝不存在|已下架|页面不存在|很抱歉.*找不到/i.test(pageText.slice(0, 8000))) {
    fail('PRODUCT_NOT_FOUND', '商品不存在或已下架');
  }

  window.scrollBy({ top: 240, behavior: 'instant' });
  await sleep(350);
  window.scrollTo({ top: originalScrollY, behavior: 'instant' });

  const titleCandidates: string[] = [];
  for (const selector of ['[class*="MainTitle"]', '[class*="ItemHeader"] h1', '#J_Title', 'h1']) {
    const value = text(document.querySelector(selector));
    if (value && value.length > 2) titleCandidates.push(value);
  }
  const ogTitle = (document.querySelector('meta[property="og:title"]') as HTMLMetaElement | null)?.content?.trim();
  if (ogTitle) titleCandidates.push(ogTitle);
  let title =
    titleCandidates.find((value) => !/^(淘宝网|天猫|登录|首页)$/i.test(value)) ??
    documentTitle.replace(/[-_—|].*(淘宝网|天猫).*$/i, '').trim();
  title = title.replace(/\s+/g, ' ').trim();
  if (!title) fail('TITLE_NOT_FOUND', '未能读取商品标题');

  const priceText = firstText([
    '[class*="Price--priceText"]',
    '[class*="priceText"]',
    '.tm-price',
    '#J_StrPrice',
    '[class*="Price"]',
  ]);
  const productPrice = parsePrice(priceText);

  const mainImageCandidates: string[] = [];
  const ogImage = (document.querySelector('meta[property="og:image"]') as HTMLMetaElement | null)?.content;
  if (ogImage) mainImageCandidates.push(ogImage);
  for (const image of document.querySelectorAll<HTMLImageElement>(
    '[class*="PicGallery"] img, #J_UlThumb img, [class*="thumbnail"] img, [class*="mainPic"] img, #J_ImgBooth',
  )) {
    mainImageCandidates.push(
      image.currentSrc,
      image.src,
      image.getAttribute('data-src') ?? '',
      image.getAttribute('data-ks-lazyload') ?? '',
    );
  }
  const mainImages = uniqueImages(mainImageCandidates).slice(0, 80);
  if (mainImages.length === 0) fail('MAIN_IMAGES_EMPTY', '未能读取商品主图');

  const detailRoot =
    document.querySelector('#J_Description, #description, [class*="desc-root"], [class*="DetailDesc"]') ??
    document.querySelector('[id*="description"], [class*="Description"]');
  const detailImageCandidates: string[] = [];
  let mainDescription = '';
  if (detailRoot) {
    detailRoot.scrollIntoView({ block: 'start', behavior: 'instant' });
    for (let index = 0; index < 6; index++) {
      window.scrollBy({ top: Math.max(window.innerHeight * 0.75, 380), behavior: 'instant' });
      await sleep(260);
    }
    for (const image of detailRoot.querySelectorAll<HTMLImageElement>('img')) {
      detailImageCandidates.push(
        image.currentSrc,
        image.src,
        image.getAttribute('data-src') ?? '',
        image.getAttribute('data-ks-lazyload') ?? '',
      );
    }
    mainDescription = text(detailRoot).slice(0, 5000);
  }
  const descriptionImages = uniqueImages(detailImageCandidates)
    .filter((image) => !mainImages.includes(image))
    .slice(0, 400);
  window.scrollTo({ top: originalScrollY, behavior: 'instant' });

  const attributes: Record<string, string> = {};
  for (const item of document.querySelectorAll('[class*="emphasisParamsInfoItem"]')) {
    const value = text(item.querySelector('[class*="ItemTitle"]:not([class*="SubTitle"])'));
    const key = text(item.querySelector('[class*="ItemSubTitle"]'));
    if (key && value && key.length <= 40 && value.length <= 180) attributes[key] = value;
  }
  for (const row of document.querySelectorAll(
    '[class*="ItemParams"] li, [class*="attributes"] li, #attributes li, .tm-tableAttr tr',
  )) {
    const rowText = text(row);
    const parts = rowText.split(/[:：]/);
    const key = parts.shift()?.trim() ?? '';
    const value = parts.join(':').trim();
    if (key && value && key.length <= 40 && value.length <= 300) attributes[key] = value;
  }

  // ---------- SKU：新版 SSR 的 skuBase + skuCore.sku2info（含库存） ----------
  type SkuEntry = {
    properties: Record<string, string>;
    id?: string;
    price?: number;
    originalPrice?: number;
    stock?: number;
    stockStatus?: string;
    logisticsTime?: string;
    skuCode?: string;
    image?: string;
    raw?: Record<string, unknown>;
  };
  const ice = (window as unknown as Record<string, unknown>).__ICE_APP_CONTEXT__ as
    | { loaderData?: { home?: { data?: { res?: Record<string, unknown> } } } }
    | undefined;
  const res = ice?.loaderData?.home?.data?.res;
  const skuBase = res?.skuBase as { props?: unknown[]; skus?: unknown[] } | undefined;
  const sku2info =
    res?.skuCore && typeof res.skuCore === 'object'
      ? (res.skuCore as Record<string, unknown>).sku2info
      : undefined;
  let skus: SkuEntry[] = [];
  let rawSkuGroups: { name: string; options: { label: string; disabled: boolean }[] }[] = [];
  let sku2InfoMerged = false;
  let jsonSkuCount = 0;
  if (skuBase && Array.isArray(skuBase.props) && Array.isArray(skuBase.skus)) {
    const propMeta = new Map<string, { name: string; values: Map<string, { name: string; image?: string }> }>();
    for (const p of skuBase.props) {
      if (!p || typeof p !== 'object') continue;
      const po = p as Record<string, unknown>;
      const pid = String(po.pid ?? po.propertyId ?? po.propId ?? po.id ?? '').trim();
      const name = String(po.name ?? po.propName ?? po.propertyName ?? '规格').trim() || '规格';
      const valuesRaw = po.values ?? po.value;
      if (!Array.isArray(valuesRaw)) continue;
      const values = new Map<string, { name: string; image?: string }>();
      for (const v of valuesRaw) {
        if (!v || typeof v !== 'object') continue;
        const vo = v as Record<string, unknown>;
        const vid = String(vo.vid ?? vo.valueId ?? vo.id ?? vo.name ?? '').trim();
        const label = String(vo.name ?? vo.valueName ?? vo.text ?? '').trim();
        if (!vid || !label) continue;
        const imgRaw = String(vo.image ?? vo.img ?? vo.pic ?? '').trim();
        values.set(vid, { name: label, image: imgRaw || undefined });
      }
      if (values.size) propMeta.set(pid || name, { name, values });
    }
    const groups = [...propMeta.values()].map((meta) => ({
      name: meta.name,
      options: [...meta.values.values()].map((v) => ({
        label: v.name,
        selected: false,
        disabled: false,
      })),
    }));
    rawSkuGroups = groups.map((g) => ({
      name: g.name,
      options: g.options.map((o) => ({ label: o.label, disabled: o.disabled })),
    }));
    const infoMap =
      sku2info && typeof sku2info === 'object' ? (sku2info as Record<string, unknown>) : undefined;
    const built: SkuEntry[] = [];
    for (const s of skuBase.skus) {
      if (!s || typeof s !== 'object') continue;
      const so = s as Record<string, unknown>;
      const propPath = String(so.propPath ?? so.propPathStr ?? so.specId ?? '').trim();
      const properties: Record<string, string> = {};
      let image = '';
      if (propPath) {
        for (const seg of propPath.split(/[;；]/)) {
          const [pid, vid] = seg.split(':');
          if (!pid || !vid) continue;
          const meta = propMeta.get(pid.trim());
          const val = meta?.values.get(vid.trim());
          if (meta && val) {
            properties[meta.name] = val.name;
            if (val.image) image = val.image;
          }
        }
      }
      const skuId = String(so.skuId ?? so.skuid ?? so.id ?? '').trim();
      const info = skuId && infoMap ? (infoMap[skuId] as Record<string, unknown> | undefined) : undefined;
      const sub = info?.subPrice && typeof info.subPrice === 'object'
        ? (info.subPrice as Record<string, unknown>)
        : undefined;
      const priceObj = info?.price && typeof info.price === 'object'
        ? (info.price as Record<string, unknown>)
        : undefined;
      const price =
        parsePrice(so.price) ??
        parsePrice(sub?.priceText) ??
        parsePrice(priceObj?.priceText) ??
        parsePrice(sub?.priceMoney);
      // priceMoney 以“分”为单位，不能直接当“元”解析；原价统一取 priceText。
      const originalPrice = parsePrice(priceObj?.priceText);
      const stock =
        parseQuantity(so.quantity ?? so.stock ?? so.amount) ??
        (info ? parseQuantity(info.quantity ?? info.stock ?? info.amount) : undefined);
      if (Object.keys(properties).length === 0 && groups.length === 1 && groups[0]!.options.length === 1) {
        properties[groups[0]!.name] = groups[0]!.options[0]!.label;
      }
      if (Object.keys(properties).length === 0) continue;
      built.push({
        properties,
        price,
        originalPrice,
        stock,
        stockStatus: typeof info?.quantityText === 'string' ? info.quantityText : undefined,
        logisticsTime: typeof info?.logisticsTime === 'string' ? info.logisticsTime : undefined,
        skuCode: skuId || undefined,
        image: image || undefined,
        raw: {
          source: 'skuBase',
          propPath,
          sku2info: info
            ? {
                quantity: info.quantity,
                quantityText: info.quantityText ?? '',
                logisticsTime: info.logisticsTime ?? '',
                moreQuantity: info.moreQuantity ?? '',
              }
            : undefined,
        },
      });
    }
    skus = built;
    jsonSkuCount = built.length;
    sku2InfoMerged = Boolean(infoMap && Object.keys(infoMap).length > 0);
  }

  // ---------- SKU：老版页面回退（DOM 规格组 + 点击采价） ----------
  type SkuOption = { label: string; element: HTMLElement; selected: boolean; disabled: boolean };
  type SkuGroup = { name: string; options: SkuOption[] };
  const domGroups: SkuGroup[] = [];
  if (skus.length === 0) {
    const groupElements = document.querySelectorAll(
      '[class*="SkuContent"] [class*="skuItem"], #J_isku [class*="prop"], .tm-sale-prop',
    );
    for (const [groupIndex, group] of Array.from(groupElements).entries()) {
      const name =
        text(group.querySelector('[class*="label"], dt, .tm-prop-title'))
          .replace(/[:：]\s*$/, '')
          .trim() || `规格${groupIndex + 1}`;
      const options: SkuOption[] = [];
      for (const option of group.querySelectorAll<HTMLElement>('li, [class*="valueItem"], .tm-img-prop span')) {
        const label =
          option.getAttribute('title')?.trim() ||
          option.getAttribute('aria-label')?.trim() ||
          text(option);
        if (!label || label.length > 80 || options.some((item) => item.label === label)) continue;
        const signal = `${option.className} ${text(option)}`;
        options.push({
          label,
          element: option,
          selected: /selected|checked|current/i.test(signal) || option.getAttribute('aria-checked') === 'true',
          disabled: /disabled|soldout|无货|缺货/i.test(signal) || option.getAttribute('aria-disabled') === 'true',
        });
      }
      if (options.length) domGroups.push({ name, options });
    }
    rawSkuGroups = domGroups.map((g) => ({
      name: g.name,
      options: g.options.map((o) => ({ label: o.label, disabled: o.disabled })),
    }));

    type Combination = { properties: Record<string, string>; options: SkuOption[] };
    let combinations: Combination[] = [{ properties: {}, options: [] }];
    for (const group of domGroups) {
      const available = group.options.filter((option) => !option.disabled).slice(0, 24);
      if (!available.length) continue;
      const next: Combination[] = [];
      for (const combination of combinations) {
        for (const option of available) {
          next.push({
            properties: { ...combination.properties, [group.name]: option.label },
            options: [...combination.options, option],
          });
          if (next.length >= 24) break;
        }
        if (next.length >= 24) break;
      }
      combinations = next.length ? next : combinations;
    }

    if (domGroups.length && combinations.length) {
      const originalSelections = domGroups.map((group) => group.options.find((option) => option.selected));
      for (const combination of combinations) {
        try {
          for (const option of combination.options) {
            if (!option.element.isConnected) continue;
            option.element.click();
            await sleep(150);
          }
          const currentPriceText = firstText([
            '[class*="Price--priceText"]',
            '[class*="priceText"]',
            '.tm-price',
            '#J_StrPrice',
            '[class*="Price"]',
          ]);
          skus.push({
            properties: combination.properties,
            price: parsePrice(currentPriceText) ?? productPrice,
            raw: { fromBrowserInteraction: true, priceText: currentPriceText },
          });
        } catch {
          skus.push({
            properties: combination.properties,
            price: productPrice,
            raw: { fromBrowserInteraction: true, clickFailed: true },
          });
        }
      }
      for (const selected of originalSelections) {
        if (selected?.element.isConnected) {
          selected.element.click();
          await sleep(100);
        }
      }
    } else if (productPrice) {
      skus.push({ properties: { 规格: '默认规格' }, price: productPrice });
    }
  }
  window.scrollTo({ top: originalScrollY, behavior: 'instant' });

  // ---------- SKU 价格探测（新版 SSR）：串行、300–800ms 随机、复用当前标签页 ----------
  const maxPriceProbes = Math.max(
    0,
    Math.min(200, Math.floor(options?.maxPriceProbes ?? 24) || 24),
  );
  let skuPriceProbeCount = 0;
  if (maxPriceProbes > 0 && skus.length > 0) {
    const itemId = new URL(pageURL).searchParams.get('id') ?? '';
    if (itemId && pageHost) {
      const missing = skus
        .filter((s) => !s.price || s.price <= 0)
        .map((s) => String(s.skuCode ?? s.id ?? '').trim())
        .filter(Boolean);
      const targets = [...new Set(missing)].slice(0, Math.min(maxPriceProbes, 200));
      if (targets.length > 0) {
        const probes: Record<string, unknown> = {};
        let consecutiveFailures = 0;
        for (let index = 0; index < targets.length; index++) {
          const skuId = targets[index]!;
          let probeError = '';
          let info: Record<string, unknown> | undefined;
          try {
            const url = `https://${pageHost}/item.htm?id=${encodeURIComponent(itemId)}&skuId=${encodeURIComponent(skuId)}`;
            const controller = typeof AbortController !== 'undefined' ? new AbortController() : null;
            const timer = controller ? setTimeout(() => controller.abort(), 10_000) : null;
            const response = await fetch(url, {
              credentials: 'include',
              signal: controller ? controller.signal : undefined,
            });
            if (timer) clearTimeout(timer);
            if (!response.ok) {
              probeError = `HTTP ${response.status}`;
            } else {
              const html = await response.text();
              const scripts: string[] = [];
              const scriptRe = /<script[^>]*>([\s\S]*?)<\/script>/g;
              let match: RegExpExecArray | null;
              while ((match = scriptRe.exec(html))) {
                const body = match[1] ?? '';
                if (
                  body.includes('__ICE_APP_CONTEXT__') &&
                  body.includes('loaderData') &&
                  body.includes('var b = {')
                ) {
                  scripts.push(body);
                }
              }
              for (const body of scripts) {
                const start = body.indexOf('var b = {');
                if (start < 0) continue;
                let depth = 0;
                let inString = false;
                let escaped = false;
                for (let j = body.indexOf('{', start); j < body.length; j++) {
                  const char = body[j]!;
                  if (inString) {
                    if (escaped) escaped = false;
                    else if (char === '\\') escaped = true;
                    else if (char === '"') inString = false;
                    continue;
                  }
                  if (char === '"') {
                    inString = true;
                    continue;
                  }
                  if (char === '{') depth++;
                  else if (char === '}') {
                    depth--;
                    if (depth === 0) {
                      try {
                        const ctx = JSON.parse(body.slice(body.indexOf('{', start), j + 1)) as {
                          loaderData?: { home?: { data?: { res?: Record<string, unknown> } } };
                        };
                        const res2 = ctx?.loaderData?.home?.data?.res;
                        const s2i =
                          res2?.skuCore && typeof res2.skuCore === 'object'
                            ? (res2.skuCore as Record<string, unknown>).sku2info
                            : undefined;
                        info = s2i && typeof s2i === 'object'
                          ? (s2i as Record<string, unknown>)[skuId] as Record<string, unknown> | undefined
                          : undefined;
                      } catch {
                        info = undefined;
                      }
                      break;
                    }
                  }
                }
                if (info) break;
              }
              if (!info) probeError = 'no-sku2info';
            }
          } catch (error) {
            probeError = String(error).slice(0, 120);
          }
          if (!probeError && info) {
            const sub = info.subPrice && typeof info.subPrice === 'object'
              ? (info.subPrice as Record<string, unknown>)
              : undefined;
            const priceObj = info.price && typeof info.price === 'object'
              ? (info.price as Record<string, unknown>)
              : undefined;
            probes[skuId] = {
              skuId,
              priceText: typeof sub?.priceText === 'string' ? sub.priceText : '',
              originalPriceText: typeof priceObj?.priceText === 'string' ? priceObj.priceText : '',
              quantity: info.quantity != null ? Number(info.quantity) : undefined,
              quantityText: typeof info.quantityText === 'string' ? info.quantityText : '',
              logisticsTime: typeof info.logisticsTime === 'string' ? info.logisticsTime : '',
            };
            consecutiveFailures = 0;
          } else {
            consecutiveFailures += 1;
          }
          if (consecutiveFailures >= 4) break;
          if (index < targets.length - 1) {
            // 随机延迟：固定间隔更易被识别
            await sleep(300 + Math.random() * 500);
          }
        }
        if (Object.keys(probes).length > 0) {
          const merged = skus.map((s) => {
            const key = String(s.skuCode ?? s.id ?? '').trim();
            const probe = key ? (probes[key] as Record<string, unknown> | undefined) : undefined;
            if (!probe) return s;
            const probePrice = parsePrice(probe.priceText) ?? parsePrice(probe.originalPriceText);
            const probeOriginalPrice = parsePrice(probe.originalPriceText);
            const probeStock = parseQuantity(probe.quantity);
            return {
              ...s,
              price: probePrice && probePrice > 0 ? probePrice : s.price,
              originalPrice:
                probeOriginalPrice && probeOriginalPrice > 0 ? probeOriginalPrice : s.originalPrice,
              stock: probeStock !== undefined ? probeStock : s.stock,
              stockStatus: typeof probe.quantityText === 'string' ? probe.quantityText : s.stockStatus,
              logisticsTime: typeof probe.logisticsTime === 'string' ? probe.logisticsTime : s.logisticsTime,
              raw: {
                ...(s.raw ?? {}),
                skuPriceProbe: {
                  priceText: probe.priceText ?? '',
                  originalPriceText: probe.originalPriceText ?? '',
                  quantity: probe.quantity,
                  quantityText: probe.quantityText ?? '',
                  logisticsTime: probe.logisticsTime ?? '',
                },
              },
            };
          });
          skus = merged;
          skuPriceProbeCount = Object.keys(probes).length;
        }
      }
    }
  }

  const skusFinal: SkuEntry[] = skus.map((s) => ({
    ...s,
    price: s.price && s.price > 0 ? s.price : productPrice && productPrice > 0 ? productPrice : s.price,
  }));

  const warnings: string[] = [];
  if (!productPrice) warnings.push('PRICE_NOT_FOUND');
  if (!descriptionImages.length) warnings.push('DETAIL_IMAGES_INCOMPLETE');
  if (!rawSkuGroups.length || !skusFinal.length) warnings.push('SKU_INCOMPLETE');
  if (!Object.keys(attributes).length) warnings.push('ATTRIBUTES_EMPTY');
  if (skusFinal.length > 0 && skusFinal.some((s) => s.stock == null)) warnings.push('STOCK_UNKNOWN');

  return {
    source: 'taobao_tmall',
    sourceUrl: pageURL,
    title,
    currency: 'CNY',
    mainDescription,
    mainImages,
    descriptionImages,
    attributes,
    skus: skusFinal,
    raw: {
      provider: 'browser_extension',
      schemaVersion: 2,
      capturedAt: new Date().toISOString(),
      pageTitle: documentTitle,
      finalUrl: location.href,
      productPrice,
      priceText,
      qualityWarnings: warnings,
      skuGroups: rawSkuGroups,
      sku2InfoMerged,
      skuPriceProbeCount,
      jsonSkuCount,
    },
  };
}

export const taobaoTmallAdapter: BrowserCollectAdapter = {
  id: 'taobao_tmall',
  label: '淘宝 / 天猫',
  supports: isSupportedTaobaoTmallURL,
  // 必须直接引用完整函数：chrome.scripting.executeScript 会序列化函数源码注入
  // 页面，箭头包装会引用模块闭包导致 ReferenceError（页面采集 EMPTY_RESULT）。
  collect: collectTaobaoTmallPage,
};
