import { TradeMindAPI, TradeMindAPIError } from './api.js';
import { isSupportedTaobaoTmallURL } from './adapters/taobao-tmall.js';
import { normalizeAPIBase, parsePairingInput } from './pairing.js';
import type { CollectTask, ExtensionDevice, PageCollectResult } from './types.js';

const STORAGE_KEYS = ['apiBase', 'deviceToken', 'device'] as const;

type StoredConnection = {
  apiBase: string;
  deviceToken: string;
  device: ExtensionDevice;
};

type ActivePage = {
  tabId: number;
  title: string;
  url: string;
  supported: boolean;
};

const element = <T extends HTMLElement>(id: string): T => {
  const found = document.getElementById(id);
  if (!found) throw new Error(`missing element #${id}`);
  return found as T;
};

const ui = {
  connectionBadge: element<HTMLSpanElement>('connectionBadge'),
  pairingForm: element<HTMLDivElement>('pairingForm'),
  connectedPanel: element<HTMLDivElement>('connectedPanel'),
  apiBase: element<HTMLInputElement>('apiBase'),
  pairingCode: element<HTMLInputElement>('pairingCode'),
  pairButton: element<HTMLButtonElement>('pairButton'),
  disconnectButton: element<HTMLButtonElement>('disconnectButton'),
  deviceName: element<HTMLElement>('deviceName'),
  connectedBase: element<HTMLSpanElement>('connectedBase'),
  pageBadge: element<HTMLSpanElement>('pageBadge'),
  pageTitle: element<HTMLElement>('pageTitle'),
  pageUrl: element<HTMLSpanElement>('pageUrl'),
  collectButton: element<HTMLButtonElement>('collectButton'),
  refreshButton: element<HTMLButtonElement>('refreshButton'),
  progressCard: element<HTMLElement>('progressCard'),
  progressMessage: element<HTMLElement>('progressMessage'),
  stepTask: element<HTMLLIElement>('stepTask'),
  stepRead: element<HTMLLIElement>('stepRead'),
  stepUpload: element<HTMLLIElement>('stepUpload'),
  resultCard: element<HTMLElement>('resultCard'),
  resultSummary: element<HTMLElement>('resultSummary'),
  openProductButton: element<HTMLButtonElement>('openProductButton'),
  toast: element<HTMLElement>('toast'),
};

let connection: StoredConnection | null = null;
let activePage: ActivePage | null = null;
let collecting = false;
let lastTask: CollectTask | null = null;
let toastTimer: number | undefined;

function showToast(message: string, error = false) {
  ui.toast.textContent = message;
  ui.toast.classList.toggle('is-error', error);
  ui.toast.classList.remove('hidden');
  if (toastTimer) window.clearTimeout(toastTimer);
  toastTimer = window.setTimeout(() => ui.toast.classList.add('hidden'), 5000);
}

function connectionAPI(): TradeMindAPI {
  if (!connection) throw new Error('请先连接 TradeMind');
  return new TradeMindAPI(connection.apiBase, connection.deviceToken);
}

function renderConnection() {
  const paired = Boolean(connection);
  ui.pairingForm.classList.toggle('hidden', paired);
  ui.connectedPanel.classList.toggle('hidden', !paired);
  ui.connectionBadge.textContent = paired ? '已连接' : '未连接';
  ui.connectionBadge.className = paired ? 'badge' : 'badge badge--muted';
  if (connection) {
    ui.deviceName.textContent = connection.device.name;
    ui.connectedBase.textContent = connection.apiBase;
    ui.apiBase.value = connection.apiBase;
  }
  renderPage();
}

