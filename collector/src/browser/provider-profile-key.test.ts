import { describe, expect, it } from 'vitest';
import { assertProviderProfileKey, isLegacyProviderProfileKey } from './provider-profile-key.js';

describe('provider profile keys', () => {
  it('allows only legacy tenant 0 or backend-shaped tenant keys', () => {
    expect(assertProviderProfileKey('pinduoduo', 'pinduoduo')).toBe('pinduoduo');
    expect(assertProviderProfileKey('pinduoduo', 'tenant_42_pinduoduo')).toBe('tenant_42_pinduoduo');
    expect(isLegacyProviderProfileKey('pinduoduo', 'pinduoduo')).toBe(true);
    expect(() => assertProviderProfileKey('pinduoduo', 'tenant_0_pinduoduo')).toThrow('INVALID_PROVIDER_PROFILE_KEY');
    expect(() => assertProviderProfileKey('pinduoduo', 'tenant_42_taobao_tmall')).toThrow('INVALID_PROVIDER_PROFILE_KEY');
    expect(() => assertProviderProfileKey('pinduoduo', 'pinduoduo-other')).toThrow('INVALID_PROVIDER_PROFILE_KEY');
  });
});
