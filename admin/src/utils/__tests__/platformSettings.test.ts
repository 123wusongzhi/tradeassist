import { describe, expect, it } from 'vitest';
import {
  mergePlatformIntegrationSummaries,
  platformSettingsHref,
  resolvePlatformSettingsTab,
  shouldShowPlatformSettingsEntry,
} from '../platformSettings';

const providers = [
  {
    platform: 'douyin_shop',
    name: '抖店',
    status: 'available',
    settingsGroupKey: 'platform_douyin_shop',
  },
  {
    platform: 'ozon',
    name: 'Ozon',
    status: 'beta',
    settingsGroupKey: '',
  },
  {
    platform: 'manual',
    name: '手工店铺',
    status: 'available',
    settingsGroupKey: '',
  },
];

describe('platform settings metadata', () => {
  it('keeps Ozon visible without treating every empty settings group as configurable', () => {
    expect(shouldShowPlatformSettingsEntry(providers[1])).toBe(true);
    expect(shouldShowPlatformSettingsEntry(providers[2])).toBe(false);
  });

  it('resolves Ozon deep links and safely falls back from unsupported platforms', () => {
    expect(resolvePlatformSettingsTab(providers, 'ozon')).toBe('ozon');
    expect(resolvePlatformSettingsTab(providers, 'missing-platform')).toBe('douyin_shop');
    expect(resolvePlatformSettingsTab([], 'ozon')).toBeUndefined();
  });

  it('adds Ozon to the integration overview with shop-credential semantics', () => {
    const merged = mergePlatformIntegrationSummaries(
      [
        {
          platform: 'douyin_shop',
          name: '抖店',
          status: 'available',
          groupKey: 'platform_douyin_shop',
          appConfigured: true,
        },
      ],
      providers,
    );

    expect(merged).toEqual([
      expect.objectContaining({
        platform: 'douyin_shop',
        appConfigured: true,
        settingsMode: 'application',
      }),
      expect.objectContaining({
        platform: 'ozon',
        appConfigured: false,
        settingsMode: 'shop_credentials',
      }),
    ]);
  });

  it('builds a canonical platform settings deep link', () => {
    expect(platformSettingsHref('OZON')).toBe('/settings/platforms?platform=ozon');
  });
});
