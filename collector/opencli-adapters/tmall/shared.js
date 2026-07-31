// shared.js — 从 TradeMind collector/src/providers/sourceTaobaoTmall 移植的纯函数
// 来源文件: image-utils.ts / title-utils.ts / json-extract.ts / parser.ts / quality.ts / auth-detect.ts
// 全部不依赖浏览器 API，Node 侧运行。

// ---------- image-utils ----------
const ICON_RE =
  /avatar|icon|logo|sprite|placeholder|blank|loading|1x1|emoji|kefu|service|comment|rate|star|shop-logo|seller/i;
const SMALL_DIM_RE = /_\d{1,2}x\d{1,2}\./i;
const PLACEHOLDER_RE = /\/s\.gif(?:\?|$)|\/spaceball\.gif|\/assets\/.*loading|1x1\.gif/i;

export function normalizeImageUrl(raw) {
  let u = String(raw ?? '').trim();
  if (!u) return '';
  if (u.startsWith('//')) u = `https:${u}`;
  if (u.startsWith('data:')) return '';
  u = u.replace(/\.webp(\?|$)/i, '.jpg$1');
  u = u.replace(/_\d+x\d+\.(jpg|jpeg|png)/i, '.$1');
  u = u.replace(/_\d+x\d+q\d+\.(jpg|jpeg|png)/i, '.$1');
  u = u.replace(/_\.(?:jpg|jpeg|png|webp)$/i, '');
  u = u.replace(/(\.(?:jpg|jpeg|png|webp))(?:_\d+x\d+)?_\.(?:jpg|jpeg|png|webp)$/i, '$1');
  return u.split('?')[0] ?? u;
}

export function isLikelyProductImage(url, width, height) {
  const u = String(url ?? '').toLowerCase();
  if (!u || u.startsWith('data:')) return false;
  if (PLACEHOLDER_RE.test(u)) return false;
  if (ICON_RE.test(u)) return false;
  if (SMALL_DIM_RE.test(u)) return false;
  if (typeof width === 'number' && typeof height === 'number' && width > 0 && height > 0) {
    if (width < 80 || height < 80) return false;
    if (width === height && width < 120) return false;
  }
  return /alicdn\.com|tbcdn\.cn|taobaocdn\.com|tmall\.com|1688\.com/i.test(u) || u.startsWith('http');
}

export function dedupeUrls(urls) {
  const seen = new Set();
  const out = [];
  for (const raw of urls) {
    const u = normalizeImageUrl(raw);
    if (!u || !isLikelyProductImage(u)) continue;
    const key = u.toLowerCase();
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(u.startsWith('http') ? u : String(raw).trim());
  }
  return out;
}

// ---------- title-utils ----------
const PLATFORM_TITLE_RE = /^(淘宝网|天猫|天猫超市|聚划算|淘宝全球购)\s*[-–—|｜]?\s*/i;
const SHOP_SUFFIX_RE = /\s*[-–—|｜]\s*[^-–—|｜]{1,40}?(?:旗舰店|专卖店|专营店|官方店|自营店|店)\s*$/i;

export function cleanTaobaoTitle(raw) {
  let t = String(raw ?? '')
    .replace(/\s+/g, ' ')
    .trim();
  if (!t) return '';
  t = t.replace(PLATFORM_TITLE_RE, '');
  t = t.replace(/\s*[-–—|｜]\s*(淘宝网|天猫|天猫超市)\s*$/i, '');
  t = t.replace(SHOP_SUFFIX_RE, '');
  t = t.replace(/\s*[-–—|｜]\s*淘宝网\s*$/i, '');
  t = t.replace(/\s*[-–—|｜]\s*天猫\s*$/i, '');
  return t.replace(/\s+/g, ' ').trim();
}

export function extractTitleFromDocumentTitle(docTitle) {
  const t = cleanTaobaoTitle(docTitle);
  if (!t || /^(淘宝网|天猫|登录|首页)$/i.test(t)) return '';
  return t;
}

// ---------- price ----------
export function parsePrice(raw) {
  if (typeof raw === 'number' && Number.isFinite(raw) && raw > 0) return raw;
  const text = String(raw ?? '').replace(/,/g, '');
  const m = text.match(/(\d+(?:\.\d{1,2})?)/);
  if (!m) return undefined;
  const n = Number(m[1]);
  return Number.isFinite(n) && n > 0 ? n : undefined;
}

// 数量解析：保留 0（0 = 无货/缺货），parsePrice 会把 0 视为无效。
export function parseQuantity(raw) {
  if (raw === undefined || raw === null) return undefined;
  if (typeof raw === 'number') return Number.isFinite(raw) ? Math.max(0, Math.floor(raw)) : undefined;
  const t = String(raw).replace(/,/g, '').trim();
  if (!/^\d+(?:\.\d+)?$/.test(t)) return undefined;
  const n = Number(t);
  return Number.isFinite(n) ? Math.max(0, Math.floor(n)) : undefined;
}

