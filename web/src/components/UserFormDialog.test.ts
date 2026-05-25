import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { setActivePinia, createPinia } from "pinia";
import InputText from "primevue/inputtext";
import Password from "primevue/password";
import Button from "primevue/button";
import { installFetch } from "../test/fetchMock";
import UserFormDialog from "./UserFormDialog.vue";
import type { AdminUser } from "../types/projection";

beforeEach(() => setActivePinia(createPinia()));
afterEach(() => vi.unstubAllGlobals());

describe("UserFormDialog", () => {
  it("creates a user with a chosen role and password", async () => {
    let posted: Record<string, unknown> | null = null;
    installFetch((url, init) => {
      if (url.endsWith("/api/admin/users") && init?.method === "POST") {
        posted = JSON.parse(String(init.body));
        return { status: 201, body: { id: "u1" } };
      }
      return { body: {} };
    });

    const wrapper = mount(UserFormDialog, {
      props: { visible: true, user: null },
    });
    await flushPromises();

    wrapper
      .findAllComponents(InputText)[0]
      .vm.$emit("update:modelValue", "alice");
    wrapper.findComponent(Password).vm.$emit("update:modelValue", "s3cret-pw");
    await flushPromises();

    const create = wrapper
      .findAllComponents(Button)
      .find((b) => b.text().includes("Create user"));
    await create?.trigger("click");
    await flushPromises();

    expect(posted).toMatchObject({
      username: "alice",
      role: "viewer",
      password: "s3cret-pw",
    });
    expect(wrapper.emitted("saved")).toBeTruthy();
    wrapper.unmount();
  });

  it("locks role and disabled controls for the protected root admin", async () => {
    installFetch(() => ({ body: {} }));
    const root: AdminUser = {
      id: "root",
      username: "admin",
      roles: ["admin"],
      disabled: false,
      protected: true,
    };
    const wrapper = mount(UserFormDialog, {
      props: { visible: true, user: root },
    });
    await flushPromises();

    expect(document.body.textContent).toContain(
      "root admin must remain an enabled admin",
    );
    wrapper.unmount();
  });
});
