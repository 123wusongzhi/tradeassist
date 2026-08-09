import { fetchCollectEnginesStatus, type CollectEnginesStatus } from './collectTasks';
import { fetchSettingsList } from './settings';
import {
  COLLECT_TAOBO_TMALL_DEFAULT_ENGINE_KEY,
  resolveDefaultCollectEngine,
  type CollectEngine,
} from '@/utils/collectEngine';
import { pickGroup } from '@/utils/settingsForm';

/**
 * 读取采集设置页保存的淘宝/天猫默认引擎。读取失败时使用后端运行时默认值，
 * 没有可用后台引擎时返回不可提交的 OpenCLI 占位值；绝不回退到已停用的 Playwright。
 */
export async function fetchDefaultCollectEngine(
  knownStatus?: CollectEnginesStatus | null,
): Promise<CollectEngine> {
  let status = knownStatus;
  let configured = '';
  try {
    const { items } = await fetchSettingsList();
    const group = pickGroup(items, 'collector');
    configured = (
      group[COLLECT_TAOBO_TMALL_DEFAULT_ENGINE_KEY] ?? ''
    ).trim().toLowerCase();
  } catch {
    // Runtime status below remains authoritative when settings are unavailable.
  }

  try {
    status ??= await fetchCollectEnginesStatus();
    return resolveDefaultCollectEngine(status, configured);
  } catch {
    return 'opencli';
  }
}
