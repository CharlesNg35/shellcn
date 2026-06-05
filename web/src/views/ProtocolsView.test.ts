import { defineComponent, reactive, ref } from "vue";
import { mount, flushPromises } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";

const route = reactive<{ query: Record<string, unknown> }>({
  query: { tab: "market" },
});
const routerReplace = vi.fn();

vi.mock("vue-router", () => ({
  useRoute: () => route,
  useRouter: () => ({ replace: routerReplace }),
}));

vi.mock("../composables/useProtocolsAdmin", () => ({
  useProtocolsAdmin: () => ({
    pluginsDir: ref("plugins.d"),
    loading: ref(false),
    saving: ref({}),
    builtIn: ref([]),
    external: ref([]),
    load: vi.fn(),
    setAvailability: vi.fn(),
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
});
