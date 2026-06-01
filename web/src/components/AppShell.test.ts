import { defineComponent, onMounted, onUnmounted } from "vue";
import { mount, flushPromises } from "@vue/test-utils";
import { createMemoryHistory, createRouter } from "vue-router";
import { createPinia, setActivePinia } from "pinia";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { installFetch } from "../test/fetchMock";
import { useAuthStore } from "../stores/auth";
import AppShell from "./AppShell.vue";

beforeEach(() => {
  setActivePinia(createPinia());
  const auth = useAuthStore();
  auth.user = { id: "u", username: "op", roles: ["viewer"] };
  auth.ready = true;
  installFetch((url) => {
    if (url.endsWith("/api/connections")) return { body: [] };
    if (url.endsWith("/api/connection-folders")) return { body: [] };
    return { status: 404, body: { error: "not found" } };
  });
});

describe("AppShell", () => {
  it("does not remount the active workspace on query-only navigation", async () => {
    let mounts = 0;
    let unmounts = 0;
    const Probe = defineComponent({
      setup() {
        onMounted(() => {
          mounts += 1;
        });
        onUnmounted(() => {
          unmounts += 1;
        });
      },
      template: "<div data-test='probe' />",
    });
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        {
          path: "/",
          component: AppShell,
          children: [
            {
              path: "c/:id",
              name: "connection",
              component: Probe,
            },
          ],
        },
      ],
    });
    await router.push("/c/a?v=one");
    await router.isReady();

    mount({ template: "<RouterView />" }, { global: { plugins: [router] } });
    await flushPromises();
    expect(mounts).toBe(1);

    await router.push("/c/a?v=two");
    await flushPromises();

    expect(mounts).toBe(1);
    expect(unmounts).toBe(0);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });
});
