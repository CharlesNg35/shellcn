import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { createRouter, createMemoryHistory, type Router } from "vue-router";
import { createPinia, setActivePinia } from "pinia";
import { installFetch } from "./test/fetchMock";
import { useAuthStore } from "./stores/auth";
import App from "./App.vue";

const connections = [
  {
    id: "a",
    name: "alpha-host",
    protocol: "ssh",
    icon: { type: "lucide", value: "terminal" },
    transport: "direct",
    online: true,
  },
];
const plugins = [
  {
    name: "ssh",
    title: "SSH",
    icon: { type: "lucide", value: "terminal" },
    category: {
      key: "shell",
      label: "Shell & terminal",
      icon: { type: "lucide", value: "terminal" },
      order: 10,
    },
  },
];

function testRouter(): Router {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      {
        path: "/login",
        name: "login",
        component: () => import("./views/LoginView.vue"),
      },
      {
        path: "/",
        component: () => import("./components/AppShell.vue"),
        children: [
          {
            path: "",
            name: "home",
            component: () => import("./views/HomeView.vue"),
          },
          {
            path: "c/:id",
            name: "connection",
            component: () => import("./views/ConnectionWorkspace.vue"),
            props: true,
          },
          {
            path: "credentials",
            name: "credentials",
            component: () => import("./views/CredentialsView.vue"),
          },
          {
            path: "profile",
            name: "profile",
            component: () => import("./views/ProfileView.vue"),
          },
          {
            path: "recordings",
            name: "recordings",
            component: () => import("./views/RecordingsView.vue"),
          },
          {
            path: "settings",
            name: "settings",
            component: () => import("./views/SettingsView.vue"),
          },
        ],
      },
    ],
  });
}

beforeEach(() => {
  localStorage.clear();
  installFetch((url) => {
    if (url.endsWith("/api/connections")) return { body: connections };
    if (url.endsWith("/api/connection-folders")) return { body: [] };
    if (url.endsWith("/api/plugins")) return { body: plugins };
    return { status: 404, body: { error: "not found" } };
  });
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("App shell", () => {
  it("shows a branded loader while the session bootstrap is pending", async () => {
    const pinia = createPinia();
    setActivePinia(pinia);
    const auth = useAuthStore();
    auth.ready = false;

    const router = testRouter();
    router.push("/");
    await router.isReady();
    const wrapper = mount(App, {
      global: { plugins: [pinia, router] },
    });

    expect(wrapper.get('[role="status"]').attributes("aria-label")).toBe(
      "Loading ShellCN",
    );
    expect(wrapper.findComponent({ name: "AppLogo" }).exists()).toBe(false);
  });

  it("renders the shell and the loaded connections", async () => {
    const pinia = createPinia();
    setActivePinia(pinia);
    // App gates the shell behind a session-ready bootstrap; mark it authenticated.
    const auth = useAuthStore();
    auth.user = { id: "u", username: "op", roles: ["viewer"] };
    auth.ready = true;

    const router = testRouter();
    router.push("/");
    await router.isReady();
    const wrapper = mount(App, {
      global: { plugins: [pinia, router] },
    });
    await flushPromises();

    expect(wrapper.text()).toContain("ShellCN");
    expect(wrapper.text()).toContain("alpha-host");
  });

  it("collapses the sidebar utility menu so connections get more height", async () => {
    const pinia = createPinia();
    setActivePinia(pinia);
    const auth = useAuthStore();
    auth.user = { id: "u", username: "op", roles: ["viewer"] };
    auth.ready = true;

    const router = testRouter();
    router.push("/");
    await router.isReady();
    const wrapper = mount(App, {
      global: { plugins: [pinia, router] },
    });
    await flushPromises();

    const menu = wrapper.get("#sidebar-utility-menu");
    const hide = wrapper.get('button[aria-label="Hide sidebar menu"]');
    expect(menu.isVisible()).toBe(true);
    expect(menu.attributes("aria-hidden")).toBe("false");
    expect(wrapper.get('button[aria-label="Sign out"]').isVisible()).toBe(true);
    expect(hide.attributes("aria-expanded")).toBe("true");

    await hide.trigger("click");

    expect(menu.attributes("aria-hidden")).toBe("true");
    expect(wrapper.get('button[aria-label="Sign out"]').isVisible()).toBe(true);
    expect(
      wrapper
        .get('button[aria-label="Show sidebar menu"]')
        .attributes("aria-expanded"),
    ).toBe("false");
    expect(localStorage.getItem("shellcn:sidebar-menu:open")).toBe("false");

    wrapper.unmount();
    const nextPinia = createPinia();
    setActivePinia(nextPinia);
    const nextAuth = useAuthStore();
    nextAuth.user = { id: "u", username: "op", roles: ["viewer"] };
    nextAuth.ready = true;
    const nextRouter = testRouter();
    nextRouter.push("/");
    await nextRouter.isReady();
    const restored = mount(App, {
      global: { plugins: [nextPinia, nextRouter] },
    });
    await flushPromises();

    expect(
      restored.get("#sidebar-utility-menu").attributes("aria-hidden"),
    ).toBe("true");
    expect(restored.get('button[aria-label="Sign out"]').isVisible()).toBe(
      true,
    );
  });
});
