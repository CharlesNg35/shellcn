import { describe, it, expect } from "vitest";
import { flushPromises, mount } from "@vue/test-utils";
import ResourceTree from "./ResourceTree.vue";
import { installFetch } from "@/test/fetchMock";
import type { TreeGroup } from "@/types/projection";

const groups: TreeGroup[] = [
  {
    key: "workloads",
    label: "Workloads",
    source: { routeId: "k8s.tree.workloads" },
  },
];

function mountTree() {
  return mount(ResourceTree, {
    props: { connectionId: "c1", groups },
    global: { stubs: { AppIcon: true } },
  });
}

describe("ResourceTree", () => {
  it("emits select-group when a top-level group is clicked", async () => {
    const w = mountTree();
    const dt = w.findComponent({ name: "Tree" });
    dt.vm.$emit("node-select", {
      key: "workloads",
      children: [],
      data: { isGroup: true, source: groups[0].source },
    });
    await w.vm.$nextTick();
    expect(w.emitted("select-group")?.[0]).toEqual(["workloads"]);
  });

  it("emits select-list with the kind for a list-opening node", async () => {
    const w = mountTree();
    const dt = w.findComponent({ name: "Tree" });
    dt.vm.$emit("node-select", {
      key: "wl:pods",
      leaf: true,
      data: { resourceKind: "pod" },
    });
    await w.vm.$nextTick();
    expect(w.emitted("select-list")?.[0]).toEqual(["pod", undefined]);
    expect(w.emitted("select-node")).toBeUndefined();
  });

  it("passes scoping params on a list node (e.g. a namespace)", async () => {
    const w = mountTree();
    const dt = w.findComponent({ name: "Tree" });
    dt.vm.$emit("node-select", {
      key: "ns:prod:pods",
      leaf: true,
      data: { resourceKind: "pod", listParams: { namespace: "prod" } },
    });
    await w.vm.$nextTick();
    expect(w.emitted("select-list")?.[0]).toEqual([
      "pod",
      { namespace: "prod" },
    ]);
  });

  it("emits select-node for a leaf with a ref (detail)", async () => {
    const w = mountTree();
    const dt = w.findComponent({ name: "Tree" });
    const row = { ref: { kind: "pod", name: "p", uid: "p1" } };
    dt.vm.$emit("node-select", { key: "p1", leaf: true, data: { row } });
    await w.vm.$nextTick();
    expect(w.emitted("select-node")?.[0]).toEqual([row, ""]);
  });

  it("emits the intermediate ancestor path as the tab qualifier", async () => {
    const w = mountTree();
    const dt = w.findComponent({ name: "Tree" });
    const row = {
      ref: { kind: "table", name: "users", uid: "app.public.users" },
    };
    dt.vm.$emit("node-select", {
      key: "t1",
      leaf: true,
      data: { row, groupLabel: "Databases", parentPath: ["app", "public"] },
    });
    await w.vm.$nextTick();
    expect(w.emitted("select-node")?.[0]).toEqual([row, "app / public"]);
  });

  it("falls back to the root group name for a node directly under it", async () => {
    const w = mountTree();
    const dt = w.findComponent({ name: "Tree" });
    const row = { ref: { kind: "container", name: "web", uid: "c1" } };
    dt.vm.$emit("node-select", {
      key: "c1",
      leaf: true,
      data: { row, groupLabel: "Containers", parentPath: [] },
    });
    await w.vm.$nextTick();
    expect(w.emitted("select-node")?.[0]).toEqual([row, "Containers"]);
  });

  it("keeps the parent category expanded when a child list node is selected", async () => {
    const w = mountTree();
    const dt = w.findComponent({ name: "Tree" });
    dt.vm.$emit("node-select", {
      key: "workloads",
      leaf: false,
      children: [],
      data: { isGroup: true, source: groups[0].source },
    });
    await w.vm.$nextTick();
    expect(dt.props("expandedKeys")).toMatchObject({ workloads: true });

    dt.vm.$emit("update:expanded-keys", {});
    dt.vm.$emit("node-select", {
      key: "kind:pod",
      leaf: true,
      data: { resourceKind: "pod" },
    });
    await w.vm.$nextTick();

    expect(w.emitted("select-list")?.at(-1)).toEqual(["pod", undefined]);
    expect(dt.props("expandedKeys")).toMatchObject({ workloads: true });
  });

  it("does not change expansion when history restores a selected view", async () => {
    const w = mountTree();
    const dt = w.findComponent({ name: "Tree" });
    await w.setProps({ selectedGroup: "workloads" });
    expect(dt.props("expandedKeys")).toEqual({});

    dt.vm.$emit("node-select", {
      key: "workloads",
      leaf: false,
      children: [],
      data: { isGroup: true, source: groups[0].source },
    });
    await w.vm.$nextTick();
    expect(dt.props("expandedKeys")).toMatchObject({ workloads: true });

    await w.setProps({ selectedUid: "pod-1", selectedGroup: undefined });
    expect(dt.props("expandedKeys")).toMatchObject({ workloads: true });
  });

  it("collapses a category only from the explicit node-collapse event", async () => {
    const w = mountTree();
    const dt = w.findComponent({ name: "Tree" });
    const workloads = {
      key: "workloads",
      leaf: false,
      children: [],
      data: { isGroup: true, source: groups[0].source },
    };
    dt.vm.$emit("node-select", workloads);
    await w.vm.$nextTick();
    dt.vm.$emit("node-collapse", workloads);
    await w.vm.$nextTick();
    expect(dt.props("expandedKeys")).not.toHaveProperty("workloads");
  });

  it("reloads expanded branches on refresh without collapsing them", async () => {
    const requests: string[] = [];
    installFetch((url) => {
      requests.push(String(url));
      return {
        body: {
          items: [
            {
              key: "kind:pod",
              label: "Pods",
              resourceKind: "pod",
            },
          ],
        },
      };
    });
    const w = mountTree();
    const dt = w.findComponent({ name: "Tree" });
    const root = dt.props("value")[0];

    dt.vm.$emit("node-expand", root);
    await flushPromises();
    expect(dt.props("expandedKeys")).toMatchObject({ workloads: true });
    expect(requests).toHaveLength(1);

    await w.setProps({ refreshKey: "namespace=prod" });
    await w.vm.$nextTick();
    expect(dt.props("value")[0].loading).not.toBe(true);
    await flushPromises();

    expect(dt.props("expandedKeys")).toMatchObject({ workloads: true });
    expect(requests).toHaveLength(2);
  });
});
