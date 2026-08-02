import type {
  CSSProperties,
  KeyboardEvent,
  ReactElement,
  ReactNode,
} from "react";
import { LogoutOutlined } from "@ant-design/icons";
import { Avatar, Dropdown, Space, Tooltip } from "antd";
import type { MenuDataItem } from "@umijs/route-utils";
import { history, request as umiRequest } from "@umijs/max";
import type { RequestConfig, RunTimeLayoutConfig } from "@/typings/umi-runtime";
import AppMessageBridge from "@/components/AppMessageBridge";
import BrandLogo from "@/components/BrandLogo";
import { refreshSession } from "@/services/auth";
import {
  clearSession,
  configureSessionRefresh,
  getAccessToken,
  recoverRequestError,
  refreshAccessToken,
  restoreLegacySession,
} from "@/services/session";
import { themeTokens, tmSemanticTokens } from "@/constants/layoutTokens";
import { postJSON } from "@/services/request";
import { filterMenuByPermission } from "@/utils/menuAccess";
import { useInitialStateModel } from "@/hooks/useInitialStateModel";
import type { InitialState, InitialStateModel } from "@/typings/umi-runtime";

/** ProLayout 侧栏菜单头部 / 头像区回调的常用 props */
type SiderMenuLayoutProps = {
  collapsed?: boolean;
};

async function loadProfileFromToken(
  token: string,
): Promise<API.CurrentUser | undefined> {
  const res = await fetch("/api/v1/auth/profile", {
    headers: { Authorization: `Bearer ${token}` },
  });
  const json = (await res.json()) as { code: number; data?: API.CurrentUser };
  if (!res.ok || json.code !== 0 || !json.data) return undefined;
  return json.data;
}

configureSessionRefresh(async () => {
  const refreshed = await refreshSession();
  return refreshed.token;
});

/**
 * Runs inside umi antd innerProvider `<App>` (under ConfigProvider).
 * Do not add another `<App>` in rootContainer — that wraps outside ConfigProvider and breaks static message.
 */
export function innerProvider(container: ReactElement) {
  return (
    <>
      <AppMessageBridge />
      {container}
    </>
  );
}

export async function getInitialState(): Promise<{
  currentUser?: API.CurrentUser;
}> {
  let token = getAccessToken() || restoreLegacySession();
  if (!token) {
    try {
      token = await refreshAccessToken();
    } catch {
      return {};
    }
  }
  if (!token) {
    return {};
  }
  const user = await loadProfileFromToken(token);
  if (!user) {
    clearSession();
    return {};
  }
  return { currentUser: user };
}

export const request: RequestConfig = {
  requestInterceptors: [
    (url, options) => {
      const token = getAccessToken() || restoreLegacySession();
      const headers: Record<string, string> = {
        ...((options.headers as Record<string, string>) || {}),
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      };
      return { url, options: { ...options, headers } };
    },
  ],

  responseInterceptors: [
    [
      (response: any) => response,
      (error: any) =>
        recoverRequestError({
          error,
          // Preserve the caller's Umi return-shape contract: helpers expect
          // business data by default, while explicit getResponse stays raw.
          replay: (url, config) => umiRequest(url, { ...config }),
          onFailure: () => window.location.assign("/user/login"),
        }),
    ],
  ],
  errorConfig: {
    errorHandler: (error: any) => {
      if (error?.info?.skipErrorHandler) {
        throw error;
      }
      // API helpers own the single-flight refresh and one-time redirect flow.
      // Throwing preserves the 401 so they can retry the original request once.
      throw error;
    },
  },
};

const TM_AVATAR_GRADIENT_BG = `linear-gradient(135deg, ${themeTokens.colorPrimary} 0%, ${tmSemanticTokens.dataAccent} 100%)`;

const TM_AVATAR_STYLE: CSSProperties = { background: TM_AVATAR_GRADIENT_BG };

/** 侧栏 / 顶栏品牌图形（与登录页同一 `logo.png`） */
const TM_BRAND_MARK = <BrandLogo height={28} />;

async function logoutAndClear(
  setInitialState: InitialStateModel["setInitialState"],
) {
  try {
    await postJSON("/api/v1/auth/logout");
  } catch {
    /* ignore */
  }
  clearSession();
  setInitialState((s) => ({ ...s, currentUser: undefined }));
  history.push("/user/login");
}

function looksLikeEmail(value: string) {
  return value.includes("@");
}

/** 侧栏/顶栏展示：邮箱账号优先显示 @ 前昵称，完整邮箱放副行 */
function resolveUserLabels(user?: API.CurrentUser) {
  const displayName = user?.displayName?.trim() || "管理员";
  const email = user?.email?.trim() || "";
  const username = user?.username?.trim() || "";
  const loginId = email || username;

  if (looksLikeEmail(displayName) && loginId && displayName === loginId) {
    const local = displayName.split("@")[0]?.trim() || displayName;
    return {
      primary: local,
      secondary: displayName,
      initial: local.slice(0, 1).toUpperCase(),
    };
  }

  return {
    primary: displayName,
    secondary: loginId && loginId !== displayName ? loginId : "",
    initial: displayName.slice(0, 1).toUpperCase(),
  };
}

function buildLogoutMenu(
  setInitialState: InitialStateModel["setInitialState"],
) {
  return {
    items: [
      {
        key: "logout",
        icon: <LogoutOutlined />,
        label: "退出登录",
        onClick: () => void logoutAndClear(setInitialState),
      },
    ],
  };
}

