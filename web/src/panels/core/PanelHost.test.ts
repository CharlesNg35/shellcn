import { describe, it, expect, beforeEach, vi } from "vitest";
import { mount } from "@vue/test-utils";
import PanelHost from "./PanelHost.vue";

const lifecycle = vi.hoisted(() => ({
  mounts: 0,
  unmounts: 0,
}));

vi.mock("./registry", () => ({
  resolvePanel: (type: string) =>
    type === "test_panel"
      ? {
          name: "TestPanel",
          props: { connectionId: { type: String, required: true } },
          mounted() {
            lifecycle.mounts += 1;
          },
          unmounted() {
            lifecycle.unmounts += 1;
          },
          template: "<div>panel {{ connectionId }}</div>",
        }
      : undefined,
}));

describe("PanelHost", () => {
  beforeEach(() => {
    lifecycle.mounts = 0;
    lifecycle.unmounts = 0;
  });

  it("renders a graceful fallback for an unknown panel type", () => {
    const w = mount(PanelHost, {
      props: { panel: "totally-made-up", connectionId: "c1" },
    });
    expect(w.text()).toContain("No renderer for panel type");
    expect(w.text()).toContain("totally-made-up");
  });

  it("renders a panel error for malformed generic config", () => {
    const w = mount(PanelHost, {
      props: {
        panel: "table",
        connectionId: "c1",
        config: { columns: "bad" },
      },
    });

    expect(w.text()).toContain("config.columns must be an array.");
  });

  it("validates dashboard cell config recursively", () => {
    const w = mount(PanelHost, {
      props: {
        panel: "dashboard",
        connectionId: "c1",
        config: {
          cells: [
            {
              key: "editor",
              panel: "code_editor",
              config: { saveMethod: "GET" },
            },
          ],
        },
      },
    });

    expect(w.text()).toContain(
      'config.cells[0].config.saveMethod has unsupported method "GET".',
    );
  });

  it("remounts panels when the connection changes", async () => {
    const w = mount(PanelHost, {
      props: {
        panel: "test_panel",
        connectionId: "c1",
        source: { routeId: "postgresql.query", method: "WS" },
        resource: { kind: "table", name: "users", uid: "public.users" },
      },
    });

    expect(lifecycle.mounts).toBe(1);
    expect(w.text()).toContain("panel c1");

    await w.setProps({ connectionId: "c2" });

    expect(lifecycle.unmounts).toBe(1);
    expect(lifecycle.mounts).toBe(2);
    expect(w.text()).toContain("panel c2");
  });
});
