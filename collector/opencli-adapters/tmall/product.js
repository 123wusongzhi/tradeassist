// tmall/product — TradeMind 天猫采集逻辑移植到 opencli adapter
// 提取层: page.evaluate 字符串（移植自 collector page-extract.ts / json-extract.ts / sku-collect.ts）
// 解析层: ./shared.js 纯函数（移植自 parser.ts / quality.ts 等）
// @trademind-managed-opencli-adapter v1
import { cli, Strategy } from '@jackwener/opencli/registry';
import { ArgumentError, AuthRequiredError, CommandExecutionError, EmptyResultError } from '@jackwener/opencli/errors';
import {
  cleanTaobaoTitle,
  extractTitleFromDocumentTitle,
  dedupeUrls,
  jsonPatchFromScan,
  mergeTaobaoPayload,
  toTaobaoSkuGroups,
  cartesianCombos,
  mergeSkuResults,
  resolvePriceInfo,
  buildQualityReport,
  resolveAccessStatus,
  parsePrice,
} from './shared.js';

// ---------- 页面内 JS：登录/验证信号检测（移植 auth-detect.ts） ----------
const AUTH_SIGNALS_JS = `(() => {
  const href = location.href || '';
  const title = document.title || '';
  const body = (document.body && document.body.innerText || '').slice(0, 8000);
  const lowerHref = href.toLowerCase();
  const verifyRe = /验证码|滑块|安全验证|验证中心|访问受限|风险验证|人机验证|请完成验证|拖动.*验证|请按住滑块|punish|x5secdata|captcha|_____tmd_____/i;
  const loginUrlRe = /(?:^|\\.)login\\.(?:taobao|tmall)\\.com|passport\\.(?:taobao|tmall)\\.com|login\\.m\\.taobao\\.com/i;
  const loggedInRes = [/我的淘宝/, /已登录/, /会员名/, /nick\\s*[:：]/i, /Hi,\\s*\\S+/];
  let loggedInHit = false;
  for (const re of loggedInRes) { if (re.test(body)) { loggedInHit = true; break; } }
  if (!loggedInHit) {
    loggedInHit = !!document.querySelector('[class*="member"], [class*="Member"], [class*="user-nick"], [class*="UserNick"], a[href*="member.taobao"], a[href*="i.taobao.com/my"]');
  }
  const verifyRequiredHit = verifyRe.test(body) || /punish|x5secdata|captcha|_____tmd_____|sec\\.taobao\\.com|sec\\.tmall\\.com/i.test(lowerHref);
  const onLoginHost = loginUrlRe.test(lowerHref);
  const explicitLogin = /请登录|立即登录|账号登录|登录后查看|登录淘宝|登录天猫/.test(body) && !loggedInHit && body.length < 4000;
  const loginRequiredHit = !loggedInHit && (onLoginHost || explicitLogin);
  const productCoreHit = !!document.querySelector(
    '[class*="ItemHeader"], [class*="MainTitle"], #J_Title, [class*="PicGallery"], #J_UlThumb'
  );
  return {
    loggedInHit, loginRequiredHit, verifyRequiredHit, productCoreHit,
    pageTitle: title, pageHref: href,
    bodySnippet: body.slice(0, 500),
  };
})()`;

// ---------- 页面内 JS：等待商品核心元素（等价 waitForProductCore） ----------
const WAIT_CORE_JS = `new Promise((resolve) => {
  const sels = ['[class*="ItemHeader"]', '[class*="MainTitle"]', '#J_Title', 'h1', '[class*="PicGallery"]', '#J_UlThumb'];
  const t0 = Date.now();
  const check = () => {
    for (const s of sels) { if (document.querySelector(s)) return resolve({ ok: true, sel: s }); }
    if (Date.now() - t0 > 15000) return resolve({ ok: false });
    setTimeout(check, 300);
  };
  check();
})`;

