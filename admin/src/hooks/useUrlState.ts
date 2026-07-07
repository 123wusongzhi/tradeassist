import { useCallback, useMemo } from 'react';
import { useLocation } from '@umijs/max';
import {
  clearQueryState,
  mergeQueryState,
  readQueryState,
  type UrlState,
  type UrlStateValue,
} from '@/utils/urlState';

export function useUrlQueryState<T extends Record<string, string | undefined>>(
  keys: readonly (keyof T & string)[],
) {
  const location = useLocation();
  const state = useMemo(() => readQueryState<T>(location.search || '', keys), [keys, location.search]);

  const setState = useCallback(
    (next: UrlState, options?: { replace?: boolean; resetKeys?: string[] }) => {
      mergeQueryState(next, {
        pathname: location.pathname,
        replace: options?.replace,
        resetKeys: options?.resetKeys,
      });
    },
    [location.pathname],
  );

  const clearState = useCallback(
    (clearKeys: readonly string[], options?: { replace?: boolean }) => {
      clearQueryState(clearKeys, { pathname: location.pathname, replace: options?.replace });
    },
    [location.pathname],
  );

  return { state, setState, clearState, search: location.search };
}

export function useUrlDrawerState(drawerName: string) {
  const { state, setState } = useUrlQueryState<{ drawer?: string; id?: string }>(['drawer', 'id']);
  const open = state.drawer === drawerName && !!state.id;
  const id = open ? state.id : undefined;

  const openDrawer = useCallback(
    (nextId: UrlStateValue) => {
      setState({ drawer: drawerName, id: nextId });
    },
    [drawerName, setState],
  );

  const closeDrawer = useCallback(() => {
    setState({ drawer: undefined, id: undefined }, { replace: true });
  }, [setState]);

  return { open, id, openDrawer, closeDrawer };
}
