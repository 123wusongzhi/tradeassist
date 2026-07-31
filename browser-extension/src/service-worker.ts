import { adapterForURL } from './adapters/registry.js';
import type { PageCollectResult } from './types.js';

type CollectMessage = {
  type: 'COLLECT_ACTIVE_TAB';
  tabId: number;
  url: string;
};

void chrome.sidePanel.setPanelBehavior({ openPanelOnActionClick: true }).catch(() => undefined);

function errorResult(error: unknown): PageCollectResult {
  const raw = error instanceof Error ? error.message : String(error);
  const separator = raw.indexOf(':');
  if (separator > 0) {
    return {
      ok: false,
      errorCode: raw.slice(0, separator).trim().toUpperCase(),
      message: raw.slice(separator + 1).trim(),
    };
  }
  return {
    ok: false,
    errorCode: 'BROWSER_EXTENSION_FAILED',
    message: raw || '浏览器页面采集失败',
  };
}

async function collect(message: CollectMessage): Promise<PageCollectResult> {
  if (!Number.isInteger(message.tabId) || message.tabId <= 0) {
    return { ok: false, errorCode: 'TAB_UNAVAILABLE', message: '无法读取当前标签页' };
  }
  const adapter = adapterForURL(message.url);
  if (!adapter) {
    return {
      ok: false,
      errorCode: 'UNSUPPORTED_PAGE',
      message: '当前仅支持淘宝或天猫商品详情页',
    };
  }
  try {
    const injected = await chrome.scripting.executeScript({
      target: { tabId: message.tabId },
      func: adapter.collect,
    });
    const product = injected[0]?.result;
    if (!product) {
      return { ok: false, errorCode: 'EMPTY_RESULT', message: '页面没有返回可保存的商品信息' };
    }
    return { ok: true, product };
  } catch (error) {
    return errorResult(error);
  }
}

chrome.runtime.onMessage.addListener((raw, _sender, sendResponse) => {
  const message = raw as Partial<CollectMessage>;
  if (message.type !== 'COLLECT_ACTIVE_TAB') return;
  void collect(message as CollectMessage).then(sendResponse);
  return true;
});
