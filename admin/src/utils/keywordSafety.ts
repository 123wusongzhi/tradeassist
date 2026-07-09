export const KEYWORD_MAX_LENGTH = 80;

export const KEYWORD_SENSITIVE_HINT =
  '搜索词会保存在浏览器地址栏和历史记录中，请避免输入手机号、邮箱、密钥等敏感信息。';

export const KEYWORD_TOO_LONG_MESSAGE = '搜索关键词过长，请缩短后再搜索。';

const PHONE_RE = /^1[3-9]\d{9}$/;
const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const ID_CARD_RE = /^\d{17}[\dXx]$/;

const SENSITIVE_WORDS = [
  'access token',
  'accesstoken',
  'refresh token',
  'refreshtoken',
  'app secret',
  'appsecret',
  'secret',
];

export function looksLikeSensitiveKeyword(value?: string): boolean {
  if (!value?.trim()) return false;
  const compact = value.trim().replace(/\s+/g, '');
  const lower = value.trim().toLowerCase();
  if (PHONE_RE.test(compact)) return true;
  if (EMAIL_RE.test(lower)) return true;
  if (ID_CARD_RE.test(compact)) return true;
  return SENSITIVE_WORDS.some((w) => lower.includes(w));
}

export function normalizeSearchKeyword(raw: unknown): { value?: string; truncated: boolean } {
  const s = typeof raw === 'string' ? raw.trim() : '';
  if (!s) return { value: undefined, truncated: false };
  if (s.length <= KEYWORD_MAX_LENGTH) return { value: s, truncated: false };
  return { value: s.slice(0, KEYWORD_MAX_LENGTH), truncated: true };
}
