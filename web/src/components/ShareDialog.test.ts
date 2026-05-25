import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { setActivePinia, createPinia } from "pinia";
import AutoComplete from "primevue/autocomplete";
import Button from "primevue/button";
import { installFetch } from "../test/fetchMock";
import ShareDialog from "./ShareDialog.vue";

beforeEach(() => setActivePinia(createPinia()));
afterEach(() => vi.unstubAllGlobals());

describe("ShareDialog", () => {
  it("lists, grants, and revokes access", async () => {
    let grants: Record<string, unknown>[] = [];
    const fetchFn = installFetch((url, init) => {
      if (String(url).includes("/api/users")) {
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

    const wrapper = mount(ShareDialog, {
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
        String(u).includes("/api/users?query=bo"),
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
});
