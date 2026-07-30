/** 管理端统一布局与间距 token（与 global.less 保持一致） */
export const layoutTokens = {
  pagePaddingX: 24,
  pagePaddingY: 20,
  pagePaddingBottom: 32,
  pageMaxWidth: 1680,
  settingsMaxWidth: 1440,
  dashboardMaxWidth: 1680,
  formMaxWidth: 1440,
  sectionGap: 24,
  cardGap: 16,
  cardPadding: 24,
  cardPaddingCompact: 20,
  formColumnGap: 24,
  formRowGap: 20,
  controlHeight: 40,
  borderRadius: 8,
  borderRadiusSm: 6,
  borderRadiusLg: 10,
  pageTitleSize: 20,
  pageDescSize: 14,
  pageTitleDescGap: 6,
  pageHeaderBottomGap: 20,
  breadcrumbTitleGap: 12,
  labelControlGap: 8,
  controlHelpGap: 6,
  fieldGroupGap: 16,
  modalWidthSm: 480,
  modalWidthMd: 640,
  modalWidthLg: 800,
  drawerWidthSm: 560,
  drawerWidthMin: 720,
  drawerWidthMax: 960,
  drawerViewportGap: 48,
} as const;

/** Ant Design theme seed values. Ordinary UI colors should come from Ant Design tokens. */
export const themeTokens = {
  colorPrimary: '#2563eb',
  colorSuccess: '#16a34a',
  colorWarning: '#d97706',
  colorError: '#dc2626',
  colorInfo: '#0891b2',
  colorText: '#0f172a',
  colorTextSecondary: '#475569',
  colorBorderSecondary: '#dbe3ee',
  colorBgLayout: '#f4f6f9',
  colorBgContainer: '#ffffff',
  fontFamily:
    "system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, 'PingFang SC', 'Microsoft YaHei', sans-serif",
} as const;

/** TradeMind-only accents that Ant Design does not describe directly. Keep this list intentionally small. */
export const tmSemanticTokens = {
  aiAccent: '#6d5df6',
  dataAccent: '#0891b2',
  taskTrackMuted: '#cbd5e1',
} as const;

export const elevationTokens = {
  cardShadow: '0 1px 2px rgba(15, 23, 42, 0.04)',
  elevatedShadow: '0 8px 24px rgba(15, 23, 42, 0.08)',
  stickyShadow: '0 -4px 16px rgba(15, 23, 42, 0.06)',
} as const;

export type LayoutTokens = typeof layoutTokens;
export type ThemeTokens = typeof themeTokens;
export type TmSemanticTokens = typeof tmSemanticTokens;
