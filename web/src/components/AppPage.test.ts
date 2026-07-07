import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import AppPage from "./AppPage.vue";

describe("AppPage", () => {
  it("uses the standard app page width", () => {
    const wrapper = mount(AppPage, {
      slots: { default: "Content" },
    });

    expect(wrapper.classes()).toEqual(expect.arrayContaining(["max-w-5xl"]));
    expect(wrapper.classes()).toEqual(expect.arrayContaining(["w-full"]));
    expect(wrapper.text()).toBe("Content");
  });

  it("can opt out of full-height layout", () => {
    const wrapper = mount(AppPage, {
      props: { fill: false },
    });

    expect(wrapper.classes()).not.toContain("h-full");
  });
});