// ---------- json-extract: skuBase 构建 ----------
export function cartesianFromGroups(groups, max = 200) {
  if (!groups.length) return [];
  let combos = [{}];
  for (const g of groups) {
    const opts = g.options.filter((o) => o.label && !o.disabled);
    if (!opts.length) continue;
    const next = [];
    for (const combo of combos) {
      for (const opt of opts) {
        next.push({ ...combo, [g.name]: opt.label });
      }
    }
    combos = next.length ? next : combos;
  }
  return combos.slice(0, max).map((properties) => ({
    properties,
    raw: { fromDomGroups: true },
  }));
}

export function buildFromSkuBase(base) {
  const propsRaw = base.props ?? base.skuProps;
  if (!Array.isArray(propsRaw) || propsRaw.length === 0) {
    return { skuGroups: [], skus: [] };
  }

  const propMeta = new Map();
  for (const p of propsRaw) {
    if (!p || typeof p !== 'object') continue;
    const pid = String(p.pid ?? p.propertyId ?? p.propId ?? p.id ?? '').trim();
    const name = String(p.name ?? p.propName ?? p.propertyName ?? '规格').trim() || '规格';
    const valuesRaw = p.values ?? p.value;
    if (!Array.isArray(valuesRaw)) continue;
    const values = new Map();
    for (const v of valuesRaw) {
      if (!v || typeof v !== 'object') continue;
      const vid = String(v.vid ?? v.valueId ?? v.id ?? v.name ?? '').trim();
      const label = String(v.name ?? v.valueName ?? v.text ?? '').trim();
      if (!vid || !label) continue;
      const imgRaw = String(v.image ?? v.img ?? v.pic ?? '').trim();
      values.set(vid, { name: label, image: imgRaw || undefined });
    }
    if (values.size) propMeta.set(pid || name, { name, values });
  }

  const skuGroups = [];
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

  const skusRaw = Array.isArray(base.skus ?? base.skuList ?? base.skuInfos)
    ? base.skus ?? base.skuList ?? base.skuInfos
    : base.skuMap && typeof base.skuMap === 'object'
      ? Object.values(base.skuMap)
      : [];
  const skus = [];
  if (Array.isArray(skusRaw)) {
    for (const s of skusRaw) {
      if (!s || typeof s !== 'object') continue;
      const propPath = String(s.propPath ?? s.propPathStr ?? s.specId ?? '').trim();
      const properties = {};
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
      const priceObj = s.price && typeof s.price === 'object' ? s.price : undefined;
      const subPriceObj = s.subPrice && typeof s.subPrice === 'object' ? s.subPrice : undefined;
      // 新版天猫 SSR：sku2info[skuId].subPrice=券后、price=优惠前（priceText 单位：元）
      const price =
        parsePrice(s.price) ??
        parsePrice(subPriceObj?.priceText) ??
        parsePrice(priceObj?.priceText) ??
        parsePrice(subPriceObj?.priceMoney) ??
        parsePrice(priceObj?.priceMoney);
      const stock = parseQuantity(s.quantity ?? s.stock ?? s.amount);
      if (Object.keys(properties).length === 0 && skuGroups.length === 1 && skuGroups[0].options.length === 1) {
        properties[skuGroups[0].name] = skuGroups[0].options[0].label;
      }
      if (Object.keys(properties).length === 0) continue;
      skus.push({
        properties,
        price,
        stock: stock !== undefined ? Math.floor(stock) : undefined,
        skuCode: String(s.skuId ?? s.skuid ?? s.id ?? '').trim() || undefined,
        image: image ? normalizeImageUrl(image) : undefined,
        raw: { source: 'skuBase', propPath },
      });
    }
  }

  if (!skus.length && skuGroups.length) {
    return { skuGroups, skus: cartesianFromGroups(skuGroups) };
  }
  return { skuGroups, skus };
}

