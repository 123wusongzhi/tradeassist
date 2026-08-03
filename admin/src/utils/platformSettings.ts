export type PlatformSettingsMode = 'application' | 'shop_credentials';

type PlatformSettingsEntryMeta = {
  platform: string;
  name: string;
  status: string;
  settingsGroupKey?: string;
};

export type PlatformIntegrationSummary = {
  platform: string;
  name: string;
  status: string;
  groupKey?: string;
  appConfigured: boolean;
  settingsMode: PlatformSettingsMode;
};

const SHOP_LEVEL_CREDENTIAL_PLATFORMS = new Set(['ozon']);

export function platformSettingsMode(platform: string): PlatformSettingsMode {
  return SHOP_LEVEL_CREDENTIAL_PLATFORMS.has(platform.trim().toLowerCase())
    ? 'shop_credentials'
    : 'application';
}

export function shouldShowPlatformSettingsEntry(meta: PlatformSettingsEntryMeta): boolean {
  return (
    platformSettingsMode(meta.platform) === 'shop_credentials' ||
    Boolean(meta.settingsGroupKey?.trim())
  );
}

export function resolvePlatformSettingsTab(
  entries: PlatformSettingsEntryMeta[],
  requestedPlatform?: string,
): string | undefined {
  const visible = entries.filter(shouldShowPlatformSettingsEntry);
  if (!visible.length) return undefined;

  const requested = requestedPlatform?.trim().toLowerCase();
  const matched = requested && visible.find((entry) => entry.platform.toLowerCase() === requested);
  if (matched) return matched.platform;

  return visible.find((entry) => entry.platform === 'douyin_shop')?.platform ?? visible[0].platform;
}

export function platformSettingsHref(platform: string): string {
  return `/settings/platforms?platform=${encodeURIComponent(platform.trim().toLowerCase())}`;
}

export function mergePlatformIntegrationSummaries(
  overview: Omit<PlatformIntegrationSummary, 'settingsMode'>[],
  providers: PlatformSettingsEntryMeta[],
): PlatformIntegrationSummary[] {
  const merged = overview.map((item) => ({
    ...item,
    settingsMode: platformSettingsMode(item.platform),
  }));
  const seen = new Set(merged.map((item) => item.platform));

  for (const provider of providers) {
    if (seen.has(provider.platform) || platformSettingsMode(provider.platform) !== 'shop_credentials') {
      continue;
    }
    merged.push({
      platform: provider.platform,
      name: provider.name,
      status: provider.status,
      groupKey: provider.settingsGroupKey?.trim() || undefined,
      appConfigured: false,
      settingsMode: 'shop_credentials',
    });
    seen.add(provider.platform);
  }

  return merged;
}
