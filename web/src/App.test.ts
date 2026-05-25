import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { createRouter, createMemoryHistory, type Router } from "vue-router";
import { createPinia } from "pinia";
import { installFetch } from "./test/fetchMock";
import App from "./App.vue";

const connections = [
  {
    id: "a",
    name: "alpha-host",
    protocol: "ssh",
    icon: { type: "name", value: "terminal" },
    transport: "direct",
    online: true,
  },
];
const plugins = [
  { name: "ssh", title: "SSH", icon: { type: "name", value: "terminal" } },
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
  installFetch((url) => {
    if (url.endsWith("/api/connections")) return { body: connections };
    if (url.endsWith("/api/plugins")) return { body: plugins };
    return { status: 404, body: { error: "not found" } };
  });
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("App shell", () => {
  it("renders the shell and the loaded connections", async () => {
    const router = testRouter();
    router.push("/");
    await router.isReady();
    const wrapper = mount(App, {
      global: { plugins: [createPinia(), router] },
    });
    await flushPromises();

    expect(wrapper.text()).toContain("ShellCN");
    expect(wrapper.text()).toContain("alpha-host");
  });
});
