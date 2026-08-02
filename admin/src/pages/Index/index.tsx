import { history } from '@umijs/max';
import { useLayoutEffect } from 'react';
import { getAccessToken } from '@/services/session';

/**
 * 根路径不要用 route.redirect（Umi 的 Navigate）与 layout.onPageChange 同时改 URL，会打成死循环。
 */
export default function IndexPage() {
  useLayoutEffect(() => {
    if (getAccessToken()) {
      history.replace('/dashboard');
    } else {
      history.replace('/user/login?redirect=/');
    }
  }, []);
  return null;
}
