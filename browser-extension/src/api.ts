import type { CollectTask, ExtensionDevice, NormalizedProduct } from './types.js';
import { normalizeAPIBase } from './pairing.js';

type Envelope<T> = {
  code: number;
  message: string;
  data: T;
};

export class TradeMindAPIError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly code: number,
  ) {
    super(message);
    this.name = 'TradeMindAPIError';
  }
}

export class TradeMindAPI {
  readonly baseURL: string;

  constructor(
    baseURL: string,
    private readonly deviceToken?: string,
  ) {
    this.baseURL = normalizeAPIBase(baseURL);
  }

  private async request<T>(path: string, init: RequestInit = {}): Promise<T> {
    const headers = new Headers(init.headers);
    headers.set('Accept', 'application/json');
    if (init.body) {
      headers.set('Content-Type', 'application/json');
    }
    if (this.deviceToken) {
      headers.set('Authorization', `Bearer ${this.deviceToken}`);
    }
    const response = await fetch(`${this.baseURL}${path}`, { ...init, headers, credentials: 'omit' });
    let envelope: Envelope<T>;
    try {
      envelope = (await response.json()) as Envelope<T>;
    } catch {
      throw new TradeMindAPIError(`TradeMind 返回了无法识别的响应（HTTP ${response.status}）`, response.status, -1);
    }
    if (!response.ok || envelope.code !== 0) {
      throw new TradeMindAPIError(envelope.message || 'TradeMind 请求失败', response.status, envelope.code);
    }
    return envelope.data;
  }

  async exchangePairing(code: string, deviceName: string) {
    return this.request<{ device: ExtensionDevice; deviceToken: string }>(
      '/api/v1/collect/browser-extension/pairings/exchange',
      {
        method: 'POST',
        body: JSON.stringify({ code, deviceName }),
      },
    );
  }

  async session() {
    return this.request<ExtensionDevice>('/api/v1/collect/browser-extension/session');
  }

  async createTask(url: string, source: string) {
    return this.request<CollectTask>('/api/v1/collect/browser-extension/tasks', {
      method: 'POST',
      body: JSON.stringify({ source, url }),
    });
  }

  async submitResult(taskID: string, product: NormalizedProduct) {
    return this.request<CollectTask>(
      `/api/v1/collect/browser-extension/tasks/${encodeURIComponent(taskID)}/result`,
      {
        method: 'POST',
        body: JSON.stringify({ product }),
      },
    );
  }

  async submitFailure(taskID: string, errorCode: string, message: string) {
    return this.request<CollectTask>(
      `/api/v1/collect/browser-extension/tasks/${encodeURIComponent(taskID)}/failure`,
      {
        method: 'POST',
        body: JSON.stringify({ errorCode, message }),
      },
    );
  }
}
