/** 采集引擎：playwright（容器/服务内浏览器）| opencli（宿主机本机已登录浏览器） */
export type CollectEngine = 'playwright' | 'opencli';

export type CollectEngineStatusItem = {
  engine: CollectEngine;
  enabled: boolean;
  configured: boolean;
  reachable: boolean;
  ready: boolean;
  status: 'ready' | 'degraded' | 'unavailable' | 'disabled';
  message: string;
  supportedSources: string[];
};

export type CollectEnginesStatus = {
  defaultEngine: CollectEngine;
  engines: CollectEngineStatusItem[];
};

/** 淘宝/天猫默认引擎在 settings「collector」分组中的 itemKey，与采集设置页保持一致。 */
export const COLLECT_TAOBO_TMALL_DEFAULT_ENGINE_KEY = 'collect_taobao_tmall_default_engine';

export const COLLECT_ENGINE_SEGMENTED_OPTIONS: {
  label: string;
  value: CollectEngine;
}[] = [
  { label: 'OpenCLI（主引擎）', value: 'opencli' },
  { label: 'Playwright（备用）', value: 'playwright' },
];

/** opencli 引擎目前仅淘宝/天猫可用，其他采集服务不展示引擎选择。 */
export const OPENCLI_SUPPORTED_SOURCE = 'taobao_tmall';

export function normalizeCollectEngine(raw?: string | null, fallback: CollectEngine = 'playwright'): CollectEngine {
  const value = (raw ?? '').trim().toLowerCase();
  if (value === 'opencli' || value === 'playwright') return value;
  return fallback;
}

/** 查找后端返回的单个引擎运行状态。 */
export function findCollectEngineStatus(status: CollectEnginesStatus | null | undefined, engine: CollectEngine) {
  return status?.engines.find((item) => item.engine === engine);
}

export function collectEngineOptions(status?: CollectEnginesStatus | null) {
  const opencli = findCollectEngineStatus(status, 'opencli');
  return COLLECT_ENGINE_SEGMENTED_OPTIONS.map((option) =>
    option.value === 'opencli'
      ? {
          ...option,
          disabled: Boolean(opencli && (!opencli.enabled || !opencli.configured)),
        }
      : option,
  );
}

export function collectEngineLabel(engine: CollectEngine): string {
  return engine === 'opencli' ? 'OpenCLI' : 'Playwright';
}

/** 从任务已持久化的 requestOptions 中安全读取执行引擎；历史任务可能没有该字段。 */
export function readCollectTaskEngine(requestOptions: unknown): CollectEngine | null {
  if (!requestOptions || typeof requestOptions !== 'object' || Array.isArray(requestOptions)) {
    return null;
  }
  const raw = (requestOptions as Record<string, unknown>).engine;
  return raw === 'opencli' || raw === 'playwright' ? raw : null;
}
