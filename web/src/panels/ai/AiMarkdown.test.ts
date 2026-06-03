import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import AiMarkdown from "./AiMarkdown.vue";

describe("AiMarkdown", () => {
  it("wraps markdown tables in a horizontal overflow container", () => {
    const wrapper = mount(AiMarkdown, {
      props: {
        source: [
          "| Point | Teaching | References |",
          "| --- | --- | --- |",
          "| A very long point | A very long teaching | https://example.com/a/really/long/reference |",
        ].join("\n"),
      },
    });

    const tableWrapper = wrapper.find(".ai-markdown-table");

    expect(tableWrapper.exists()).toBe(true);
    expect(tableWrapper.find("table").exists()).toBe(true);
  });

  it("renders raw html as text", () => {
    const wrapper = mount(AiMarkdown, {
      props: {
        source: "Hello <img src=x onerror=alert(1)>",
      },
    });

    expect(wrapper.html()).toContain("Hello");
    expect(wrapper.find("img").exists()).toBe(false);
  });
});
