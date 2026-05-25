import { describe, expect, it } from "vitest";
import { mount } from "@vue/test-utils";
import AppLogo from "./AppLogo.vue";

describe("AppLogo", () => {
  it("renders the ShellCN logo with primary-color inheritance", () => {
    const wrapper = mount(AppLogo, {
      props: { size: 40, label: "ShellCN" },
      attrs: { class: "text-primary-600" },
    });

    const svg = wrapper.find("svg");
    expect(svg.attributes("width")).toBe("40");
    expect(svg.attributes("height")).toBe("40");
    expect(svg.attributes("aria-label")).toBe("ShellCN");
    expect(wrapper.find("rect").attributes("fill")).toBe("currentColor");
  });
});
