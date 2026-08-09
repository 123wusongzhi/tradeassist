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
  { label: 'Playwright（已停用）', value: 'playwright' },
];

/** opencli 引擎目前仅淘宝/天猫可用，其他采集服务不展示引擎选择。 */
export const OPENCLI_SUPPORTED_SOURCE = 'taobao_tmall';

export function normalizeCollectEngine(raw?: string | null, fallback: CollectEngine = 'opencli'): CollectEngine {
  const value = (raw ?? '').trim().toLowerCase();
  if (value === 'opencli' || value === 'playwright') return value;
  return fallback;
}

/** 查找后端返回的单个引擎运行状态。 */
export function findCollectEngineStatus(status: CollectEnginesStatus | null | undefined, engine: CollectEngine) {
  return status?.engines.find((item) => item.engine === engine);
}

/** 只有后端明确声明已启用且配置完整时，引擎才允许提交任务。 */
export function collectEngineSelectable(
  status: CollectEnginesStatus | null | undefined,
  engine: CollectEngine,
): boolean {
  const item = findCollectEngineStatus(status, engine);
  return Boolean(item?.enabled && item.configured);
}

/** 判断某来源是否至少有一个已启用、已配置且声明支持它的后台引擎。 */
export function collectSourceHasEnabledEngine(
  status: CollectEnginesStatus | null | undefined,
  source?: string | null,
): boolean {
  const normalized = (source ?? '').trim().toLowerCase();
  if (!normalized || !status) return false;
  return status.engines.some(
    (item) => item.enabled && item.configured && item.supportedSources.includes(normalized),
  );
}

/** 持久化设置只能作为偏好；后端运行时状态决定最终默认值。 */
export function resolveDefaultCollectEngine(
  status: CollectEnginesStatus | null | undefined,
  preferred?: string | null,
): CollectEngine {
  const candidates: CollectEngine[] = [];
  const add = (raw?: string | null) => {
    const normalized = (raw ?? '').trim().toLowerCase();
    if ((normalized === 'opencli' || normalized === 'playwright') && !candidates.includes(normalized)) {
      candidates.push(normalized);
    }
  };
  add(preferred);
  add(status?.defaultEngine);
  add('opencli');
  add('playwright');
  const selected = candidates.find((engine) => {
    const item = findCollectEngineStatus(status, engine);
    return collectEngineSelectable(status, engine) && item?.supportedSources.includes(OPENCLI_SUPPORTED_SOURCE);
  });
  return selected ?? 'opencli';
}

export function collectEngineOptions(status?: CollectEnginesStatus | null) {
  return COLLECT_ENGINE_SEGMENTED_OPTIONS.map((option) => ({
    ...option,
    disabled: !collectEngineSelectable(status, option.value),
  }));
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
