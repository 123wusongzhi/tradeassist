import { fetchCollectEnginesStatus, type CollectEnginesStatus } from './collectTasks';
import { fetchSettingsList } from './settings';
import {
  COLLECT_TAOBO_TMALL_DEFAULT_ENGINE_KEY,
  findCollectEngineStatus,
  normalizeCollectEngine,
  type CollectEngine,
} from '@/utils/collectEngine';
import { pickGroup } from '@/utils/settingsForm';

/**
 * 读取采集设置页保存的淘宝/天猫默认引擎。读取失败时使用后端运行时默认值，
 * 两个来源都不可用时才回退 Playwright。
 */
export async function fetchDefaultCollectEngine(
  knownStatus?: CollectEnginesStatus | null,
): Promise<CollectEngine> {
  let status = knownStatus;
  try {
    const { items } = await fetchSettingsList();
    const group = pickGroup(items, 'collector');
    const configured = (
      group[COLLECT_TAOBO_TMALL_DEFAULT_ENGINE_KEY] ?? ''
    ).trim().toLowerCase();
    if (configured === 'playwright') return 'playwright';
    if (configured === 'opencli') {
      status ??= await fetchCollectEnginesStatus();
      const opencli = findCollectEngineStatus(status, 'opencli');
      return opencli?.enabled && opencli.configured ? 'opencli' : 'playwright';
    }
  } catch {
    // Runtime status below remains authoritative when settings are unavailable.
  }

  try {
    status ??= await fetchCollectEnginesStatus();
    return normalizeCollectEngine(status.defaultEngine);
  } catch {
    return 'playwright';
  }
}
