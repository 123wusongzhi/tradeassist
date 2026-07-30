import { defineConfig } from '@umijs/max';
import { elevationTokens, layoutTokens, themeTokens } from './src/constants/layoutTokens';
import routes from './config/routes';

export default defineConfig({
  title: '贸灵 TradeMind',
  npmClient: 'npm',
  antd: {
    appConfig: {},
    configProvider: {
      theme: {
        cssVar: true,
        token: {
          colorPrimary: themeTokens.colorPrimary,
          colorSuccess: themeTokens.colorSuccess,
          colorWarning: themeTokens.colorWarning,
          colorError: themeTokens.colorError,
          colorInfo: themeTokens.colorInfo,
          colorText: themeTokens.colorText,
          colorTextSecondary: themeTokens.colorTextSecondary,
          colorBorderSecondary: themeTokens.colorBorderSecondary,
          colorBgLayout: themeTokens.colorBgLayout,
          colorBgContainer: themeTokens.colorBgContainer,
          borderRadius: layoutTokens.borderRadius,
          borderRadiusLG: layoutTokens.borderRadiusLg,
          controlHeight: layoutTokens.controlHeight,
          boxShadowTertiary: elevationTokens.cardShadow,
          fontFamily: themeTokens.fontFamily,
        },
        components: {
          Layout: {
            bodyBg: themeTokens.colorBgLayout,
            headerBg: themeTokens.colorBgContainer,
            footerBg: themeTokens.colorBgLayout,
            siderBg: themeTokens.colorBgContainer,
          },
          Menu: {
            itemBorderRadius: layoutTokens.borderRadius,
            itemHeight: 40,
            itemMarginBlock: 4,
            itemMarginInline: 8,
            iconSize: 16,
            collapsedIconSize: 16,
          },
          Card: {
            headerFontSize: layoutTokens.pageDescSize + 1,
            borderRadiusLG: layoutTokens.borderRadius,
          },
          Button: {
            borderRadius: layoutTokens.borderRadius,
          },
          Table: {
            headerBg: '#f8fafc',
            cellFontSize: 14,
          },
        },
      },
    },
  },
  access: {},
  model: {},
  initialState: {},
  request: {},
  layout: {
    /** 侧栏/顶栏品牌仅在 `app.tsx` 的 `logo` 中渲染，此处不设 title，避免与 logo 内文案重复 */
    title: false,
    locale: false,
    layout: 'mix',
    navTheme: 'light',
    fixedHeader: true,
    fixSiderbar: true,
    contentWidth: 'Fluid',
  },
  routes,
  devtool: process.env.NODE_ENV === 'production' ? false : 'source-map',
  proxy: {
    '/api': {
      target: 'http://127.0.0.1:8080',
      changeOrigin: true,
    },
    '/static': {
      target: 'http://127.0.0.1:8080',
      changeOrigin: true,
    },
  },
});