// ---------- 页面内 JS：DOM 大提取（移植 page-extract.extractTaobaoPagePayload） ----------
const EXTRACT_DOM_JS = `(() => {
  const pickText = (el) => ((el && el.textContent) || '').replace(/\\s+/g, ' ').trim();

  const titleCandidates = [];
  for (const sel of ['[class*="MainTitle"]', '[class*="ItemHeader"] h1', '#J_Title', 'h1', 'meta[property="og:title"]']) {
    const el = document.querySelector(sel);
    if (sel.includes('meta')) {
      const c = el && el.content && el.content.trim();
      if (c) titleCandidates.push(c);
    } else {
      const t = pickText(el);
      if (t) titleCandidates.push(t);
    }
  }
  let title = titleCandidates.find((t) => t.length > 2 && !/淘宝|天猫|登录/.test(t)) || titleCandidates[0] || '';
  if (!title) {
    const docTitle = (document.title || '').replace(/\\s+/g, ' ').trim();
    if (docTitle && !/^(淘宝网|天猫|登录|首页)$/i.test(docTitle)) title = docTitle;
  }
  const originalTitle = titleCandidates[0] || title;

  const priceTexts = [];
  for (const sel of ['[class*="Price--priceText"]', '[class*="priceText"]', '.tm-price', '#J_StrPrice', '[class*="Price"]']) {
    const t = pickText(document.querySelector(sel));
    if (t && /\\d/.test(t)) priceTexts.push(t);
  }
  const priceText = priceTexts[0] || '';
  let priceRange = '';
  if (priceTexts.length > 1) priceRange = priceTexts.slice(0, 3).join(' - ');

  let shopName = '';
  // 天猫新版店铺卡片（detailWrap），文本形如 "ENMG恩爵专卖店5.088VIP好评率..."，截到店名结尾
  const detailWrapA = document.querySelector('a[class*="detailWrap"]');
  if (detailWrapA) {
    const raw = pickText(detailWrapA);
    const m = raw.match(/^(.{2,40}?(?:旗舰店|专卖店|专营店|官方店|自营店|店))/);
    if (m) shopName = m[1];
  }
  if (!shopName) {
    for (const sel of ['[class*="ShopHeader"] [class*="shopName"]', '[class*="shopName"]', '.tb-shop-name a', 'a[href*="shop"]']) {
      const t = pickText(document.querySelector(sel));
      if (t && t.length <= 80 && !/进店|客服|退出|登录|注册|开店|反馈|直播/.test(t)) { shopName = t; break; }
    }
  }

  const imgSet = new Set();
  const pushImg = (raw) => {
    if (!raw) return;
    let u = String(raw).trim();
    if (u.startsWith('//')) u = 'https:' + u;
    if (!u.startsWith('http')) return;
    imgSet.add(u);
  };
  for (const img of document.querySelectorAll('[class*="PicGallery"] img, #J_UlThumb img, [class*="thumbnail"] img, [class*="mainPic"] img')) {
    pushImg(img.src || img.getAttribute('data-src'));
  }
  for (const li of document.querySelectorAll('#J_UlThumb li, [class*="thumbnailItem"]')) {
    const bg = (li.style && li.style.backgroundImage) || '';
    const m = bg.match(/url\\(["']?(.*?)["']?\\)/);
    if (m && m[1]) pushImg(m[1]);
  }
  const mainImages = [...imgSet];

  const detailSet = new Set();
  const detailRoot =
    document.querySelector('#J_Description, #description, [class*="desc-root"], [class*="DetailDesc"]') ||
    document.querySelector('[id*="desc"], [class*="Detail"]');
  if (detailRoot) {
    for (const img of detailRoot.querySelectorAll('img')) {
      const src = img.src || img.getAttribute('data-src');
      if (src) detailSet.add(src.startsWith('//') ? 'https:' + src : src);
    }
  }
  const descriptionImages = [...detailSet];

  const attributes = {};
  // 天猫新版重点参数区：Item 内 Title=值, SubTitle=键
  for (const item of document.querySelectorAll('[class*="emphasisParamsInfoItem"]')) {
    const vEl = item.querySelector('[class*="ItemTitle"]:not([class*="SubTitle"])');
    const kEl = item.querySelector('[class*="ItemSubTitle"]');
    const v = pickText(vEl);
    const k = pickText(kEl);
    if (k && v && k.length <= 40 && v.length <= 120) attributes[k] = v;
  }
  for (const row of document.querySelectorAll('[class*="ItemParams"] li, [class*="attributes"] li, #attributes li, .tm-tableAttr tr')) {
    const t = pickText(row);
    const parts = t.split(/[:：]/);
    if (parts.length >= 2) {
      const k = (parts[0] || '').trim();
      const v = parts.slice(1).join(':').trim();
      if (k && v && k.length <= 40) attributes[k] = v;
    }
  }

  const skuGroups = [];
  for (const group of document.querySelectorAll('[class*="SkuContent"] [class*="skuItem"], #J_isku [class*="prop"], .tm-sale-prop')) {
    const nameEl = group.querySelector('[class*="label"], dt, .tm-prop-title');
    const name = pickText(nameEl) || '规格';
    const options = [];
    for (const opt of group.querySelectorAll('li, [class*="valueItem"], .tm-img-prop span')) {
      const label = pickText(opt);
      if (!label || label.length > 80) continue;
      const cls = (opt.className || '').toString();
      options.push({
        label,
        selected: /selected|checked|current/i.test(cls) || opt.getAttribute('aria-checked') === 'true',
        disabled: /disabled|soldout|无货|缺货/i.test(cls + pickText(opt)),
      });
    }
    if (options.length) skuGroups.push({ name, options });
  }

  return {
    title, originalTitle, priceText, priceRange, shopName,
    mainImages, descriptionImages, attributes, skuGroups,
    debug: {
      titleCandidates, priceTexts,
      mainImageCount: mainImages.length,
      detailImageCount: descriptionImages.length,
      skuGroupCount: skuGroups.length,
      pageUrl: location.href,
      documentTitle: document.title || '',
    },
  };
})()`;

