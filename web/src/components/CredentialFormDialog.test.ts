import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { setActivePinia, createPinia } from "pinia";
import Button from "primevue/button";
import { installFetch } from "../test/fetchMock";
import CredentialFormDialog from "./CredentialFormDialog.vue";

beforeEach(() => setActivePinia(createPinia()));
afterEach(() => vi.unstubAllGlobals());

describe("CredentialFormDialog", () => {
  it("rotates an existing secret behind a write-only Replace affordance", async () => {
    let put: Record<string, unknown> | null = null;
    installFetch((url, init) => {
      if (url.includes("/credentials/c1") && init?.method === "PUT") {
        put = JSON.parse(String(init.body));
        return { body: { id: "c1" } };
      }
      return { body: {} };
    });

    const wrapper = mount(CredentialFormDialog, {
      props: {
        visible: true,
        credential: {
          id: "c1",
          name: "ops key",
          kind: "ssh_password",
          ownerId: "u-demo",
        },
      },
    });
    await flushPromises();

    // The secret is write-only: a "Replace" affordance, not a populated input.
    expect(document.body.textContent).toContain("Replace");
    expect(document.body.querySelector('input[type="password"]')).toBeFalsy();

    // Saving without replacing keeps the stored secret (blank secret in body).
    const save = wrapper
      .findAllComponents(Button)
      .find((b) => b.text().includes("Save changes"));
    await save?.trigger("click");
    await flushPromises();

    expect(put).toMatchObject({
      name: "ops key",
      kind: "ssh_password",
      secret: "",
    });
    expect(wrapper.emitted("saved")).toBeTruthy();
    wrapper.unmount();
  });

  it("requires secret material when creating", async () => {
    const fetchFn = installFetch(() => ({ status: 201, body: { id: "new" } }));
    const wrapper = mount(CredentialFormDialog, {
      props: { visible: true, credential: null },
    });
    await flushPromises();

    const save = wrapper
      .findAllComponents(Button)
      .find((b) => b.text().includes("Create credential"));
    await save?.trigger("click");
    await flushPromises();

    // No POST happens because name + secret are required and empty.
    const posted = fetchFn.mock.calls.find(
      ([, i]) => (i as RequestInit | undefined)?.method === "POST",
    );
    expect(posted).toBeUndefined();
    expect(document.body.textContent).toContain("required");
    wrapper.unmount();
  });
});
