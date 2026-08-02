const PROVIDERS = new Set(['1688', 'pinduoduo', 'taobao_tmall']);

/** Validates keys minted by the backend from trusted tenant context. */
export function assertProviderProfileKey(provider: string, raw: string): string {
  const key = raw.trim();
  if (!PROVIDERS.has(provider)) throw new Error('INVALID_PROVIDER_PROFILE_KEY:provider');
  // tenant 0 retains the pre-multitenant data directory for a deliberate migration path.
  if (key === provider) return key;
  if (!new RegExp(`^tenant_[1-9][0-9]*_${provider}$`).test(key)) {
    throw new Error('INVALID_PROVIDER_PROFILE_KEY');
  }
  return key;
}

export function isLegacyProviderProfileKey(provider: string, key: string): boolean {
  return key.trim() === provider;
}
