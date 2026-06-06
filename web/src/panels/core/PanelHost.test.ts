import { describe, it, expect, beforeEach, vi } from "vitest";
import { computed, defineComponent, h, nextTick } from "vue";
import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import PanelHost from "./PanelHost.vue";
import PanelLoader from "../../components/PanelLoader.vue";
import { useScopeStore } from "../../stores/scope";
import { providePanelConfigSchemas } from "./config";
import { providePanelRecordingResolver } from "./recording";
import type { PanelConfigSchemas } from "./config";

const lifecycle = vi.hoisted(() => ({
  mounts: 0,
  unmounts: 0,
}));

vi.mock("./registry", () => ({
  resolvePanel: (type: string) =>
    type === "test_panel"
      ? {
          name: "TestPanel",
          props: {
            connectionId: { type: String, required: true },
            recording: { type: Object, default: null },
          },
          mounted() {
            lifecycle.mounts += 1;
          },
          unmounted() {
            lifecycle.unmounts += 1;
          },
          template:
            '<div>panel {{ connectionId }} <span v-if="recording">{{ recording.policy }}</span></div>',
        }
      : undefined,
}));

const testSchemas: PanelConfigSchemas = {
  test_panel: {
    type: "object",
    properties: {
      zoom: { type: "boolean" },
    },
  },
  code_editor: {
    type: "object",
    properties: {
      saveMethod: {
        type: "string",
        enum: ["POST", "PUT", "PATCH", "DELETE"],
      },
    },
  },
  dashboard: {
    type: "object",
    properties: {
      cells: {
        type: "array",
        items: {
          type: "object",
          properties: {
            key: { type: "string" },
            panel: { type: "string" },
            config: { type: "object" },
          },
        },
      },
    },
  },
  table: {
    type: "object",
    properties: {
      columns: { type: "array", items: { type: "object" } },
    },
  },
};

const SchemaProvider = defineComponent({
  props: {
    recordingRoute: { type: String, default: "" },
  },
  setup(props, { slots }) {
    providePanelConfigSchemas(computed(() => testSchemas));
    providePanelRecordingResolver((source) =>
      props.recordingRoute && source?.routeId === props.recordingRoute
        ? { class: "terminal", policy: "manual", authoritative: true }
        : null,
    );
    return () => h("div", slots.default?.());
  },
});

function mountPanelHost(props: InstanceType<typeof PanelHost>["$props"]) {
  return mount(PanelHost, {
    props,
  });
}

describe("PanelHost", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    lifecycle.mounts = 0;
    lifecycle.unmounts = 0;
  });

  it("renders a graceful fallback for an unknown panel type", () => {
    const w = mountPanelHost({ panel: "totally-made-up", connectionId: "c1" });
    expect(w.text()).toContain("No renderer for panel type");
    expect(w.text()).toContain("totally-made-up");
  });

  it("renders a panel error for malformed generic config", () => {
    const w = mount(SchemaProvider, {
      slots: {
        default: () =>
          h(PanelHost, {
            panel: "table",
            connectionId: "c1",
            config: { columns: "bad" },
          }),
      },
    });

    expect(w.text()).toContain("config.columns must be an array.");
  });

  it("uses the shared panel loader for async panel loading", () => {
    const w = mount(PanelLoader);
    expect(w.find('[data-test="panel-loader"]').exists()).toBe(true);
    expect(w.text()).not.toContain("Loading panel");
  });

  it("validates dashboard cell config recursively", () => {
    const w = mount(SchemaProvider, {
      slots: {
        default: () =>
          h(PanelHost, {
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
          }),
      },
    });

    expect(w.text()).toContain(
      "config.cells[0].config.saveMethod must be one of POST, PUT, PATCH, DELETE.",
    );
  });

  it("passes runtime recording metadata outside plugin config validation", () => {
    const w = mount(SchemaProvider, {
      slots: {
        default: () =>
          h(PanelHost, {
            panel: "test_panel",
            connectionId: "c1",
            config: { zoom: true },
            recording: {
              class: "terminal",
              policy: "manual",
              authoritative: true,
            },
          }),
      },
    });

    expect(w.text()).toContain("manual");
    expect(w.text()).not.toContain("not supported");
  });

  it("resolves recording metadata from the panel host context", () => {
    const w = mount(SchemaProvider, {
      props: { recordingRoute: "ssh.shell" },
      slots: {
        default: () =>
          h(PanelHost, {
            panel: "test_panel",
            connectionId: "c1",
            source: { routeId: "ssh.shell" },
            config: { zoom: true },
          }),
      },
    });

    expect(w.text()).toContain("manual");
    expect(w.text()).not.toContain("not supported");
  });

  it("rejects runtime metadata when it leaks into plugin config", () => {
    const w = mount(SchemaProvider, {
      slots: {
        default: () =>
          h(PanelHost, {
            panel: "test_panel",
            connectionId: "c1",
            config: {
              zoom: true,
              _recording: {
                class: "terminal",
                policy: "manual",
                authoritative: true,
              },
            },
          }),
      },
    });

    expect(w.text()).toContain("config._recording is not supported.");
  });

  it("remounts panels when the connection changes", async () => {
    const w = mountPanelHost({
      panel: "test_panel",
      connectionId: "c1",
      source: { routeId: "postgresql.query", method: "WS" },
      resource: { kind: "table", name: "users", uid: "public.users" },
    });

    expect(lifecycle.mounts).toBe(1);
    expect(w.text()).toContain("panel c1");

    await w.setProps({ connectionId: "c2" });

    expect(lifecycle.unmounts).toBe(1);
    expect(lifecycle.mounts).toBe(2);
    expect(w.text()).toContain("panel c2");
  });

  it("remounts panels when connection scope changes", async () => {
    const scope = useScopeStore();
    scope.configure("c1", [{ param: "database" }]);
    mountPanelHost({
      panel: "test_panel",
      connectionId: "c1",
      source: { routeId: "redis.keys.list" },
    });

    expect(lifecycle.mounts).toBe(1);
    scope.set("c1", "database", "1");
    await nextTick();

    expect(lifecycle.unmounts).toBe(1);
    expect(lifecycle.mounts).toBe(2);
  });
});
