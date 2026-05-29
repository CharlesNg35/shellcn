import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { setActivePinia, createPinia } from "pinia";
import { defineComponent, h, type PropType } from "vue";
import AutoComplete from "primevue/autocomplete";
import InputText from "primevue/inputtext";
import Button from "primevue/button";
import ConfirmDialog from "primevue/confirmdialog";
import { installFetch } from "../test/fetchMock";
import { useAuthStore } from "../stores/auth";
import { Role } from "../constants/roles";
import ShareDialog from "./ShareDialog.vue";

// ShareDialog raises its revoke confirmation through PrimeVue's ConfirmationService,
// so a global <ConfirmDialog> must be present to render the accept button.
const Harness = defineComponent({
  props: {
    visible: { type: Boolean, required: true },
    resource: {
      type: String as PropType<"connections" | "credentials">,
      required: true,
    },
    resourceId: { type: String, required: true },
    resourceName: { type: String, required: true },
  },
  setup(props) {
    return () => h("div", [h(ShareDialog, props), h(ConfirmDialog)]);
  },
});

beforeEach(() => setActivePinia(createPinia()));
afterEach(() => vi.unstubAllGlobals());

describe("ShareDialog", () => {
  it("admin lists, grants by picker, and revokes access", async () => {
    useAuthStore().user = {
      id: "admin",
      username: "admin",
      roles: [Role.Admin],
    };
    let grants: Record<string, unknown>[] = [];
    const fetchFn = installFetch((url, init) => {
      if (String(url).includes("/api/admin/users/search")) {
        return { body: [{ id: "u-bob", username: "bob", displayName: "Bob" }] };
      }
      if (url.includes("/grants") && (!init || init.method === "GET")) {
        return { body: grants };
      }
      if (url.includes("/grants") && init?.method === "POST") {
        const g = {
          id: "g1",
          subjectId: "u-bob",
          username: "bob",
          access: "use",
        };
        grants = [g];
        return { status: 201, body: g };
      }
      if (url.includes("/grants/g1") && init?.method === "DELETE") {
        grants = [];
        return { body: { ok: true } };
      }
      return { body: {} };
    });

    const wrapper = mount(Harness, {
      props: {
        visible: true,
        resource: "credentials",
        resourceId: "c1",
        resourceName: "ops key",
      },
    });
    await flushPromises();

    // Pick the subject and add the grant.
    await wrapper
      .findComponent(AutoComplete)
      .vm.$emit("complete", { query: "bo" });
    await flushPromises();
    await wrapper.findComponent(AutoComplete).vm.$emit("update:modelValue", {
      id: "u-bob",
      username: "bob",
      displayName: "Bob",
      label: "Bob (bob)",
    });
    const addBtn = wrapper
      .findAllComponents(Button)
      .find((b) => b.text().trim() === "Add");
    await addBtn?.trigger("click");
    await flushPromises();

    const posted = fetchFn.mock.calls.find(
      ([u, i]) =>
        String(u).includes("/credentials/c1/grants") &&
        (i as RequestInit | undefined)?.method === "POST",
    );
    expect(posted).toBeTruthy();
    expect(
      fetchFn.mock.calls.some(([u]) =>
        String(u).includes("/api/admin/users/search?query=bo"),
      ),
    ).toBe(true);
    expect(document.body.textContent).toContain("bob");

    // Revoke removes it from the list.
    const revoke = document.body.querySelector<HTMLElement>(
      '[aria-label="Revoke bob"]',
    );
    expect(revoke).toBeTruthy();
    revoke?.click();
    await flushPromises();
    const confirm = Array.from(document.body.querySelectorAll("button")).find(
      (b) => b.textContent?.trim() === "Revoke",
    ) as HTMLElement | undefined;
    expect(confirm).toBeTruthy();
    confirm?.click();
    await flushPromises();
    const deleted = fetchFn.mock.calls.find(
      ([u, i]) =>
        String(u).includes("/credentials/c1/grants/g1") &&
        (i as RequestInit | undefined)?.method === "DELETE",
    );
    expect(deleted).toBeTruthy();

    wrapper.unmount();
  });

  it("operators share by email (no user enumeration)", async () => {
    useAuthStore().user = {
      id: "op",
      username: "op",
      roles: [Role.Operator],
    };
    const fetchFn = installFetch((url, init) => {
      if (url.includes("/grants") && init?.method === "POST") {
        return {
          status: 201,
          body: { id: "g1", subjectId: "u-z", username: "z", access: "use" },
        };
      }
      return { body: [] };
    });

    const wrapper = mount(Harness, {
      props: {
        visible: true,
        resource: "credentials",
        resourceId: "c1",
        resourceName: "ops key",
      },
    });
    await flushPromises();

    // No autocomplete for operators — an email field instead.
    expect(wrapper.findComponent(AutoComplete).exists()).toBe(false);
    await wrapper.findComponent(InputText).setValue("z@example.com");
    await wrapper
      .findAllComponents(Button)
      .find((b) => b.text().trim() === "Add")
      ?.trigger("click");
    await flushPromises();

    // It never queries the user directory, and posts the email.
    expect(
      fetchFn.mock.calls.some(([u]) =>
        String(u).includes("/admin/users/search"),
      ),
    ).toBe(false);
    const posted = fetchFn.mock.calls.find(
      ([u, i]) =>
        String(u).includes("/grants") &&
        (i as RequestInit | undefined)?.method === "POST",
    );
    expect(JSON.parse(String((posted?.[1] as RequestInit).body))).toMatchObject(
      {
        email: "z@example.com",
      },
    );

    wrapper.unmount();
  });
});
