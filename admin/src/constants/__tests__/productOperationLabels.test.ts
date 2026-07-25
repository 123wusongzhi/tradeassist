import { describe, expect, it } from 'vitest';
import { localizeCollectWarningCode, localizePublishCheckItem, readinessGroupLabel, readinessStatusLabel } from '../productOperationLabels';

describe('product operation labels', () => {
  it('normalizes readiness status labels with safe fallback', () => {
    expect(readinessStatusLabel(' READY ')).toBe('已准备好');
    expect(readinessStatusLabel('custom_status')).toBe('custom_status');
    expect(readinessStatusLabel(null)).toBe('—');
  });

  it('keeps unknown group casing when no mapping exists', () => {
    expect(readinessGroupLabel('SKU')).toBe('商品规格');
    expect(readinessGroupLabel('customGroup')).toBe('customGroup');
  });

  it('localizes publish check codes while preserving backend-provided user text', () => {
    expect(localizePublishCheckItem({ code: 'PRICE_MISSING', level: 'ERROR' })).toEqual({
      title: '销售价格未设置',
      message: '请为商品或规格填写有效销售价。',
      severity: 'error',
    });

    expect(localizePublishCheckItem({ code: 'PRICE_MISSING', title: '平台价格异常', message: '请重新检查', level: 'warning' })).toEqual({
      title: '平台价格异常',
      message: '请重新检查',
      severity: 'warning',
    });
  });

  it('maps machine collect warning codes to actionable user copy', () => {
    expect(localizeCollectWarningCode('TITLE_EMPTY')).toContain('商品标题待完善');
    expect(localizeCollectWarningCode('UNKNOWN_MACHINE_CODE')).toBe('采集提示需检查：请核对商品内容后再发布。');
  });
});