function renderPage() {
  if (!activePage) {
    ui.pageBadge.textContent = '无法读取';
    ui.pageBadge.className = 'badge badge--warning';
    ui.pageTitle.textContent = '没有可用的当前标签页';
    ui.pageUrl.textContent = '';
  } else {
    ui.pageBadge.textContent = activePage.supported ? '可以采集' : '不支持';
    ui.pageBadge.className = activePage.supported ? 'badge' : 'badge badge--warning';
    ui.pageTitle.textContent = activePage.title || '未命名页面';
    ui.pageUrl.textContent = activePage.url;
  }
  ui.collectButton.disabled = !connection || !activePage?.supported || collecting;
}

function setProgress(stage: 'task' | 'read' | 'upload' | 'done', message: string) {
  ui.progressCard.classList.remove('hidden');
  ui.resultCard.classList.add('hidden');
  const order = ['task', 'read', 'upload'] as const;
  const current = stage === 'done' ? order.length : order.indexOf(stage);
  const items = [ui.stepTask, ui.stepRead, ui.stepUpload];
  items.forEach((item, index) => {
    item.classList.toggle('is-done', index < current || stage === 'done');
    item.classList.toggle('is-active', index === current && stage !== 'done');
  });
  ui.progressMessage.textContent = message;
}

async function readStoredConnection(): Promise<StoredConnection | null> {
  const stored = await chrome.storage.local.get([...STORAGE_KEYS]);
  if (
    typeof stored.apiBase !== 'string' ||
    typeof stored.deviceToken !== 'string' ||
    !stored.device ||
    typeof stored.device !== 'object'
  ) {
    return null;
  }
  return {
    apiBase: stored.apiBase,
    deviceToken: stored.deviceToken,
    device: stored.device as ExtensionDevice,
  };
}

async function saveConnection(next: StoredConnection) {
  await chrome.storage.local.set(next);
  connection = next;
  renderConnection();
}

async function clearConnection() {
  await chrome.storage.local.remove([...STORAGE_KEYS]);
  connection = null;
  lastTask = null;
  ui.resultCard.classList.add('hidden');
  ui.progressCard.classList.add('hidden');
  renderConnection();
}

async function ensureHostPermission(apiBase: string) {
  const originPattern = `${new URL(apiBase).origin}/*`;
  if (await chrome.permissions.contains({ origins: [originPattern] })) return;
  const granted = await chrome.permissions.request({ origins: [originPattern] });
  if (!granted) {
    throw new Error('需要允许扩展访问你的 TradeMind 地址');
  }
}

async function pair() {
  ui.pairButton.disabled = true;
  ui.pairButton.textContent = '正在连接…';
  try {
    const payload = parsePairingInput(ui.pairingCode.value, ui.apiBase.value);
    await ensureHostPermission(payload.apiBase);
    const api = new TradeMindAPI(payload.apiBase);
    const result = await api.exchangePairing(payload.code, `Chrome 侧边栏 · ${navigator.platform || 'Desktop'}`);
    await saveConnection({
      apiBase: payload.apiBase,
      deviceToken: result.deviceToken,
      device: result.device,
    });
    ui.pairingCode.value = '';
    showToast('连接成功，现在可以采集当前商品');
  } catch (error) {
    showToast(error instanceof Error ? error.message : '连接失败', true);
  } finally {
    ui.pairButton.disabled = false;
    ui.pairButton.textContent = '连接';
  }
}

async function refreshActivePage() {
  try {
    const [tab] = await chrome.tabs.query({ active: true, lastFocusedWindow: true });
    if (!tab?.id || !tab.url) throw new Error('active tab unavailable');
    activePage = {
      tabId: tab.id,
      title: tab.title ?? '',
      url: tab.url,
      supported: isSupportedTaobaoTmallURL(tab.url),
    };
  } catch {
    activePage = null;
  }
  renderPage();
}

function browserFailure(error: unknown): { code: string; message: string } {
  if (error instanceof TradeMindAPIError && error.status === 401) {
    return { code: 'DEVICE_UNAUTHORIZED', message: '扩展连接已失效，请重新配对' };
  }
  const raw = error instanceof Error ? error.message : String(error);
  const separator = raw.indexOf(':');
  if (separator > 0) {
    return {
      code: raw.slice(0, separator).trim().toUpperCase(),
      message: raw.slice(separator + 1).trim(),
    };
  }
  return { code: 'BROWSER_EXTENSION_FAILED', message: raw || '采集失败' };
}