// ---------- 页面内 JS：JSON 扫描（移植 json-extract 的 evaluate 体） ----------
const EXTRACT_JSON_JS = `(() => {
  const imageUrls = [];
  const attrPairs = {};
  const skuBases = [];

  const walkForSkuBase = (node, depth) => {
    if (depth > 14 || !node || typeof node !== 'object') return;
    if (Array.isArray(node)) { for (const item of node.slice(0, 40)) walkForSkuBase(item, depth + 1); return; }
    const o = node;
    const props = o.props || o.skuProps;
    const skus = o.skus || o.skuList || o.skuInfos;
    if (Array.isArray(props) && (Array.isArray(skus) || o.skuMap)) skuBases.push(o);
    if (o.skuBase && typeof o.skuBase === 'object') skuBases.push(o.skuBase);
    if (o.skuCore && typeof o.skuCore === 'object') {
      const core = o.skuCore;
      if (core.sku2info && core.props) {
        skuBases.push({ props: core.props, skus: Object.keys(core.sku2info).map((id) => ({ skuId: id, ...core.sku2info[id] })) });
      }
    }
    for (const v of Object.values(o).slice(0, 50)) walkForSkuBase(v, depth + 1);
  };

  const walkForImageUrls = (node, depth) => {
    if (depth > 12 || node == null) return;
    if (typeof node === 'string') {
      const s = node.trim();
      if (/alicdn\\.com|tbcdn\\.cn|taobaocdn\\.com/i.test(s) && /\\.(?:jpg|jpeg|png|webp)/i.test(s)) imageUrls.push(s);
      return;
    }
    if (Array.isArray(node)) { for (const item of node.slice(0, 80)) walkForImageUrls(item, depth + 1); return; }
    if (typeof node !== 'object') return;
    for (const [k, v] of Object.entries(node).slice(0, 60)) {
      if (/desc|detail|content|gallery|image|pic|photo|thumb/i.test(k)) walkForImageUrls(v, depth + 1);
    }
  };

  const roots = [];
  for (const key of ['__INITIAL_STATE__', '__ICE_APP_CONTEXT__', '__INIT_DATA', 'g_config', 'Hub', 'TShop', 'detailData', 'pageData', 'loaderData']) {
    try { const v = window[key]; if (v && typeof v === 'object') roots.push(v); } catch (e) {}
  }
  for (const script of Array.from(document.scripts)) {
    const text = script.textContent || '';
    if (text.length < 80 || text.length > 800000) continue;
    if (!/skuBase|skuCore|skuProps|propPath|picGallery|itemImages|auctionImages/i.test(text)) continue;
    const trimmed = text.trim();
    if (trimmed.startsWith('{') || trimmed.startsWith('[')) {
      try { roots.push(JSON.parse(trimmed)); continue; } catch (e) {}
    }
    const m = text.match(/\\{[\\s\\S]{120,}\\}/);
    if (m) { try { roots.push(JSON.parse(m[0])); } catch (e) {} }
  }

  for (const root of roots) { walkForSkuBase(root, 0); walkForImageUrls(root, 0); }

  for (const root of roots) {
    if (!root || typeof root !== 'object') continue;
    const item = root.item || root.itemDO;
    if (item && typeof item === 'object') {
      for (const key of ['images', 'itemImages', 'picList', 'mainPicList', 'auctionImages']) {
        const arr = item[key];
        if (Array.isArray(arr)) for (const u of arr) imageUrls.push(String(u));
      }
      const propsList = item.propsName || item.properties;
      if (Array.isArray(propsList)) {
        for (const row of propsList) {
          if (!row || typeof row !== 'object') continue;
          const k = String(row.name || row.key || '').trim();
          const v = String(row.value || row.val || '').trim();
          if (k && v) attrPairs[k] = v;
        }
      }
    }
  }

  return { imageUrls, attrPairs, skuBases, rootCount: roots.length, skuBaseCount: skuBases.length };
})()`;

