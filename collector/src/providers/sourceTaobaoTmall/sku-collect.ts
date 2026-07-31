import type { Page } from 'playwright';
import type { ProductSku } from '../../types/product.js';
import type { TaobaoPagePayload } from './page-extract.js';

export type TaobaoSkuGroupOption = {
  name: string;
  available: boolean;
  imageUrl: string;
};

export type TaobaoSkuGroup = {
  name: string;
  options: TaobaoSkuGroupOption[];
};

export type TaobaoSkuCollectOptions = {
  enabled: boolean;
  maxClicks: number;
  /** 按 skuId 探测 per-SKU 价格的最大数量（新版天猫 SSR 初始不返回 SKU 价格） */
  maxPriceProbes: number;
};

export type TaobaoSkuPriceProbeResult = {
  skuId: string;
  priceText?: string;
  originalPriceText?: string;
  quantity?: number;
  quantityText?: string;
  logisticsTime?: string;
  /** 探测失败信息（仅错误条目携带） */
  error?: string;
};

// 页面内 fetch 各 skuId 的 SSR 页面并提取 per-SKU 价格/库存。
// 与 collector/opencli-adapters/tmall/product.js 的 PROBE_SKU_PRICE_JS 保持同步。
// 注意：必须串行执行、单条 300–800ms 随机延迟、连续失败提前停止，
// 全程复用当前标签页（同源 fetch），绝不并发、不新开窗口。
export const SKU_PRICE_PROBE_PAGE_JS = `(async (payloadJson) => {
  const payload = JSON.parse(payloadJson);
  const { itemId, host, skuIds } = payload;
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

  const extractContext = (html) => {
    const scripts = [];
    const re = /<script[^>]*>([\\s\\S]*?)<\\/script>/g;
    let m;
    while ((m = re.exec(html))) {
      const t = m[1];
      if (t.indexOf('__ICE_APP_CONTEXT__') >= 0 && t.indexOf('loaderData') >= 0 && t.indexOf('var b = {') >= 0) {
        scripts.push(t);
      }
    }
    for (const t of scripts) {
      const start = t.indexOf('var b = {');
      if (start < 0) continue;
      let i = t.indexOf('{', start);
      let depth = 0;
      let inStr = false;
      let esc = false;
      for (let j = i; j < t.length; j++) {
        const c = t[j];
        if (inStr) {
          if (esc) esc = false;
          else if (c === '\\\\') esc = true;
          else if (c === '"') inStr = false;
          continue;
        }
        if (c === '"') { inStr = true; continue; }
        if (c === '{') depth++;
        else if (c === '}') {
          depth--;
          if (depth === 0) {
            try { return JSON.parse(t.slice(i, j + 1)); } catch (e) { return null; }
          }
        }
      }
    }
    return null;
  };

  const probeOne = async (skuId) => {
    const u = 'https://' + host + '/item.htm?id=' + encodeURIComponent(itemId) + '&skuId=' + encodeURIComponent(skuId);
    try {
      const ctrl = typeof AbortController !== 'undefined' ? new AbortController() : null;
      const timer = ctrl ? setTimeout(() => ctrl.abort(), 10000) : null;
      const res = await fetch(u, { credentials: 'include', signal: ctrl ? ctrl.signal : undefined });
      if (timer) clearTimeout(timer);
      if (!res.ok) return { skuId, error: 'HTTP ' + res.status };
      const html = await res.text();
      const ctx = extractContext(html);
      if (!ctx) return { skuId, error: 'no-context' };
      const data = ctx && ctx.loaderData && ctx.loaderData.home && ctx.loaderData.home.data && ctx.loaderData.home.data.res;
      const info = data && data.skuCore && data.skuCore.sku2info && data.skuCore.sku2info[skuId];
      if (!info) return { skuId, error: 'no-sku2info' };
      return {
        skuId,
        priceText: (info.subPrice && info.subPrice.priceText) || '',
        originalPriceText: (info.price && info.price.priceText) || '',
        quantity: info.quantity != null ? Number(info.quantity) : undefined,
        quantityText: info.quantityText || '',
        logisticsTime: info.logisticsTime || '',
      };
    } catch (e) {
      return { skuId, error: String(e).slice(0, 120) };
    }
  };

  const results = {};
  let consecutiveFailures = 0;
  const limit = Math.min(skuIds.length, 48);
  // 串行执行：一次一个 fetch，固定间隔更易被识别，因此用随机延迟。
  for (let idx = 0; idx < limit; idx++) {
    const cur = skuIds[idx];
    const r = await probeOne(cur);
    if (r && !r.error) results[r.skuId] = r;
    consecutiveFailures = r && r.error ? consecutiveFailures + 1 : 0;
    if (consecutiveFailures >= 4) break;
    if (idx < limit - 1) await sleep(300 + Math.random() * 500);
  }
  return results;
})`;

