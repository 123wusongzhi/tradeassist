import { history } from '@umijs/max';

export type UrlStateValue = string | number | boolean | null | undefined;
export type UrlState = Record<string, UrlStateValue>;

const ALLOWED_QUERY_KEYS = new Set([
  'page',
  'pageSize',
  'keyword',
  'status',
  'type',
  'taskType',
  'priority',
  'platform',
  'shopId',
  'tab',
  'id',
  'drawer',
  'source',
  'start',
  'end',
  'detailTaskType',
  'failureCategory',
  'severity',
  'recoveryStatus',
  'normalizedStatus',
  'includeResolved',
  'includeMarked',
  'timeRange',
]);

function normalizeValue(value: UrlStateValue): string | undefined {
  if (value === undefined || value === null || value === '') return undefined;
  if (typeof value === 'boolean') return value ? 'true' : undefined;
  return String(value);
}

function sameValue(a: string | null, b: string | undefined) {
  return (a || undefined) === b;
}

export function readQueryState<T extends Record<string, string | undefined>>(
  search: string,
  keys: readonly (keyof T & string)[],
): T {
  const sp = new URLSearchParams(search || '');
  return keys.reduce<Record<string, string | undefined>>((acc, key) => {
    acc[key] = sp.get(key) || undefined;
    return acc;
  }, {}) as T;
}

export function writeQueryState(next: UrlState, options?: { replace?: boolean; pathname?: string }) {
  mergeQueryState(next, { replace: options?.replace, pathname: options?.pathname, resetKeys: [] });
}

export function mergeQueryState(
  next: UrlState,
  options?: { replace?: boolean; pathname?: string; resetKeys?: string[] },
) {
  const pathname = options?.pathname || history.location.pathname;
  const sp = new URLSearchParams(history.location.search || '');

  options?.resetKeys?.forEach((key) => sp.delete(key));

  let changed = false;
  Object.entries(next).forEach(([key, raw]) => {
    if (!ALLOWED_QUERY_KEYS.has(key)) return;
    const value = normalizeValue(raw);
    if (sameValue(sp.get(key), value)) return;
    changed = true;
    if (value === undefined) {
      sp.delete(key);
    } else {
      sp.set(key, value);
    }
  });

  if (!changed) return;
  const qs = sp.toString();
  const url = qs ? `${pathname}?${qs}` : pathname;
  if (options?.replace) {
    history.replace(url);
  } else {
    history.push(url);
  }
}

export function clearQueryState(keys: readonly string[], options?: { replace?: boolean; pathname?: string }) {
  const pathname = options?.pathname || history.location.pathname;
  const sp = new URLSearchParams(history.location.search || '');
  let changed = false;
  keys.forEach((key) => {
    if (sp.has(key)) {
      sp.delete(key);
      changed = true;
    }
  });
  if (!changed) return;
  const qs = sp.toString();
  const url = qs ? `${pathname}?${qs}` : pathname;
  if (options?.replace) {
    history.replace(url);
  } else {
    history.push(url);
  }
}

export function appendSourceToUrl(url: string, source = 'dashboard') {
  const [path, query = ''] = url.split('?');
  const sp = new URLSearchParams(query);
  if (!sp.has('source')) sp.set('source', source);
  const qs = sp.toString();
  return qs ? `${path}?${qs}` : path;
}
