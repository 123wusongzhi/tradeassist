import { history } from '@umijs/max';
import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/services/session', () => ({ clearSession: vi.fn() }));

import { finishAuthenticatedLogout } from '@/services/authenticatedLogout';
import { clearSession } from '@/services/session';

describe('finishAuthenticatedLogout', () => {
  beforeEach(() => {
    vi.mocked(clearSession).mockReset();
  });

  it('clears the browser session and initial user before replacing the login route', async () => {
    const setInitialState = vi.fn().mockResolvedValue(undefined);

    await finishAuthenticatedLogout(setInitialState);

    expect(clearSession).toHaveBeenCalledOnce();
    expect(setInitialState).toHaveBeenCalledOnce();
    const update = setInitialState.mock.calls[0]?.[0];
    expect(typeof update).toBe('function');
    expect(update?.({ currentUser: { username: 'admin' }, keep: 'value' })).toEqual({
      currentUser: undefined,
      keep: 'value',
    });
    expect(history.replace).toHaveBeenCalledWith('/user/login');
  });
});