export async function collectSkuPricesByFetch(
  page: Page,
  sourceUrl: string,
  skuIds: string[],
  max: number,
): Promise<Record<string, TaobaoSkuPriceProbeResult>> {
  if (skuIds.length === 0 || max <= 0) return {};
  let itemId = '';
  let host = '';
  try {
    const parsed = new URL(sourceUrl);
    itemId = parsed.searchParams.get('id') ?? '';
    host = parsed.hostname;
  } catch {
    return {};
  }
  if (!itemId || !host) return {};
  const targets = [...new Set(skuIds.map((s) => s.trim()).filter(Boolean))].slice(0, Math.min(max, 48));
  if (targets.length === 0) return {};
  const probePayload = JSON.stringify({ itemId, host, skuIds: targets });
  const result = (await page
    .evaluate(`${SKU_PRICE_PROBE_PAGE_JS}(${JSON.stringify(probePayload)})`)
    .catch(() => null)) as Record<string, TaobaoSkuPriceProbeResult> | null;
  return result ?? {};
}

function parsePriceFromProbe(raw: unknown): number | undefined {
  if (typeof raw === 'number' && Number.isFinite(raw) && raw > 0) return raw;
  const text = String(raw ?? '').replace(/,/g, '');
  const m = text.match(/(\d+(?:\.\d{1,2})?)/);
  if (!m) return undefined;
  const n = Number(m[1]);
  return Number.isFinite(n) && n > 0 ? n : undefined;
}

function parseQuantityFromProbe(raw: unknown): number | undefined {
  if (raw === undefined || raw === null) return undefined;
  if (typeof raw === 'number') return Number.isFinite(raw) ? Math.max(0, Math.floor(raw)) : undefined;
  const t = String(raw).replace(/,/g, '').trim();
  if (!/^\d+(?:\.\d+)?$/.test(t)) return undefined;
  const n = Number(t);
  return Number.isFinite(n) ? Math.max(0, Math.floor(n)) : undefined;
}

