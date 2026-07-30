import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import { createCollectTask, fetchCollectEnginesStatus } from '../collectTasks';

const requestMock = vi.mocked(request);

describe('collect engine API service', () => {
  it('reads the authenticated engine status endpoint', async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: 'ok',
      data: { defaultEngine: 'opencli', engines: [] },
    });

    await fetchCollectEnginesStatus();

    expect(requestMock).toHaveBeenCalledWith('/api/v1/collect/engines/status', {
      method: 'GET',
    });
  });

  it('preserves an explicit engine on task creation', async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: 'ok',
      data: { id: 'task-1' },
    });
    const body = {
      source: 'taobao_tmall',
      url: 'https://item.taobao.com/item.htm?id=1',
      engine: 'opencli' as const,
    };

    await createCollectTask(body);

    expect(requestMock).toHaveBeenCalledWith('/api/v1/collect/tasks', {
      method: 'POST',
      data: body,
    });
  });
});
