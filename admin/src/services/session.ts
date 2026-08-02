import {
  AUTH_SESSION_LEGACY,
  AUTH_SESSION_MODE_KEY,
  AUTH_SESSION_SECURE,
  AUTH_TOKEN_KEY,
} from "@/constants/auth";

let accessToken = "";
let refreshFlight: Promise<string> | undefined;
let refreshRequest: (() => Promise<string>) | undefined;
let refreshFailureHandled = false;
let sessionGeneration = 0;

export function configureSessionRefresh(fn: () => Promise<string>) {
  refreshRequest = fn;
}

export function getAccessToken() {
  return accessToken;
}

export function setSession(token: string, sessionMode?: string) {
  sessionGeneration += 1;
  accessToken = token;
  refreshFailureHandled = false;
  // Missing/unknown mode must fail toward the cookie-backed flow. Legacy
  // localStorage persistence is allowed only when the server explicitly asks
  // for it (development compatibility).
  const mode =
    sessionMode === AUTH_SESSION_LEGACY
      ? AUTH_SESSION_LEGACY
      : AUTH_SESSION_SECURE;
  localStorage.setItem(AUTH_SESSION_MODE_KEY, mode);
  if (mode === AUTH_SESSION_SECURE) localStorage.removeItem(AUTH_TOKEN_KEY);
  else localStorage.setItem(AUTH_TOKEN_KEY, token);
}

export function restoreLegacySession() {
  if (localStorage.getItem(AUTH_SESSION_MODE_KEY) !== AUTH_SESSION_LEGACY) {
    localStorage.removeItem(AUTH_TOKEN_KEY);
    return "";
  }
  accessToken = localStorage.getItem(AUTH_TOKEN_KEY) || "";
  return accessToken;
}

export function clearSession() {
  sessionGeneration += 1;
  accessToken = "";
  localStorage.removeItem(AUTH_TOKEN_KEY);
  localStorage.removeItem(AUTH_SESSION_MODE_KEY);
}

export async function refreshAccessToken() {
  if (!refreshRequest) throw new Error("refresh not configured");
  if (!refreshFlight) {
    const generation = sessionGeneration;
    refreshFlight = refreshRequest()
      .then((token) => {
        if (generation !== sessionGeneration)
          throw new Error("session was cleared during refresh");
        setSession(token, AUTH_SESSION_SECURE);
        return token;
      })
      .finally(() => {
        refreshFlight = undefined;
      });
  }
  return refreshFlight;
}

type RequestErrorLike = {
  config?: Record<string, any> & { url?: string; __authRetried?: boolean };
  response?: { status?: number; url?: string };
};

/** Applies the refresh-and-replay policy to an Axios/Umi request rejection. */
export async function recoverRequestError<T>(input: {
  error: RequestErrorLike;
  replay: (url: string, config: Record<string, any>) => Promise<T>;
  onFailure: () => void;
}): Promise<T> {
  const original = input.error?.config;
  const url = original?.url || input.error?.response?.url || "";
  if (
    input.error?.response?.status !== 401 ||
    !original ||
    original.__authRetried ||
    /\/auth\/(?:login|refresh)/.test(url)
  ) {
    throw input.error;
  }
  original.__authRetried = true;
  return recoverUnauthorized({
    url,
    retried: false,
    replay: () => input.replay(original.url || url, original),
    onFailure: input.onFailure,
  });
}

export async function recoverUnauthorized<T>(input: {
  url?: string;
  retried?: boolean;
  replay: () => Promise<T>;
  onFailure: () => void;
}): Promise<T> {
  if (input.retried || /\/auth\/(?:login|refresh)/.test(input.url || ""))
    throw new Error("unauthorized");
  try {
    await refreshAccessToken();
  } catch (error) {
    clearSession();
    if (!refreshFailureHandled) {
      refreshFailureHandled = true;
      input.onFailure();
    }
    throw error;
  }
  return input.replay();
}
