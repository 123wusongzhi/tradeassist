import { describe, expect, it } from 'vitest';
import type { CollectEnginesStatus } from '@/services/collectTasks';
import {
  collectEngineOptions,
  collectEngineLabel,
  findCollectEngineStatus,
  normalizeCollectEngine,
  readCollectTaskEngine,
} from '../collectEngine';

function status(enabled: boolean, configured: boolean): CollectEnginesStatus {
  return {
    defaultEngine: enabled ? 'opencli' : 'playwright',
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
    ],
  };
}

describe('collect engine UI routing', () => {
  it('keeps OpenCLI first but disables it when the bridge is not configured', () => {
    const options = collectEngineOptions(status(false, false));
    expect(options[0]).toMatchObject({ value: 'opencli', disabled: true });
    expect(options[1]).toMatchObject({ value: 'playwright' });
  });

  it('keeps a configured but temporarily unreachable bridge selectable', () => {
    const runtime = status(true, true);
    expect(collectEngineOptions(runtime)[0]).toMatchObject({
      value: 'opencli',
      disabled: false,
    });
    expect(findCollectEngineStatus(runtime, 'opencli')?.reachable).toBe(false);
  });

  it('normalizes unknown persisted values to Playwright', () => {
    expect(normalizeCollectEngine('opencli')).toBe('opencli');
    expect(normalizeCollectEngine('unknown')).toBe('playwright');
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
