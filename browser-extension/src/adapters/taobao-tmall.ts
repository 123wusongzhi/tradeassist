import type { BrowserCollectAdapter } from './types.js';
import type { NormalizedProduct } from '../types.js';

const SUPPORTED_HOSTS = new Set([
  'detail.tmall.com',
  'chaoshi.tmall.com',
  'detail.tmall.hk',
  'item.taobao.com',
  'ju.taobao.com',
  'world.taobao.com',
]);

export function isSupportedTaobaoTmallURL(raw: string): boolean {
  try {
    const url = new URL(raw);
    return url.protocol === 'https:' && SUPPORTED_HOSTS.has(url.hostname.toLowerCase());
  } catch {
    return false;
  }
}

// This function is serialized by chrome.scripting.executeScript. Keep every
// page helper inside the function body; it must not close over extension state.
export async function collectTaobaoTmallPage(): Promise<NormalizedProduct> {
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
    const match = String(raw ?? '')
      .replace(/,/g, '')
      .match(/(\d+(?:\.\d{1,2})?)/);
    if (!match) return undefined;
    const value = Number(match[1]);
    return Number.isFinite(value) && value > 0 ? value : undefined;
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

  type SkuOption = { label: string; element: HTMLElement; selected: boolean; disabled: boolean };
  type SkuGroup = { name: string; options: SkuOption[] };
  const skuGroups: SkuGroup[] = [];
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
    if (options.length) skuGroups.push({ name, options });
  }

  type Combination = { properties: Record<string, string>; options: SkuOption[] };
  let combinations: Combination[] = [{ properties: {}, options: [] }];
  for (const group of skuGroups) {
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

  const skus: NormalizedProduct['skus'] = [];
  const originalSelections = skuGroups.map((group) => group.options.find((option) => option.selected));
  if (skuGroups.length && combinations.length) {
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
  window.scrollTo({ top: originalScrollY, behavior: 'instant' });

  const warnings: string[] = [];
  if (!productPrice) warnings.push('PRICE_NOT_FOUND');
  if (!descriptionImages.length) warnings.push('DETAIL_IMAGES_INCOMPLETE');
  if (!skuGroups.length || !skus.length) warnings.push('SKU_INCOMPLETE');
  if (!Object.keys(attributes).length) warnings.push('ATTRIBUTES_EMPTY');

  return {
    source: 'taobao_tmall',
    sourceUrl: pageURL,
    title,
    currency: 'CNY',
    mainDescription,
    mainImages,
    descriptionImages,
    attributes,
    skus,
    raw: {
      provider: 'browser_extension',
      schemaVersion: 1,
      capturedAt: new Date().toISOString(),
      pageTitle: documentTitle,
      finalUrl: location.href,
      productPrice,
      priceText,
      qualityWarnings: warnings,
      skuGroups: skuGroups.map((group) => ({
        name: group.name,
        options: group.options.map((option) => ({
          label: option.label,
          disabled: option.disabled,
        })),
      })),
    },
  };
}

export const taobaoTmallAdapter: BrowserCollectAdapter = {
  id: 'taobao_tmall',
  label: '淘宝 / 天猫',
  supports: isSupportedTaobaoTmallURL,
  collect: collectTaobaoTmallPage,
};