// ---------- 页面内 JS：滚动 + 详情图收集（移植 scrollAndCollectDetailImages） ----------
const SCROLL_DETAIL_JS = `(async () => {
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
  const detailSel = '#J_Description, #description, [class*="desc-root"], [class*="DetailDesc"]';
  const root = document.querySelector(detailSel);
  if (!root) return { images: [] };
  root.scrollIntoView({ block: 'start' });
  await sleep(500);
  for (let i = 0; i < 6; i++) {
    window.scrollBy(0, Math.max(window.innerHeight * 0.8, 400));
    await sleep(400);
  }
  const out = [];
  for (const img of root.querySelectorAll('img')) {
    const src = img.src || img.getAttribute('data-src') || img.getAttribute('data-ks-lazyload');
    if (src) out.push(src.startsWith('//') ? 'https:' + src : src);
  }
  window.scrollTo(0, 0);
  return { images: out };
})()`;

// ---------- 页面内 JS：SKU 逐组合点击采价（移植 collectSkuPricesByClick） ----------
const SKU_CLICK_JS = `(async (combosJson) => {
  const combos = JSON.parse(combosJson);
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
  const pickText = (el) => ((el && el.textContent) || '').replace(/\\s+/g, ' ').trim();
  const priceSel = '[class*="Price--priceText"], [class*="priceText"], .tm-price, #J_StrPrice, [class*="Price"]';
  const results = [];

  const findOptionEl = (groupName, optName) => {
    for (const group of document.querySelectorAll('[class*="SkuContent"] [class*="skuItem"], #J_isku [class*="prop"], .tm-sale-prop')) {
      const gText = pickText(group.querySelector('[class*="label"], dt, .tm-prop-title') || group);
      if (!gText || !group.textContent.includes(groupName)) continue;
      for (const opt of group.querySelectorAll('li, [class*="valueItem"], .tm-img-prop span')) {
        const label = pickText(opt);
        if (label === optName || (label && optName && label.includes(optName))) return opt;
      }
    }
    return null;
  };

  for (const combo of combos) {
    try {
      for (const [groupName, optName] of Object.entries(combo.attrs)) {
        const el = findOptionEl(groupName, optName);
        if (el) {
          const cls = (el.className || '').toString();
          if (!/selected|checked|current/i.test(cls)) {
            el.click();
            await sleep(300);
          }
        }
      }
      await sleep(200);
      const priceEl = document.querySelector(priceSel);
      const priceText = priceEl ? pickText(priceEl) : '';
      results.push({ properties: combo.attrs, priceText, available: combo.available });
    } catch (e) {
      results.push({ properties: combo.attrs, priceText: '', available: combo.available, clickFailed: true });
    }
  }
  return { results };
})`;

