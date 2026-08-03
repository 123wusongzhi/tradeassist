import { ok } from './envelope';

export function settingsResponse(path: string) {
  if (path !== '/api/v1/settings/integrations/overview') return null;

  return ok({
    ai: { configured: false, provider: 'openai_compatible', model: '' },
    image: { removebg: false, openaiImage: false, comfyui: false },
    storage: { kind: 'local', configured: true },
    mail: { configured: false },
    // The backend overview currently contains only platforms with app settings.
    // The Admin must merge shop-credential platforms from /platform/providers.
    platforms: [
      {
        platform: 'douyin_shop',
        name: '抖店',
        status: 'available',
        groupKey: 'platform_douyin_shop',
        appConfigured: false,
      },
    ],
    collectRulesCount: 0,
    disclaimerShort: 'E2E 集成总览使用模拟数据。',
  });
}