// 把 skuId 价格探测结果合并进 skus（按 skuCode 匹配，不覆盖已有有效价格）。
export function mergeSkuPriceProbe(
  base: ProductSku[],
  probes: Record<string, TaobaoSkuPriceProbeResult>,
): ProductSku[] {
  const hasAny = Object.keys(probes).some(
    (key) =>
      probes[key] &&
      typeof probes[key] === 'object' &&
      !probes[key].error &&
      (probes[key].priceText || probes[key].originalPriceText || probes[key].quantity != null),
  );
  if (!hasAny) return base;
  return base.map((s) => {
    const key = String(s.skuCode ?? s.id ?? '').trim();
    const p = key ? probes[key] : undefined;
    if (!p || typeof p !== 'object' || (p as { error?: string }).error) return s;
    const probePrice = parsePriceFromProbe(p.priceText) ?? parsePriceFromProbe(p.originalPriceText);
    const probeStock = parseQuantityFromProbe(p.quantity);
    return {
      ...s,
      price: probePrice && probePrice > 0 ? probePrice : s.price,
      stock: probeStock !== undefined ? probeStock : s.stock,
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

function parsePriceFromText(text: string): number | undefined {
  const t = text.replace(/,/g, '').trim();
  const m = t.match(/(\d+(?:\.\d{1,2})?)/);
  if (!m) return undefined;
  const n = Number(m[1]);
  return Number.isFinite(n) && n > 0 ? n : undefined;
}

function buildSkuName(attrs: Record<string, string>): string {
  return Object.values(attrs).filter(Boolean).join(' / ');
}

function cartesianCombos(
  groups: TaobaoSkuGroup[],
  max: number,
): { attrs: Record<string, string>; available: boolean; imageUrl: string }[] {
  let combos: { attrs: Record<string, string>; available: boolean; imageUrl: string }[] = [
    { attrs: {}, available: true, imageUrl: '' },
  ];
  for (const g of groups) {
    const opts = g.options.filter((o) => o.name);
    if (!opts.length) continue;
    const next: typeof combos = [];
    for (const combo of combos) {
      for (const opt of opts) {
        next.push({
          attrs: { ...combo.attrs, [g.name]: opt.name },
          available: combo.available && opt.available,
          imageUrl: opt.imageUrl || combo.imageUrl,
        });
      }
    }
    combos = next.length ? next : combos;
    if (combos.length > max) break;
  }
  return combos.slice(0, max);
}

export function toTaobaoSkuGroups(payload: TaobaoPagePayload): TaobaoSkuGroup[] {
  return payload.skuGroups.map((g) => ({
    name: g.name,
    options: g.options.map((o) => ({
      name: o.label,
      available: !o.disabled,
      imageUrl: '',
    })),
  }));
}

export async function collectSkuPricesByClick(
  page: Page,
  groups: TaobaoSkuGroup[],
  options: TaobaoSkuCollectOptions,
): Promise<ProductSku[]> {
  if (!options.enabled || groups.length === 0) return [];

  const combos = cartesianCombos(groups, Math.min(options.maxClicks, 48));
  if (combos.length > options.maxClicks) {
    return combos.slice(0, options.maxClicks).map((c) => ({
      properties: c.attrs,
      price: undefined,
      stock: undefined,
      image: c.imageUrl || undefined,
      raw: { fromSkuClick: false, available: c.available },
    }));
  }

  const skus: ProductSku[] = [];
  const priceSel =
    '[class*="Price--priceText"], [class*="priceText"], .tm-price, #J_StrPrice, [class*="Price"]';

  for (const combo of combos.slice(0, options.maxClicks)) {
    try {
      for (const [groupName, optName] of Object.entries(combo.attrs)) {
        const group = groups.find((g) => g.name === groupName);
        if (!group) continue;
        const opt = group.options.find((o) => o.name === optName);
        if (!opt || !opt.available) continue;

        const locator = page
          .locator(
            `[class*="SkuContent"] [class*="skuItem"], #J_isku [class*="prop"], .tm-sale-prop`,
          )
          .filter({ hasText: groupName })
          .locator('li, [class*="valueItem"], .tm-img-prop span')
          .filter({ hasText: optName })
          .first();

        if ((await locator.count()) > 0) {
          await locator.click({ timeout: 1500 }).catch(() => undefined);
          await page.waitForTimeout(280);
        }
      }

      const priceText = await page
        .locator(priceSel)
        .first()
        .textContent()
        .catch(() => '');
      const price = parsePriceFromText(priceText ?? '');

      skus.push({
        properties: combo.attrs,
        price,
        stock: undefined,
        image: combo.imageUrl || undefined,
        raw: { fromSkuClick: true, available: combo.available, priceText: priceText?.trim() },
      });
    } catch {
      skus.push({
        properties: combo.attrs,
        price: undefined,
        stock: undefined,
        image: combo.imageUrl || undefined,
        raw: { fromSkuClick: true, available: combo.available, clickFailed: true },
      });
    }
  }

  return skus;
}

export function mergeSkuResults(
  base: ProductSku[],
  clicked: ProductSku[],
): { skus: ProductSku[]; skuGroups: TaobaoSkuGroup[] } {
  if (!clicked.length) {
    return { skus: base, skuGroups: [] };
  }
  const byKey = new Map<string, ProductSku>();
  for (const s of base) {
    const key = JSON.stringify(s.properties ?? {});
    byKey.set(key, s);
  }
  for (const s of clicked) {
    const key = JSON.stringify(s.properties ?? {});
    const prev = byKey.get(key);
    if (!prev || (s.price && s.price > 0 && (!prev.price || prev.price <= 0))) {
      byKey.set(key, { ...prev, ...s, properties: s.properties });
    }
  }
  return { skus: [...byKey.values()], skuGroups: [] };
}
