import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { setActivePinia, createPinia } from "pinia";
import InputText from "primevue/inputtext";
import Button from "primevue/button";
import { installFetch } from "../test/fetchMock";
import InviteDialog from "./InviteDialog.vue";

beforeEach(() => setActivePinia(createPinia()));
afterEach(() => vi.unstubAllGlobals());

describe("InviteDialog", () => {
  it("creates an invitation and reveals the copyable link", async () => {
    installFetch((url, init) => {
      if (url.endsWith("/api/admin/invitations") && init?.method === "POST") {
        return {
          status: 201,
          body: {
            invitation: {
              id: "i1",
              email: "new@example.com",
              role: "viewer",
              status: "pending",
              createdAt: new Date().toISOString(),
              expiresAt: new Date().toISOString(),
            },
            link: "https://host/invite/tok-123",
            emailSent: false,
          },
        };
      }
      return { body: {} };
    });

    const wrapper = mount(InviteDialog, { props: { visible: true } });
    await flushPromises();

    wrapper
      .findAllComponents(InputText)[0]
      .vm.$emit("update:modelValue", "new@example.com");
    await flushPromises();
    const create = wrapper
      .findAllComponents(Button)
      .find((b) => b.text().includes("Create invitation"));
    await create?.trigger("click");
    await flushPromises();

    expect(wrapper.emitted("created")).toBeTruthy();
    expect(document.body.textContent).toContain("https://host/invite/tok-123");
    wrapper.unmount();
  });
});
