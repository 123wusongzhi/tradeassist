import { describe, expect, it } from 'vitest';
import { mapCollectErrorMessage, mapCollectorErrorCodeDetail, mapCollectorErrorCodeLabel } from '../collectErrors';

describe('collect error copy', () => {
  it('turns the legacy OpenCLI connection-refused error into an actionable message', () => {
    const raw =
      'collector request: Post "http://host.docker.internal:3100/v1/collect": dial tcp 192.168.65.254:3100: connect: connection refused';

    const message = mapCollectErrorMessage(raw, 'taobao_tmall');

    expect(message).toContain('OpenCLI Bridge');
    expect(message).toContain('Playwright');
    expect(message).not.toContain('host.docker.internal');
  });

  it('keeps a generic collector outage free of internal network details', () => {
    const message = mapCollectErrorMessage('dial tcp 172.20.0.8:3001: connect: connection refused', '1688');

    expect(message).toBe('采集服务当前无法连接。请检查对应采集引擎是否已启动，确认状态正常后再重试。');
  });

  it('does not present an OpenCLI empty result as proof that the item was removed', () => {
    const raw = `ok: false
error:
  code: EMPTY_RESULT
  message: tmall product returned no data
  help: 商品不存在或已下架`;

    expect(mapCollectErrorMessage(raw, 'taobao_tmall', 'opencli')).toContain('不能仅凭本次结果确认');
    expect(mapCollectorErrorCodeLabel('ITEM_NOT_FOUND', 'opencli')).toBe('OpenCLI 未获取到商品内容');
    expect(mapCollectorErrorCodeDetail('ITEM_NOT_FOUND', 'taobao_tmall', 'opencli')).toContain('登录态');
  });
});
