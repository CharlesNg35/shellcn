import { describe, it, expect, beforeEach } from "vitest";
import { mount } from "@vue/test-utils";
import { setActivePinia, createPinia } from "pinia";
import { useAuthStore } from "../stores/auth";
import RoleGate from "./RoleGate.vue";

beforeEach(() => setActivePinia(createPinia()));

describe("RoleGate", () => {
  it("renders admin-only content for admins and hides it otherwise", () => {
    const auth = useAuthStore();
    auth.user = { id: "a", username: "a", roles: ["admin"] };
    const shown = mount(RoleGate, {
      props: { admin: true },
      slots: { default: "<p>secret</p>" },
    });
    expect(shown.text()).toContain("secret");

    auth.user = { id: "o", username: "o", roles: ["operator"] };
    const hidden = mount(RoleGate, {
      props: { admin: true },
      slots: { default: "<p>secret</p>", denied: "<p>nope</p>" },
    });
    expect(hidden.text()).not.toContain("secret");
    expect(hidden.text()).toContain("nope");
  });

  it("renders content for everyone when not admin-gated", () => {
    const auth = useAuthStore();
    auth.user = { id: "o", username: "o", roles: ["operator"] };
    const w = mount(RoleGate, { slots: { default: "<p>open</p>" } });
    expect(w.text()).toContain("open");
  });
});
