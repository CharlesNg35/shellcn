/* eslint-disable vue/one-component-per-file */
import { defineComponent, reactive, ref } from "vue";
import { mount, flushPromises } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";

const route = reactive<{ query: Record<string, unknown> }>({
  query: { tab: "market" },
});
const routerReplace = vi.fn();
const protocolMocks = vi.hoisted(() => ({
  load: vi.fn(),
  setAvailability: vi.fn(),
  refreshPlugins: vi.fn(),
  builtIn: [] as Array<{
    name: string;
    title: string;
    availability: "enabled" | "admin_only" | "disabled";
    external?: boolean;
  }>,
}));

vi.mock("vue-router", () => ({
  useRoute: () => route,
  useRouter: () => ({ replace: routerReplace }),
}));

vi.mock("../composables/useProtocolsAdmin", () => ({
  useProtocolsAdmin: () => ({
    pluginsDir: ref("plugins.d"),
    loading: ref(false),
    saving: ref({}),
    builtIn: ref(protocolMocks.builtIn),
    external: ref([]),
    load: protocolMocks.load,
    setAvailability: protocolMocks.setAvailability,
  }),
}));

vi.mock("../composables/useMarketAdmin", () => ({
  useMarketAdmin: () => ({
    enabled: ref(true),
    entries: ref([]),
    loading: ref(false),
    installing: ref({}),
    uninstalling: ref({}),
    load: vi.fn(),
    install: vi.fn(),
    uninstall: vi.fn(),
  }),
}));

vi.mock("../composables/useConfirmAction", () => ({
  useConfirmAction: () => ({ confirmDanger: vi.fn() }),
}));

vi.mock("../stores/connections", () => ({
  useConnectionsStore: () => ({
    refreshPlugins: protocolMocks.refreshPlugins,
  }),
}));

const TabsStub = defineComponent({
  name: "TestTabs",
  props: { value: { type: String, default: "builtin" } },
  emits: ["update:value"],
  template: '<div data-test="tabs" :data-value="value"><slot /></div>',
});

import ProtocolsView from "./ProtocolsView.vue";

describe("ProtocolsView", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    route.query = { tab: "market" };
    routerReplace.mockClear();
    protocolMocks.load.mockReset();
    protocolMocks.setAvailability.mockReset();
    protocolMocks.refreshPlugins.mockReset();
    protocolMocks.setAvailability.mockResolvedValue(undefined);
    protocolMocks.refreshPlugins.mockResolvedValue(undefined);
    protocolMocks.builtIn = [];
  });

  it("selects the Marketplace tab from the route query", async () => {
    const wrapper = mount(ProtocolsView, {
      global: {
        stubs: {
          Tabs: TabsStub,
          TabList: { template: "<div><slot /></div>" },
          Tab: { template: "<button><slot /></button>" },
          TabPanels: { template: "<div><slot /></div>" },
          TabPanel: { template: "<section><slot /></section>" },
          ProtocolTable: { template: "<div />" },
          MarketTable: { template: "<div />" },
          AppBreadcrumb: { template: "<nav />" },
          AppIcon: { template: "<span />" },
        },
      },
    });
    await flushPromises();

    expect(wrapper.find('[data-test="tabs"]').attributes("data-value")).toBe(
      "market",
    );

    wrapper.findComponent(TabsStub).vm.$emit("update:value", "external");
    await flushPromises();

    expect(routerReplace).toHaveBeenCalledWith({
      query: { tab: "external" },
    });
  });

  it("refreshes the protocol catalog after an availability change", async () => {
    route.query = {};
    protocolMocks.builtIn = [
      {
        name: "ssh",
        title: "SSH",
        availability: "enabled",
      },
    ];

    const ProtocolTableStub = defineComponent({
      props: {
        protocols: { type: Array, required: true },
      },
      emits: ["set-availability"],
      template: `
        <button
          v-if="protocols.length"
          data-test="availability"
          @click="$emit('set-availability', protocols[0], 'disabled')"
        >
          Toggle
        </button>
      `,
    });

    const wrapper = mount(ProtocolsView, {
      global: {
        stubs: {
          Tabs: TabsStub,
          TabList: { template: "<div><slot /></div>" },
          Tab: { template: "<button><slot /></button>" },
          TabPanels: { template: "<div><slot /></div>" },
          TabPanel: { template: "<section><slot /></section>" },
          ProtocolTable: ProtocolTableStub,
          MarketTable: { template: "<div />" },
          AppBreadcrumb: { template: "<nav />" },
          AppIcon: { template: "<span />" },
        },
      },
    });
    await flushPromises();

    await wrapper.get('[data-test="availability"]').trigger("click");
    await flushPromises();

    expect(protocolMocks.setAvailability).toHaveBeenCalledWith(
      protocolMocks.builtIn[0],
      "disabled",
    );
    expect(protocolMocks.refreshPlugins).toHaveBeenCalledOnce();
  });
});
