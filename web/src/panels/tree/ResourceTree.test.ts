import { describe, it, expect } from "vitest";
import { mount } from "@vue/test-utils";
import ResourceTree from "./ResourceTree.vue";
import type { TreeGroup } from "../../types/projection";

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
    expect(w.emitted("select-list")?.[0]).toEqual(["pod"]);
    expect(w.emitted("select-node")).toBeUndefined();
  });

  it("emits select-node for a leaf with a ref (detail)", async () => {
    const w = mountTree();
    const dt = w.findComponent({ name: "Tree" });
    const row = { ref: { kind: "pod", name: "p", uid: "p1" } };
    dt.vm.$emit("node-select", { key: "p1", leaf: true, data: { row } });
    await w.vm.$nextTick();
    expect(w.emitted("select-node")?.[0]).toEqual([row]);
  });
});
