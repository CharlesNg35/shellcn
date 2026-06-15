import { describe, expect, it } from "vitest";
import { mount } from "@vue/test-utils";
import Checkbox from "primevue/checkbox";
import ToggleSwitch from "primevue/toggleswitch";
import PrimeVue from "primevue/config";
import Tabs from "primevue/tabs";
import TabList from "primevue/tablist";
import Tab from "primevue/tab";
import { primeVuePassthrough } from "./preset";

describe("primeVuePassthrough", () => {
  it("styles checked toggles using the state emitted on the slider", () => {
    const wrapper = mount(ToggleSwitch, { props: { modelValue: true } });

    expect(
      wrapper.find('input[role="switch"]').attributes("checked"),
    ).toBeDefined();
    expect(
      wrapper.find('[data-pc-section="slider"]').attributes("data-p"),
    ).toBe("checked");
    expect(wrapper.find('[data-pc-section="slider"]').classes()).toContain(
      "data-[p~=checked]:bg-primary-500",
    );
  });

  it("styles checked checkbox boxes using the state emitted on the box", () => {
    const wrapper = mount(Checkbox, {
      props: { binary: true, modelValue: true },
    });

    expect(
      wrapper.find('input[type="checkbox"]').attributes("checked"),
    ).toBeDefined();
    expect(wrapper.find('[data-pc-section="box"]').attributes("data-p")).toBe(
      "checked",
    );
    expect(wrapper.find('[data-pc-section="box"]').classes()).toContain(
      "data-[p~=checked]:bg-primary-500",
    );
  });

  it("defines baseline styles for common PrimeVue components", () => {
    for (const key of [
      "badge",
      "card",
      "datepicker",
      "divider",
      "drawer",
      "message",
      "panel",
      "progressspinner",
      "radiobutton",
      "skeleton",
      "slider",
      "tag",
      "toast",
      "toolbar",
    ]) {
      expect(primeVuePassthrough).toHaveProperty(key);
    }
    expect(primeVuePassthrough.directives).toHaveProperty("tooltip");
  });

  it("keeps tab navigation visible while tab panels scroll", () => {
    expect(primeVuePassthrough.tabs.root).toContain("min-w-0");
    expect(primeVuePassthrough.tablist.root).toContain("sticky");
    expect(primeVuePassthrough.tablist.root).toContain("top-0");
    expect(primeVuePassthrough.tablist.root).toContain("overflow-hidden");
    expect(primeVuePassthrough.tablist.root).toContain("backdrop-blur");
    expect(primeVuePassthrough.tablist.content).toContain("overflow-x-auto");
    expect(primeVuePassthrough.tablist.content).toContain(
      "[scrollbar-width:none]",
    );
    expect(primeVuePassthrough.tablist.tabList).toContain("flex-nowrap");
    expect(primeVuePassthrough.tablist.tabList).toContain("pl-1");
    expect(primeVuePassthrough.tablist.tabList).toContain("after:w-8");
    expect(primeVuePassthrough.tablist.tabList).not.toContain("pr-9");
    expect(primeVuePassthrough.tab.root).toContain("shrink-0");
    expect(primeVuePassthrough.tab.root).toContain("whitespace-nowrap");
    expect(primeVuePassthrough.tablist.prevButton).toContain("absolute");
    expect(primeVuePassthrough.tablist.prevButton).toContain("left-0");
    expect(primeVuePassthrough.tablist.nextButton).toContain("right-0");
  });

  it("applies tablist pass-through classes to PrimeVue scroll navigators", () => {
    const wrapper = mount(
      {
        components: { Tabs, TabList, Tab },
        template: `
          <Tabs value="0" scrollable>
            <TabList>
              <Tab v-for="i in 12" :key="i" :value="String(i - 1)">Tab {{ i }}</Tab>
            </TabList>
          </Tabs>
        `,
      },
      {
        global: {
          plugins: [[PrimeVue, { unstyled: true, pt: primeVuePassthrough }]],
        },
      },
    );

    const root = wrapper.get('[data-pc-name="tablist"]');
    expect(root.classes()).toContain("overflow-hidden");
    expect(wrapper.get('[data-pc-section="content"]').classes()).toContain(
      "overflow-x-auto",
    );
    expect(wrapper.get('[data-pc-section="tablist"]').classes()).toContain(
      "flex-nowrap",
    );
    wrapper.unmount();
  });

  it("keeps modal chrome bounded and scrollable", () => {
    expect(primeVuePassthrough.dialog.mask).toContain("pointer-events-auto");
    expect(primeVuePassthrough.dialog.mask).toContain("fixed");
    expect(primeVuePassthrough.dialog.mask).toContain("z-50");
    expect(primeVuePassthrough.confirmdialog.mask).toContain(
      "pointer-events-auto",
    );
    expect(primeVuePassthrough.dialog.root).toContain(
      "max-h-[calc(100vh-2rem)]",
    );
    expect(primeVuePassthrough.dialog.root).toContain("flex-col");
    expect(primeVuePassthrough.dialog.header).toContain("shrink-0");
    expect(primeVuePassthrough.dialog.footer).toContain("shrink-0");
    expect(primeVuePassthrough.dialog.content).toContain("min-h-0");
    expect(primeVuePassthrough.dialog.content).toContain("overflow-auto");
    expect(primeVuePassthrough.dialog.pcMaximizeButton.root).toContain(
      "rounded-md",
    );
    expect(primeVuePassthrough.dialog.pcCloseButton.root).toContain(
      "rounded-md",
    );
    expect(primeVuePassthrough.confirmdialog.content).toContain(
      "overflow-auto",
    );
  });

  it("styles drawers in unstyled mode", () => {
    expect(primeVuePassthrough.drawer.mask).toContain("pointer-events-auto");
    expect(primeVuePassthrough.drawer.mask).toContain("z-50");
    expect(primeVuePassthrough.drawer.root).toContain("fixed");
    expect(primeVuePassthrough.drawer.root).toContain("bg-surface-0");
    expect(primeVuePassthrough.drawer.content).toContain("min-h-0");
    expect(primeVuePassthrough.drawer.pcCloseButton.root).toContain(
      "rounded-full",
    );
  });

  it("keeps data tables horizontally scrollable without stretching cells", () => {
    expect(primeVuePassthrough.datatable.tableContainer).toContain(
      "overflow-auto",
    );
    expect(primeVuePassthrough.datatable.tableContainer).toContain(
      "thin-scrollbar",
    );
    expect(primeVuePassthrough.datatable.table).toContain("w-max");
    expect(primeVuePassthrough.datatable.table).toContain("min-w-full");
  });

  it("allows constrained buttons to truncate their labels", () => {
    const root = primeVuePassthrough.button.root({
      props: {},
    } as Parameters<typeof primeVuePassthrough.button.root>[0]);

    expect(root).toContain("min-w-0");
    expect(primeVuePassthrough.button.label).toContain("min-w-0");
    expect(primeVuePassthrough.button.label).toContain("truncate");
  });

  it("styles PrimeVue tooltip directive overlays", () => {
    expect(primeVuePassthrough.directives.tooltip.root).toContain("absolute");
    expect(primeVuePassthrough.directives.tooltip.root).toContain("z-[1100]");
    expect(primeVuePassthrough.directives.tooltip.text).toContain(
      "bg-surface-950",
    );
    expect(primeVuePassthrough.directives.tooltip.text).toContain("text-xs");
    expect(primeVuePassthrough.directives.tooltip.arrow).toContain("rotate-45");
  });

  it("styles autocomplete token mode before it is used by scope controls", () => {
    expect(primeVuePassthrough.autocomplete.inputMultiple).toContain("min-h-9");
    expect(primeVuePassthrough.autocomplete.inputMultiple).toContain(
      "focus-within:ring-2",
    );
    expect(primeVuePassthrough.autocomplete.inputChip).toContain("flex-1");
    expect(primeVuePassthrough.autocomplete.pcChip.root).toContain(
      "inline-flex",
    );
    expect(primeVuePassthrough.autocomplete.pcChip.label).toContain("truncate");
  });

  it("merges conflicting button classes so size props take effect", () => {
    const small = primeVuePassthrough.button.root({
      props: { size: "small" },
    } as Parameters<typeof primeVuePassthrough.button.root>[0]);
    expect(small).toContain("text-xs");
    expect(small).toContain("px-2.5");
    expect(small).toContain("py-1");
    expect(small).not.toContain("text-sm");
    expect(small).not.toContain("px-3");
    expect(small).not.toContain("py-1.5");

    const large = primeVuePassthrough.button.root({
      props: { size: "large" },
    } as Parameters<typeof primeVuePassthrough.button.root>[0]);
    expect(large).toContain("text-base");
    expect(large).toContain("px-4");
    expect(large).toContain("py-2");
    expect(large).not.toContain("text-sm");

    const rounded = primeVuePassthrough.button.root({
      props: { rounded: true },
    } as Parameters<typeof primeVuePassthrough.button.root>[0]);
    expect(rounded).toContain("rounded-full");
    expect(rounded).not.toContain("rounded-md");
  });
});