async function submitResultWithRetry(
  api: TradeMindAPI,
  taskID: string,
  product: Extract<PageCollectResult, { ok: true }>['product'],
) {
  try {
    return await api.submitResult(taskID, product);
  } catch (error) {
    const retryable =
      !(error instanceof TradeMindAPIError) || error.status === 409 || error.status >= 500;
    if (!retryable) throw error;
    await new Promise<void>((resolve) => window.setTimeout(resolve, 500));
    return api.submitResult(taskID, product);
  }
}

async function collectCurrentPage() {
  if (!connection || !activePage?.supported || collecting) return;
  collecting = true;
  lastTask = null;
  renderPage();
  setProgress('task', '正在创建可追踪的采集任务…');
  const api = connectionAPI();
  let task: CollectTask | null = null;
  try {
    task = await api.createTask(activePage.url);
    setProgress('read', '正在读取当前页面的标题、图片、参数和规格…');
    const result = await chrome.runtime.sendMessage<PageCollectResult>({
      type: 'COLLECT_ACTIVE_TAB',
      tabId: activePage.tabId,
      url: activePage.url,
    });
    if (!result?.ok) {
      const failure = result ?? {
        ok: false as const,
        errorCode: 'EMPTY_RESULT',
        message: '浏览器没有返回采集结果',
      };
      await api.submitFailure(task.id, failure.errorCode, failure.message).catch(() => undefined);
      throw new Error(`${failure.errorCode}: ${failure.message}`);
    }
    setProgress('upload', '页面读取完成，正在由 Backend 校验并创建商品草稿…');
    task = await submitResultWithRetry(api, task.id, result.product);
    lastTask = task;
    setProgress('done', '采集结果已保存。');
    ui.resultCard.classList.remove('hidden');
    ui.resultSummary.textContent = `任务 ${task.id.slice(0, 8)} · 商品草稿 ${task.resultProductId?.slice(0, 8) ?? '已创建'}`;
    showToast('商品采集完成');
  } catch (error) {
    const failure = browserFailure(error);
    if (failure.code === 'DEVICE_UNAUTHORIZED') {
      await clearConnection();
    }
    setProgress('done', `采集失败：${failure.message}`);
    ui.stepTask.classList.remove('is-done');
    ui.stepRead.classList.remove('is-done');
    ui.stepUpload.classList.remove('is-done');
    showToast(failure.message, true);
  } finally {
    collecting = false;
    renderPage();
  }
}

async function validateStoredSession() {
  connection = await readStoredConnection();
  if (!connection) return;
  try {
    const device = await connectionAPI().session();
    connection.device = device;
    await chrome.storage.local.set({ device });
  } catch (error) {
    if (error instanceof TradeMindAPIError && error.status === 401) {
      await clearConnection();
    }
  }
}

ui.pairButton.addEventListener('click', () => void pair());
ui.disconnectButton.addEventListener('click', () => void clearConnection());
ui.collectButton.addEventListener('click', () => void collectCurrentPage());
ui.refreshButton.addEventListener('click', () => void refreshActivePage());
ui.openProductButton.addEventListener('click', () => {
  if (!connection || !lastTask?.resultProductId) return;
  void chrome.tabs.create({
    url: `${connection.apiBase}/product/drafts/${encodeURIComponent(lastTask.resultProductId)}?source=browser-extension`,
  });
});
chrome.tabs.onActivated.addListener(() => void refreshActivePage());
chrome.tabs.onUpdated.addListener((_tabID, changeInfo) => {
  if (changeInfo.status === 'complete' || changeInfo.url) void refreshActivePage();
});

void (async () => {
  await validateStoredSession();
  renderConnection();
  await refreshActivePage();
})();