// 新版天猫 SSR：把 skuCore.sku2info（按 skuId 索引的库存/价格信息）合并进
// skuBase.skus（propPath + skuId）。初始加载只有库存，带 skuId 加载才会附带价格。
export function mergeSku2InfoIntoSkus(skus, sku2info) {
  if (!sku2info || typeof sku2info !== 'object') return skus;
  return skus.map((s) => {
    const key = String(s.skuCode ?? s.skuId ?? s.skuid ?? s.id ?? '').trim();
    const info = key ? sku2info[key] : undefined;
    if (!info || typeof info !== 'object') return s;
    const priceObj = info.price && typeof info.price === 'object' ? info.price : undefined;
    const subPriceObj = info.subPrice && typeof info.subPrice === 'object' ? info.subPrice : undefined;
    const price =
      parsePrice(info.price) ??
      parsePrice(subPriceObj?.priceText) ??
      parsePrice(priceObj?.priceText) ??
      parsePrice(subPriceObj?.priceMoney) ??
      parsePrice(priceObj?.priceMoney);
    const stock = parseQuantity(info.quantity ?? info.stock ?? info.amount);
    return {
      ...s,
      price: price && price > 0 ? price : s.price,
      stock: stock !== undefined ? stock : s.stock,
      raw: {
        ...(s.raw ?? {}),
        sku2info: {
          quantity: info.quantity,
          quantityText: info.quantityText ?? '',
          logisticsTime: info.logisticsTime ?? '',
          moreQuantity: info.moreQuantity ?? '',
        },
      },
    };
  });
}

