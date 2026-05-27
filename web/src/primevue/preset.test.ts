import { describe, expect, it } from "vitest";
import { mount } from "@vue/test-utils";
import Checkbox from "primevue/checkbox";
import ToggleSwitch from "primevue/toggleswitch";
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
      "message",
      "panel",
      "radiobutton",
      "skeleton",
      "slider",
      "tag",
      "toast",
      "toolbar",
    ]) {
      expect(primeVuePassthrough).toHaveProperty(key);
    }
  });

  it("keeps modal chrome bounded and scrollable", () => {
    expect(primeVuePassthrough.dialog.root).toContain(
      "max-h-[calc(100vh-2rem)]",
    );
    expect(primeVuePassthrough.dialog.root).toContain("flex-col");
    expect(primeVuePassthrough.dialog.header).toContain("shrink-0");
    expect(primeVuePassthrough.dialog.footer).toContain("shrink-0");
    expect(primeVuePassthrough.dialog.content).toContain("min-h-0");
    expect(primeVuePassthrough.dialog.content).toContain("overflow-auto");
    expect(primeVuePassthrough.confirmdialog.content).toContain(
      "overflow-auto",
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
});
