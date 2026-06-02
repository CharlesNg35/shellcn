import { defineComponent, ref } from "vue";
import { mount, flushPromises } from "@vue/test-utils";
import { createMemoryHistory, createRouter, type Router } from "vue-router";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it } from "vitest";
import { useWorkspaceStore } from "../stores/workspace";
import { Layout, type PluginProjection } from "../types/projection";
import { useWorkspaceUrlSync } from "./useWorkspaceUrlSync";

const projection: PluginProjection = {
  apiVersion: 2,
  name: "docker",
  version: "0.1.0",
  title: "Docker",
  description: "",
  icon: { type: "lucide", value: "box" },
  category: {
    key: "containers",
    label: "Containers",
    icon: { type: "lucide", value: "box" },
    order: 1,
  },
  config: { groups: [] },
  capabilities: [],
  supportedTransports: ["direct"],
  layout: Layout.SidebarTree,
  tree: [{ key: "containers", label: "Containers", resourceKind: "container" }],
  resources: [
    {
      kind: "container",
      title: "Containers",
      list: { routeId: "docker.containers" },
      columns: [],
      detail: { header: {}, tabs: [] },
    },
  ],
  actions: [],
};

function testRouter(): Router {
  return createRouter({
    history: createMemoryHistory(),
    routes: [{ path: "/", component: { template: "<div />" } }],
  });
}

function mountSync(router: Router, initial = projection) {
  const Host = defineComponent({
    setup() {
      const proj = ref<PluginProjection | null>(initial);
      const sync = useWorkspaceUrlSync({
        connectionId: () => "c1",
        projection: proj,
      });
      return { proj, restoreFromUrl: sync.restoreFromUrl };
    },
    template: "<div />",
  });
  return mount(Host, { global: { plugins: [router] } });
}

describe("useWorkspaceUrlSync", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it("restores the active sidebar-tree view from the URL", async () => {
    const router = testRouter();
    await router.push("/?v=group:containers&vc=c1");
    const wrapper = mountSync(router);

    wrapper.vm.restoreFromUrl();

    expect(useWorkspaceStore().activeView("c1")?.id).toBe("group:containers");
  });

  it("pushes newly opened views but replaces already-open tab switches", async () => {
    const router = testRouter();
    const wrapper = mountSync(router);
    const ws = useWorkspaceStore();

    ws.openView("c1", {
      id: "group:containers",
      title: "Containers",
      kind: "list",
      groupKey: "containers",
    });
    await flushPromises();
    expect(router.currentRoute.value.query.v).toBe("group:containers");
    expect(router.currentRoute.value.query.vc).toBe("c1");

    ws.openView("c1", {
      id: "detail:abc",
      title: "web",
      kind: "detail",
      ref: { kind: "container", uid: "abc", name: "web" },
    });
    await flushPromises();
    expect(router.currentRoute.value.query.v).toBe(
      "detail:container:abc:n=web",
    );

    ws.activateView("c1", "group:containers");
    await flushPromises();
    expect(router.currentRoute.value.query.v).toBe("group:containers");

    await router.back();
    await flushPromises();
    expect(router.currentRoute.value.query.v).not.toBe(
      "detail:container:abc:n=web",
    );

    wrapper.unmount();
  });

  it("does not rewrite the URL while the projection is unavailable", async () => {
    const router = testRouter();
    await router.push("/?v=group:containers&vc=c1");
    const wrapper = mountSync(router);

    wrapper.vm.proj = null;
    await flushPromises();

    expect(router.currentRoute.value.query.v).toBe("group:containers");
  });

  it("ignores a sidebar-tree locator owned by another connection", async () => {
    const router = testRouter();
    await router.push("/?v=detail:container:abc:n=web&vc=other");
    const wrapper = mountSync(router);

    wrapper.vm.restoreFromUrl();

    expect(useWorkspaceStore().activeView("c1")).toBeUndefined();
  });
});
