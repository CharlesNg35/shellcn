import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { setActivePinia, createPinia } from "pinia";
import { installFetch } from "../test/fetchMock";
import { useAuthStore } from "./auth";

const session = {
  user: { id: "u1", username: "alice", roles: ["operator"] },
  csrfToken: "csrf-xyz",
};

beforeEach(() => {
  setActivePinia(createPinia());
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("auth store", () => {
  it("bootstraps an established session from /auth/me", async () => {
    installFetch((url) => {
      if (url.endsWith("/api/auth/me")) return { body: session };
      return { status: 404, body: {} };
    });
    const auth = useAuthStore();
    await auth.ensureReady();
    expect(auth.isAuthenticated).toBe(true);
    expect(auth.user?.username).toBe("alice");
    expect(auth.ready).toBe(true);
  });

  it("clears on an unauthenticated bootstrap", async () => {
    installFetch(() => ({ status: 401, body: { error: "unauthorized" } }));
    const auth = useAuthStore();
    await auth.ensureReady();
    expect(auth.isAuthenticated).toBe(false);
    expect(auth.ready).toBe(true);
  });

  it("logs in and out", async () => {
    installFetch((url) => {
      if (url.endsWith("/api/auth/login"))
        return { body: { mfaRequired: false, session } };
      if (url.endsWith("/api/auth/logout")) return { body: { ok: true } };
      return { status: 404, body: {} };
    });
    const auth = useAuthStore();
    const result = await auth.login("alice", "pw");
    expect(result.mfaRequired).toBe(false);
    expect(auth.isAuthenticated).toBe(true);
    expect(auth.isAdmin).toBe(false);
    await auth.logout();
    expect(auth.isAuthenticated).toBe(false);
  });

  it("completes a two-step login when 2FA is required", async () => {
    installFetch((url) => {
      if (url.endsWith("/api/auth/login"))
        return { body: { mfaRequired: true, mfaToken: "challenge-token" } };
      if (url.endsWith("/api/auth/login/mfa"))
        return { body: { mfaRequired: false, session } };
      return { status: 404, body: {} };
    });
    const auth = useAuthStore();
    const result = await auth.login("alice", "pw");
    expect(result.mfaRequired).toBe(true);
    expect(auth.awaitingMfa).toBe(true);
    expect(auth.isAuthenticated).toBe(false);

    await auth.completeMfa("123456");
    expect(auth.isAuthenticated).toBe(true);
    expect(auth.awaitingMfa).toBe(false);
  });

  it("bootstraps only once across concurrent callers", async () => {
    const fetchFn = installFetch((url) => {
      if (url.endsWith("/api/auth/me")) return { body: session };
      return { status: 404, body: {} };
    });
    const auth = useAuthStore();
    await Promise.all([auth.ensureReady(), auth.ensureReady()]);
    const meCalls = fetchFn.mock.calls.filter(([u]) =>
      String(u).endsWith("/api/auth/me"),
    );
    expect(meCalls).toHaveLength(1);
  });
});