// 把 skuId 价格探测结果（页面内 fetch 各 skuId SSR 页得到）合并进 skus。
// probes 形如 { [skuId]: { priceText, originalPriceText, quantity, quantityText, logisticsTime } }。
export function applySkuPriceProbe(skus, probes) {
  if (!probes || typeof probes !== 'object') return skus;
  const hasAny = Object.keys(probes).some(
    (k) =>
      probes[k] &&
      typeof probes[k] === 'object' &&
      !probes[k].error &&
      (probes[k].priceText || probes[k].originalPriceText || probes[k].quantity != null),
  );
  if (!hasAny) return skus;
  return skus.map((s) => {
    const key = String(s.skuCode ?? s.id ?? s.skuId ?? '').trim();
    const p = key ? probes[key] : undefined;
    if (!p || typeof p !== 'object' || p.error) return s;
    const probePrice = parsePrice(p.priceText) ?? parsePrice(p.originalPriceText);
    const probeStock = parseQuantity(p.quantity);
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

export function jsonPatchFromScan(raw) {
  let skuGroups = [];
  let skus = [];
  for (const base of raw.skuBases ?? []) {
    const built = buildFromSkuBase(base);
    if (built.skus.length > skus.length) {
      skuGroups = built.skuGroups;
      skus = built.skus;
    } else if (built.skuGroups.length > skuGroups.length && !skus.length) {
      skuGroups = built.skuGroups;
    }
  }
  const sku2info = Array.isArray(raw.sku2infos ?? [])
    ? raw.sku2infos.find((v) => v && typeof v === 'object' && Object.keys(v).length > 1) ?? {}
    : {};
  const mainImages = dedupeUrls((raw.imageUrls ?? []).slice(0, 30));
  return {
    mainImages,
    descriptionImages: [],
    attributes: raw.attrPairs ?? {},
    skuGroups,
    skus,
    sku2info,
    debug: {
      jsonRootCount: raw.rootCount,
      skuBaseCount: raw.skuBaseCount,
      jsonSku2InfoCount: Object.keys(sku2info).length,
      jsonMainImageCount: mainImages.length,
      jsonSkuCount: skus.length,
    },
  };
}

// ---------- merge（json-extract.mergeTaobaoPayload） ----------
export function mergeTaobaoPayload(dom, json) {
  const skuGroups = json.skuGroups.length >= dom.skuGroups.length ? json.skuGroups : dom.skuGroups;
  let skus = json.skus.length ? json.skus : dom.skus;
  if (!skus.length && skuGroups.length) {
    skus = cartesianFromGroups(skuGroups);
  }
  const sku2info = json.sku2info && Object.keys(json.sku2info).length ? json.sku2info : {};
  return {
    ...dom,
    mainImages: dedupeUrls([...dom.mainImages, ...json.mainImages]),
    descriptionImages: dedupeUrls([...dom.descriptionImages, ...json.descriptionImages]),
    attributes: { ...dom.attributes, ...json.attributes },
    skuGroups,
    skus,
    sku2info,
    debug: { ...dom.debug, ...json.debug },
  };
}

// ---------- sku-collect（Node 侧部分） ----------
export function toTaobaoSkuGroups(payload) {
  return payload.skuGroups.map((g) => ({
    name: g.name,
    options: g.options.map((o) => ({
      name: o.label,
      available: !o.disabled,
      imageUrl: '',
    })),
  }));
}

export function cartesianCombos(groups, max) {
  let combos = [{ attrs: {}, available: true, imageUrl: '' }];
  for (const g of groups) {
    const opts = g.options.filter((o) => o.name);
    if (!opts.length) continue;
    const next = [];
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

export function mergeSkuResults(base, clicked) {
  if (!clicked.length) return { skus: base };
  const byKey = new Map();
  for (const s of base) byKey.set(JSON.stringify(s.properties ?? {}), s);
  for (const s of clicked) {
    const key = JSON.stringify(s.properties ?? {});
    const prev = byKey.get(key);
    if (!prev || (s.price && s.price > 0 && (!prev.price || prev.price <= 0))) {
      byKey.set(key, { ...prev, ...s, properties: s.properties });
    }
  }
  return { skus: [...byKey.values()] };
}

// ---------- parser: 价格归并 ----------
export function resolvePriceInfo(payload, skus) {
  const base = { price: parsePrice(payload.priceText) };
  const skuPrices = skus.map((s) => s.price).filter((p) => !!p && p > 0);
  let priceMin = base.price;
  let priceMax = base.price;
  let priceSource = 'unknown';

  if (base.price && base.price > 0) priceSource = 'page_display';
  if (skuPrices.length) {
    priceMin = Math.min(...skuPrices);
    priceMax = Math.max(...skuPrices);
    if (!base.price || base.price <= 0) priceSource = 'sku';
  }

  const rangeMatch = (payload.priceRange || payload.priceText || '').match(
    /(\d+(?:\.\d{1,2})?)\s*[-–—~至]\s*(\d+(?:\.\d{1,2})?)/,
  );
  if (rangeMatch) {
    priceMin = Number(rangeMatch[1]);
    priceMax = Number(rangeMatch[2]);
    if (!base.price) priceSource = 'page_display';
  }

  return {
    price: base.price ?? (skuPrices.length === 1 ? skuPrices[0] : undefined),
    priceMin,
    priceMax,
    currency: 'CNY',
    priceText: payload.priceRange || payload.priceText,
    priceSource,
  };
}

// ---------- quality ----------
export function buildQualityReport(assembled) {
  const warnings = [...new Set(assembled.warnings)];
  const errors = [];
  if (!String(assembled.title ?? '').trim()) errors.push('TITLE_NOT_FOUND');
  if (assembled.mainImages.length === 0) errors.push('MAIN_IMAGES_EMPTY');

  let score = 1;
  if (!String(assembled.title ?? '').trim()) score -= 0.35;
  if (assembled.mainImages.length === 0) score -= 0.35;
  else if (assembled.mainImages.length < 2) score -= 0.05;
  if (!assembled.price || assembled.price <= 0) score -= 0.12;
  if (assembled.descriptionImages.length === 0) score -= 0.08;
  else if (assembled.descriptionImages.length < 3) score -= 0.03;
  if (assembled.skus.length === 0) score -= 0.08;
  if (Object.keys(assembled.attributes).length === 0) score -= 0.05;

  const WARNING_CODES = new Set([
    'PRICE_NOT_FOUND',
    'SKU_INCOMPLETE',
    'DETAIL_IMAGES_INCOMPLETE',
    'ATTRIBUTES_EMPTY',
    'STOCK_UNKNOWN',
  ]);
  for (const w of warnings) if (WARNING_CODES.has(w)) score -= 0.04;

  const status = errors.length > 0 ? 'failed' : warnings.length > 0 ? 'warning' : 'ok';
  return {
    status,
    score: Math.round(Math.max(0, Math.min(1, score)) * 100) / 100,
    warnings,
    errors,
  };
}

// ---------- auth-detect 正则与判定 ----------
export const VERIFICATION_RE =
  /验证码|滑块|安全验证|验证中心|访问受限|风险验证|人机验证|请完成验证|拖动.*验证|请按住滑块|punish|x5secdata|captcha|_____tmd_____/i;
export const LOGIN_URL_RE =
  /(?:^|\.)login\.(?:taobao|tmall)\.com|passport\.(?:taobao|tmall)\.com|login\.m\.taobao\.com/i;
const NOT_FOUND_RE =
  /(?:商品|宝贝).{0,12}(?:不存在|已下架|已失效)|找不到(?:该|此)?(?:商品|宝贝)|页面不存在|404\s*(?:not found|页面)/i;
const ACCESS_DENIED_RE = /访问被拒绝|无权访问|拒绝访问|access denied/i;

// signals: evaluateAuthSignals() 的返回
export function resolveAccessStatus(signals) {
  const body = signals.bodySnippet ?? '';
  if (signals.verifyRequiredHit) {
    return { status: 'verify_required', errorCode: 'VERIFY_REQUIRED' };
  }
  if (signals.loginRequiredHit) {
    return { status: 'login_required', errorCode: 'LOGIN_REQUIRED' };
  }
  if (ACCESS_DENIED_RE.test(body)) {
    return { status: 'access_denied', errorCode: 'ACCESS_DENIED' };
  }
  const pageText = `${signals.pageTitle ?? ''}\n${body}`;
  if (!signals.productCoreHit && NOT_FOUND_RE.test(pageText)) {
    return { status: 'not_found', errorCode: 'ITEM_NOT_FOUND' };
  }
  return { status: 'public' };
}