/** 侧栏底部账号：整行可点，向上弹出菜单；邮箱账号双行展示避免截断 */
function SiderUserFooter({ collapsed }: { collapsed?: boolean }) {
  const { setInitialState, initialState } = useInitialStateModel();
  const { primary, secondary, initial } = resolveUserLabels(
    initialState?.currentUser,
  );
  const menu = buildLogoutMenu(setInitialState);
  const tooltipTitle = secondary ? `${primary}\n${secondary}` : primary;

  const avatar = (
    <Avatar size={32} style={TM_AVATAR_STYLE}>
      {initial}
    </Avatar>
  );

  const body = (
    <div className="tm-sider-user">
      {avatar}
      <div className="tm-sider-user__meta">
        <span className="tm-sider-user__name" title={primary}>
          {primary}
        </span>
        {secondary ? (
          <span className="tm-sider-user__sub" title={secondary}>
            {secondary}
          </span>
        ) : (
          <span className="tm-sider-user__sub">管理员</span>
        )}
      </div>
    </div>
  );

  return (
    <Dropdown
      menu={menu}
      trigger={["click"]}
      placement={collapsed ? "topRight" : "topLeft"}
      overlayStyle={{ minWidth: 140 }}
    >
      <div
        className="tm-sider-user-trigger"
        role="button"
        tabIndex={0}
        aria-label={`当前用户 ${primary}`}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            (e.currentTarget as HTMLDivElement).click();
          }
        }}
      >
        {collapsed ? (
          <Tooltip title={tooltipTitle} placement="right">
            <span className="tm-sider-user tm-sider-user--collapsed">
              {avatar}
            </span>
          </Tooltip>
        ) : (
          body
        )}
      </div>
    </Dropdown>
  );
}

function RightActions() {
  const { setInitialState, initialState } = useInitialStateModel();
  const { primary, initial } = resolveUserLabels(initialState?.currentUser);

  return (
    <Dropdown
      menu={buildLogoutMenu(setInitialState)}
      placement="bottomRight"
      trigger={["click"]}
    >
      <Space
        size={10}
        className="tm-header-user"
        style={{ cursor: "pointer", paddingInline: 4 }}
      >
        <Avatar size={32} style={{ ...TM_AVATAR_STYLE, fontSize: 14 }}>
          {initial}
        </Avatar>
        <span className="tm-layout-user-name" title={primary}>
          {primary}
        </span>
      </Space>
    </Dropdown>
  );
}

export const layout: RunTimeLayoutConfig = ({ initialState }) => ({
  title: false,
  logo: TM_BRAND_MARK,
  /** ProLayout 在侧栏会把 avatar 区域与（未定义 actionsRender 时的）rightContentRender 各渲染一遍，导致两行相同账号 */
  actionsRender: () => [],
  menuHeaderRender: (
    logoDom: ReactNode,
    _titleDom: ReactNode,
    props?: SiderMenuLayoutProps,
  ) => {
    const collapsed = props?.collapsed;
    const goHome = () => history.push("/dashboard");
    const interactive = {
      role: "button" as const,
      tabIndex: 0,
      onClick: goHome,
      onKeyDown: (e: KeyboardEvent) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          goHome();
        }
      },
    };

    if (collapsed) {
      return (
        <div
          {...interactive}
          style={{
            display: "flex",
            justifyContent: "center",
            alignItems: "center",
            padding: "14px 0 10px",
            cursor: "pointer",
            width: "100%",
          }}
        >
          {logoDom}
        </div>
      );
    }

    return (
      <div
        {...interactive}
        style={{
          display: "flex",
          alignItems: "center",
          gap: 10,
          padding: "14px 16px 10px",
          cursor: "pointer",
          width: "100%",
          minWidth: 0,
        }}
      >
        {logoDom}
        <span
          style={{
            fontWeight: 600,
            fontSize: 16,
            letterSpacing: "-0.02em",
            color: themeTokens.colorText,
            whiteSpace: "nowrap",
            overflow: "hidden",
            textOverflow: "ellipsis",
          }}
        >
          贸灵{" "}
          <span
            style={{ fontWeight: 500, color: themeTokens.colorTextSecondary }}
          >
            TradeMind
          </span>
        </span>
      </div>
    );
  },
  avatarProps: initialState?.currentUser
    ? {
        render: (
          _avatarProps: Record<string, unknown>,
          _defaultDom: ReactNode,
          menuProps?: SiderMenuLayoutProps,
        ) => <SiderUserFooter collapsed={menuProps?.collapsed} />,
      }
    : false,
  token: {
    headerHeight: 56,
    colorBgLayout: themeTokens.colorBgLayout,
    colorTextMenuSelected: themeTokens.colorPrimary,
    colorBgMenuItemSelected: "rgba(37, 99, 235, 0.09)",
    siderWidth: 224,
  },
  menu: { locale: false },
  menuDataRender: (menuData: MenuDataItem[]) =>
    filterMenuByPermission(
      menuData,
      initialState?.currentUser?.role,
      initialState?.currentUser?.permissions,
    ),
  onPageChange: () => {
    const { pathname } = history.location;
    if (pathname === "/user/login" || pathname.startsWith("/user/login"))
      return;
    // 必须用 token 判断：initialState 在此闭包里不会在登录后刷新，会一直当作未登录并反复 push 登录页，触发 Navigate 死循环。
    if (!getAccessToken()) {
      history.replace(`/user/login?redirect=${encodeURIComponent(pathname)}`);
    }
  },
  rightContentRender: () => <RightActions key="nav-right" />,
});
