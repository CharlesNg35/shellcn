import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { setActivePinia, createPinia } from "pinia";
import { installFetch } from "../test/fetchMock";
import router from "./index";

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

describe("router auth guard", () => {
  it("redirects an unauthenticated visitor to /login with a redirect query", async () => {
    installFetch((url) => {
      if (url.endsWith("/api/auth/me")) return { status: 401, body: {} };
      return { body: {} };
    });
    await router.push("/settings");
    await router.isReady();
    expect(router.currentRoute.value.name).toBe("login");
    expect(router.currentRoute.value.query.redirect).toBe("/settings");
  });

  it("lets an authenticated visitor reach a protected route", async () => {
    installFetch((url) => {
      if (url.endsWith("/api/auth/me")) return { body: session };
      return { body: [] };
    });
    await router.push("/settings");
    await router.isReady();
    expect(router.currentRoute.value.name).toBe("settings");
  });

  it("bounces an authenticated visitor away from /login", async () => {
    installFetch((url) => {
      if (url.endsWith("/api/auth/me")) return { body: session };
      return { body: [] };
    });
    await router.push("/login");
    await router.isReady();
    expect(router.currentRoute.value.name).toBe("home");
  });
});
