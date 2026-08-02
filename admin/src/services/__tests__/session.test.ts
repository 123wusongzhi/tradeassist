import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  clearSession,
  configureSessionRefresh,
  getAccessToken,
  recoverRequestError,
  recoverUnauthorized,
  restoreLegacySession,
  setSession,
} from "@/services/session";
import { normalizeRole, permissionsForRole } from "@/utils/permission";

describe("secure session", () => {
  beforeEach(() => {
    localStorage.clear();
    clearSession();
  });
  it("does not persist a secure access token", () => {
    setSession("access-token", "secure_session");
    expect(getAccessToken()).toBe("access-token");
    expect(localStorage.getItem("trademind_admin_token")).toBeNull();
    expect(sessionStorage.getItem("trademind_admin_token")).toBeNull();
  });
  it("fails safely to secure mode when the server omits sessionMode", () => {
    setSession("access-token");
    expect(localStorage.getItem("trademind_admin_token")).toBeNull();
    expect(localStorage.getItem("trademind_auth_session_mode")).toBe(
      "secure_session",
    );
    clearSession();
    localStorage.setItem("trademind_admin_token", "stale-token");
    expect(restoreLegacySession()).toBe("");
    expect(localStorage.getItem("trademind_admin_token")).toBeNull();
  });
  it("replays with the caller's default or explicit getResponse shape", async () => {
    configureSessionRefresh(vi.fn().mockResolvedValue("new-token"));
    const replay = vi.fn().mockResolvedValue({ code: 0, data: { ok: true } });
    const unauthorized = { response: { status: 401 } };
    await recoverRequestError({ error: { ...unauthorized, config: { url: "/api/default" } }, replay, onFailure: vi.fn() });
    expect(replay).toHaveBeenLastCalledWith("/api/default", expect.objectContaining({ __authRetried: true }));
    await recoverRequestError({ error: { ...unauthorized, config: { url: "/api/raw", getResponse: true } }, replay, onFailure: vi.fn() });
    expect(replay).toHaveBeenLastCalledWith("/api/raw", expect.objectContaining({ getResponse: true, __authRetried: true }));
  });
  it("single-flights concurrent 401 refresh and retries once", async () => {
    const refresh = vi.fn().mockResolvedValue("new-token");
    configureSessionRefresh(refresh);
    const request = vi.fn().mockResolvedValue({ ok: true });
    await Promise.all([
      recoverUnauthorized({
        url: "/api/x",
        replay: request,
        onFailure: vi.fn(),
      }),
      recoverUnauthorized({
        url: "/api/y",
        replay: request,
        onFailure: vi.fn(),
      }),
    ]);
    expect(refresh).toHaveBeenCalledTimes(1);
  });
  it("clears the session and redirects only once when a shared refresh fails", async () => {
    setSession("old-token", "secure_session");
    configureSessionRefresh(
      vi.fn().mockRejectedValue(new Error("refresh failed")),
    );
    const unauthorized = Object.assign(new Error("unauthorized"), {
      response: { status: 401 },
    });
    const redirect = vi.fn();
    const request = vi.fn().mockRejectedValue(unauthorized);
    await expect(
      Promise.all([
        recoverUnauthorized({
          url: "/api/orders",
          replay: request,
          onFailure: redirect,
        }),
        recoverUnauthorized({
          url: "/api/products",
          replay: request,
          onFailure: redirect,
        }),
      ]),
    ).rejects.toThrow("refresh failed");
    expect(getAccessToken()).toBe("");
    expect(localStorage.getItem("trademind_admin_token")).toBeNull();
    expect(redirect).toHaveBeenCalledTimes(1);
  });
  it("does not refresh auth endpoints or retry an already replayed 401", async () => {
    const refresh = vi.fn().mockResolvedValue("new-token");
    configureSessionRefresh(refresh);
    const unauthorized = Object.assign(new Error("unauthorized"), {
      response: { status: 401 },
    });
    const authRequest = vi.fn().mockRejectedValue(unauthorized);
    await expect(
      recoverUnauthorized({
        url: "/api/v1/auth/refresh",
        replay: authRequest,
        onFailure: vi.fn(),
      }),
    ).rejects.toThrow("unauthorized");
    expect(refresh).not.toHaveBeenCalled();

    const retriedRequest = vi.fn().mockRejectedValue(unauthorized);
    await expect(
      recoverUnauthorized({
        url: "/api/orders",
        retried: true,
        replay: retriedRequest,
        onFailure: vi.fn(),
      }),
    ).rejects.toThrow("unauthorized");
    expect(retriedRequest).not.toHaveBeenCalled();
    expect(refresh).not.toHaveBeenCalled();
  });
  it("marks and replays a rejected Umi request exactly once after refresh", async () => {
    configureSessionRefresh(vi.fn().mockResolvedValue("new-token"));
    const config = { url: "/api/v1/orders", method: "GET" };
    const replay = vi.fn(
      async (_url: string, replayConfig: Record<string, unknown>) => {
        expect(getAccessToken()).toBe("new-token");
        expect(replayConfig.__authRetried).toBe(true);
        return { data: { code: 0 } };
      },
    );
    await expect(
      recoverRequestError({
        error: { config, response: { status: 401 } },
        replay,
        onFailure: vi.fn(),
      }),
    ).resolves.toEqual({ data: { code: 0 } });
    expect(replay).toHaveBeenCalledTimes(1);
    await expect(
      recoverRequestError({
        error: { config, response: { status: 401 } },
        replay,
        onFailure: vi.fn(),
      }),
    ).rejects.toEqual(expect.objectContaining({ response: { status: 401 } }));
    expect(replay).toHaveBeenCalledTimes(1);
  });
  it("does not resurrect a session when logout wins a refresh race", async () => {
    let resolveRefresh: (token: string) => void = () => undefined;
    configureSessionRefresh(
      () =>
        new Promise<string>((resolve) => {
          resolveRefresh = resolve;
        }),
    );
    const pending = recoverUnauthorized({
      url: "/api/v1/orders",
      replay: vi.fn(),
      onFailure: vi.fn(),
    });
    clearSession();
    resolveRefresh("late-token");
    await expect(pending).rejects.toThrow("session was cleared during refresh");
    expect(getAccessToken()).toBe("");
  });
  it("fails closed for unknown roles", () => {
    expect(normalizeRole("evil-admin")).toBe("readonly");
    expect(normalizeRole(null)).toBe("readonly");
    expect(permissionsForRole("evil-admin", ["sku.bind"])).not.toContain(
      "sku.bind",
    );
  });
});