cli({
  site: 'tmall',
  name: 'product',
  access: 'read',
  description: '天猫/淘宝商品详情采集（TradeMind 解析逻辑移植）：标题、价格、主图、详情图、属性、SKU',
  domain: 'detail.tmall.com',
  strategy: Strategy.COOKIE,
  navigateBefore: false,
  example: 'opencli tmall product "https://detail.tmall.com/item.htm?id=xxx" -f json',
  args: [
    {
      name: 'url',
      type: 'string',
      required: true,
      positional: true,
      help: '天猫/淘宝商品详情 URL',
    },
    {
      name: 'sku-click',
      type: 'int',
      default: 24,
      help: 'SKU 点击采价最大组合数（0 关闭）',
    },
  ],
  columns: ['title', 'priceText', 'shopName', 'mainImageCount', 'detailImageCount', 'skuCount', 'qualityStatus'],
  func: async (page, args) => {
    const url = String(args.url ?? '').trim();
    if (
      !/detail\.tmall\.com|item\.taobao\.com|chaoshi\.tmall\.com|detail\.tmall\.hk|ju\.taobao\.com|world\.taobao\.com/i.test(
        url,
      )
    ) {
      throw new ArgumentError('仅支持淘宝/天猫商品详情 URL（detail.tmall.com / item.taobao.com 等）');
    }
    const skuClickMax = Math.max(0, Math.min(48, Number(args['sku-click'] ?? 24) || 0));

    // 1. 导航 + 初始等待
    await page.goto(url, { settleMs: 3500 });
    await page.wait(2);

    // 2. 登录/验证/下架检测
    const signals = await page.evaluate(AUTH_SIGNALS_JS);
    const access = resolveAccessStatus(signals);
    if (access.status === 'verify_required') {
      throw new AuthRequiredError('detail.tmall.com', '页面出现安全验证/滑块，请在该 Chrome 手动完成验证后重试');
    }
    if (access.status === 'login_required') {
      throw new AuthRequiredError('detail.tmall.com', '页面需要登录，请在该 Chrome 登录淘宝后重试');
    }
    if (access.status === 'not_found') {
      throw new EmptyResultError('tmall product', '商品不存在或已下架');
    }

    // 3. 等商品核心元素
    await page.evaluate(WAIT_CORE_JS);

    // 4. 轻滚动触发懒加载
    await page.evaluate(
      `(async () => { const s=(ms)=>new Promise(r=>setTimeout(r,ms)); window.scrollBy(0,200); await s(400); window.scrollTo(0,0); return true; })()`,
    );

    // 5. DOM 提取
    const domPayload = await page.evaluate(EXTRACT_DOM_JS);
    if (domPayload && typeof domPayload === 'object') domPayload.skus = [];

    // 6. JSON 扫描 + Node 侧构建 patch
    const rawScan = await page.evaluate(EXTRACT_JSON_JS).catch(() => null);
    const jsonPatch = rawScan
      ? jsonPatchFromScan(rawScan)
      : {
          mainImages: [],
          descriptionImages: [],
          attributes: {},
          skuGroups: [],
          skus: [],
        };

    // 7. 合并 payload
    const payload = mergeTaobaoPayload(domPayload, jsonPatch);

    // 8. SKU 点击采价（有规格组时）
    let skus = payload.skus;
    const skuGroups = toTaobaoSkuGroups(payload);
    if (skuClickMax > 0 && skuGroups.length > 0) {
      const combos = cartesianCombos(skuGroups, Math.min(skuClickMax, 48));
      if (combos.length && combos.length <= skuClickMax) {
        const clicked = await page
          .evaluate(
            `${SKU_CLICK_JS}(${JSON.stringify(JSON.stringify(combos.map((c) => ({ attrs: c.attrs, available: c.available }))))})`,
          )
          .catch(() => null);
        if (clicked && Array.isArray(clicked.results)) {
          const clickedSkus = clicked.results.map((r) => ({
            properties: r.properties,
            price: parsePrice(r.priceText),
            stock: undefined,
            raw: {
              fromSkuClick: true,
              available: r.available,
              priceText: (r.priceText || '').trim(),
            },
          }));
          skus = mergeSkuResults(skus, clickedSkus).skus;
        }
      }
    }

    // 9. 详情图滚动收集
    const detailRes = await page.evaluate(SCROLL_DETAIL_JS).catch(() => null);
    const descriptionImages = dedupeUrls([
      ...(payload.descriptionImages || []),
      ...((detailRes && detailRes.images) || []),
    ]);

    // 10. 价格归并 + 组装
    const priceInfo = resolvePriceInfo(payload, skus);
    const title =
      cleanTaobaoTitle(String(payload.title || '').trim()) ||
      extractTitleFromDocumentTitle(String(payload.debug?.documentTitle ?? ''));

    const warnings = [];
    if (!priceInfo.price || priceInfo.price <= 0) warnings.push('PRICE_NOT_FOUND');
    if (descriptionImages.length === 0) warnings.push('DETAIL_IMAGES_INCOMPLETE');
    if (skuGroups.length === 0 || skus.length === 0) warnings.push('SKU_INCOMPLETE');
    if (Object.keys(payload.attributes).length === 0) warnings.push('ATTRIBUTES_EMPTY');
    if (skus.length > 0 && skus.some((s) => s.stock == null)) warnings.push('STOCK_UNKNOWN');

    const skusFinal = skus.map((s) => ({
      ...s,
      price: s.price && s.price > 0 ? s.price : priceInfo.price && priceInfo.price > 0 ? priceInfo.price : s.price,
    }));

    const assembled = {
      title,
      price: priceInfo.price,
      priceMin: priceInfo.priceMin,
      priceMax: priceInfo.priceMax,
      priceText: priceInfo.priceText,
      currency: 'CNY',
      shopName: payload.shopName,
      mainImages: payload.mainImages,
      descriptionImages,
      attributes: payload.attributes,
      skuGroups,
      skus: skusFinal,
      warnings: [...new Set(warnings)],
    };
    const quality = buildQualityReport(assembled);

    if (!assembled.title.trim()) {
      throw new EmptyResultError('tmall product', '未能提取到商品标题（TITLE_NOT_FOUND）');
    }
    if (assembled.mainImages.length === 0) {
      throw new EmptyResultError('tmall product', '未能提取到商品主图（MAIN_IMAGES_EMPTY）');
    }

    return [
      {
        title: assembled.title,
        priceText: assembled.priceText || String(assembled.price ?? ''),
        price: assembled.price,
        priceMin: assembled.priceMin,
        priceMax: assembled.priceMax,
        shopName: assembled.shopName,
        currency: 'CNY',
        mainImageCount: assembled.mainImages.length,
        detailImageCount: assembled.descriptionImages.length,
        skuCount: assembled.skus.length,
        qualityStatus: quality.status,
        qualityScore: quality.score,
        warnings: assembled.warnings,
        mainImages: assembled.mainImages,
        descriptionImages: assembled.descriptionImages,
        attributes: assembled.attributes,
        skuGroups: assembled.skuGroups,
        skus: assembled.skus,
        sourceUrl: url,
        debug: payload.debug,
      },
    ];
  },
});
