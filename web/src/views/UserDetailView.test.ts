import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { mount, flushPromises, type VueWrapper } from "@vue/test-utils";
import { setActivePinia, createPinia } from "pinia";
import { useAuthStore } from "../stores/auth";
import { Role } from "../constants/roles";
import type { AdminUser } from "../types/projection";

const getUser = vi.fn<(...a: unknown[]) => Promise<AdminUser>>();
const deactivate = vi.fn<(...a: unknown[]) => Promise<AdminUser>>();
vi.mock("../api/admin", () => ({
  adminUsersApi: {
    get: (...a: unknown[]) => getUser(...a),
    connections: vi.fn(async () => []),
    audit: vi.fn(async () => ({ items: [], total: 0 })),
    activate: vi.fn(),
    deactivate: (...a: unknown[]) => deactivate(...a),
  },
}));
vi.mock("vue-router", () => ({ useRouter: () => ({ push: vi.fn() }) }));

import UserDetailView from "./UserDetailView.vue";

const target: AdminUser = {
  id: "u1",
  username: "alice",
  email: "alice@example.com",
  displayName: "Alice",
  roles: [Role.Operator],
  disabled: false,
  protected: false,
};

let wrapper: VueWrapper | undefined;

beforeEach(() => {
  vi.useFakeTimers();
  setActivePinia(createPinia());
  const auth = useAuthStore();
  auth.user = { id: "admin", username: "admin", roles: [Role.Admin] };
  getUser.mockResolvedValue(target);
  deactivate.mockResolvedValue({ ...target, disabled: true });
});

afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
  vi.clearAllTimers();
  vi.useRealTimers();
});

function mountView(id: string) {
  wrapper = mount(UserDetailView, { props: { id } });
  return wrapper;
}

function deactivateButton(w: ReturnType<typeof mount>) {
  return w
    .findAll("button")
    .find((b) => b.text().includes("Deactivate account"));
}

describe("UserDetailView", () => {
  it("shows the user's overview and deactivates a manageable account", async () => {
    const w = mountView("u1");
    await flushPromises();

    expect(getUser).toHaveBeenCalledWith("u1");
    expect(w.text()).toContain("alice");
    expect(w.text()).toContain("alice@example.com");

    const btn = deactivateButton(w);
    expect(btn).toBeTruthy();
    await btn!.trigger("click");
    await flushPromises();
    expect(deactivate).toHaveBeenCalledWith("u1");
  });

  it("never offers to deactivate the protected root admin", async () => {
    getUser.mockResolvedValue({
      ...target,
      roles: [Role.Admin],
      protected: true,
    });
    const w = mountView("u1");
    await flushPromises();
    expect(deactivateButton(w)).toBeUndefined();
  });

  it("never offers self-deactivation", async () => {
    getUser.mockResolvedValue({ ...target, id: "admin" });
    const w = mountView("admin");
    await flushPromises();
    expect(deactivateButton(w)).toBeUndefined();
  });
});
