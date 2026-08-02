import { history } from '@umijs/max';
import type { InitialStateModel } from '@/typings/umi-runtime';
import { clearSession } from './session';

/** Clear browser auth state before returning an authenticated user to login. */
export async function finishAuthenticatedLogout(
  setInitialState: InitialStateModel['setInitialState'],
): Promise<void> {
  clearSession();
  await setInitialState((state) => ({ ...state, currentUser: undefined }));
  history.replace('/user/login');
}
