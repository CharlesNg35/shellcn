import { describe, it, expect, beforeEach, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { computed } from "vue";

vi.mock("@/composables/useNotify", () => ({
  useNotify: () => ({ success: () => {}, error: () => {} }),
}));

import { useActionSuccess } from "./actionSuccess";
import { useDockStore } from "@/stores/dock";
import { PanelType } from "@/types/projection";

describe("useActionSuccess open_panel effect", () => {
  beforeEach(() => setActivePinia(createPinia()));

  it("opens a dock panel resolving ${response.x} and ${resource.x}", async () => {
    const runtime = {
      connectionId: () => "c1",
      tabs: computed(() => []),
      resolvePanel: () => PanelType.Terminal,
      selectTab: () => {},
      context: () => ({
        resource: { kind: "pod", name: "web", namespace: "default", uid: "u1" },
      }),
    };
    const action = {
      id: "debug",
      label: "Debug",
      routeId: "x",
      onSuccess: {
        effects: [
          {
            type: "open_panel",
            openPanel: {
              open: "dock",
              panel: PanelType.Terminal,
              title: "Debug · ${response.name}",
              source: {
                routeId: "kubernetes.pod.exec",
                method: "WS",
                params: {
                  container: "${response.container}",
                  name: "${resource.name}",
                },
              },
            },
          },
        ],
      },
    } as never;

    await useActionSuccess(runtime).run(action, {
      container: "debugger-abc",
      name: "web",
    });

    const items = useDockStore().state("c1").items;
    expect(items).toHaveLength(1);
    expect(items[0].panel).toBe(PanelType.Terminal);
    expect(items[0].source.params?.container).toBe("debugger-abc");
    expect(items[0].source.params?.name).toBe("web");
    expect(items[0].title).toBe("Debug · web"); // title interpolates ${response.name}
  });
});
