import { describe, expect, it } from 'vitest';
import type { CollectEnginesStatus } from '@/services/collectTasks';
import {
  collectEngineOptions,
  collectEngineLabel,
  collectSourceHasEnabledEngine,
  findCollectEngineStatus,
  normalizeCollectEngine,
  readCollectTaskEngine,
  resolveDefaultCollectEngine,
} from '../collectEngine';

function status(enabled: boolean, configured: boolean, playwrightEnabled = false): CollectEnginesStatus {
  return {
    defaultEngine: 'opencli',
    engines: [
      {
        engine: 'opencli',
        enabled,
        configured,
        reachable: false,
        ready: false,
        status: enabled ? 'unavailable' : 'disabled',
        message: '',
        supportedSources: ['taobao_tmall'],
      },
      {
        engine: 'playwright',
        enabled: playwrightEnabled,
        configured: true,
        reachable: false,
        ready: false,
        status: playwrightEnabled ? 'unavailable' : 'disabled',
        message: '',
        supportedSources: ['1688', 'pinduoduo', 'taobao_tmall', 'custom'],
      },
    ],
  };
}

describe('collect engine UI routing', () => {
  it('keeps OpenCLI first but disables it when the bridge is not configured', () => {
    const options = collectEngineOptions(status(false, false));
    expect(options[0]).toMatchObject({ value: 'opencli', disabled: true });
    expect(options[1]).toMatchObject({ value: 'playwright', disabled: true, label: 'Playwright（已停用）' });
  });

  it('keeps a configured but temporarily unreachable bridge selectable', () => {
    const runtime = status(true, true);
    expect(collectEngineOptions(runtime)[0]).toMatchObject({
      value: 'opencli',
      disabled: false,
    });
    expect(findCollectEngineStatus(runtime, 'opencli')?.reachable).toBe(false);
  });

  it('normalizes unknown persisted values to the fail-closed OpenCLI placeholder', () => {
    expect(normalizeCollectEngine('opencli')).toBe('opencli');
    expect(normalizeCollectEngine('unknown')).toBe('opencli');
  });

  it('does not honor a persisted Playwright preference when runtime disables it', () => {
    const runtime = status(true, true);
    expect(resolveDefaultCollectEngine(runtime, 'playwright')).toBe('opencli');
    expect(collectSourceHasEnabledEngine(runtime, 'taobao_tmall')).toBe(true);
    expect(collectSourceHasEnabledEngine(runtime, '1688')).toBe(false);
  });

  it('keeps the explicit Playwright recovery path when runtime enables it', () => {
    const runtime = status(true, true, true);
    expect(resolveDefaultCollectEngine(runtime, 'playwright')).toBe('playwright');
    expect(collectSourceHasEnabledEngine(runtime, '1688')).toBe(true);
  });

  it('supports an explicit OpenCLI fallback for an unset self-hosted default', () => {
    expect(normalizeCollectEngine(undefined, 'opencli')).toBe('opencli');
    expect(normalizeCollectEngine('playwright', 'opencli')).toBe('playwright');
  });

  it('reads only persisted supported engines from task options', () => {
    expect(readCollectTaskEngine({ engine: 'opencli' })).toBe('opencli');
    expect(readCollectTaskEngine({ engine: 'playwright' })).toBe('playwright');
    expect(readCollectTaskEngine({ engine: 'unknown' })).toBeNull();
    expect(readCollectTaskEngine(null)).toBeNull();
    expect(collectEngineLabel('opencli')).toBe('OpenCLI');
  });
});
