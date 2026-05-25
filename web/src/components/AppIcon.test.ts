import { describe, it, expect } from "vitest";
import { mount } from "@vue/test-utils";
import AppIcon from "./AppIcon.vue";

describe("AppIcon", () => {
  it("renders a named Lucide icon as inline svg", () => {
    const w = mount(AppIcon, {
      props: { icon: { type: "name", value: "terminal" } },
    });
    expect(w.find("svg").exists()).toBe(true);
  });

  it("renders emoji as text", () => {
    const w = mount(AppIcon, {
      props: { icon: { type: "emoji", value: "🐳" } },
    });
    expect(w.text()).toContain("🐳");
  });

  it("renders an https url as an image", () => {
    const w = mount(AppIcon, {
      props: { icon: { type: "url", value: "https://x/i.svg" } },
    });
    expect(w.find("img").attributes("src")).toBe("https://x/i.svg");
  });

  it("renders a data: base64 image", () => {
    const w = mount(AppIcon, {
      props: {
        icon: { type: "base64", value: "data:image/svg+xml;base64,AAA" },
      },
    });
    expect(w.find("img").exists()).toBe(true);
  });

  it("falls back to a Lucide icon for an unsafe url or unknown name", () => {
    expect(
      mount(AppIcon, {
        props: { icon: { type: "url", value: "javascript:alert(1)" } },
      })
        .find("svg")
        .exists(),
    ).toBe(true);
    expect(
      mount(AppIcon, {
        props: { icon: { type: "name", value: "no-such-glyph" } },
      })
        .find("svg")
        .exists(),
    ).toBe(true);
  });

  it("renders sanitized inline svg markup", () => {
    const w = mount(AppIcon, {
      props: {
        icon: {
          type: "svg",
          value: '<svg viewBox="0 0 24 24"><circle r="8"/></svg>',
        },
      },
    });
    const html = w.html();
    expect(html).toContain("<svg");
    expect(html).toContain("circle");
  });

  it("strips scripts/handlers from inline svg (XSS guard)", () => {
    const w = mount(AppIcon, {
      props: {
        icon: {
          type: "svg",
          value:
            '<svg onload="alert(1)"><script>alert(2)</script><circle r="8"/></svg>',
        },
      },
    });
    const html = w.html();
    expect(html).not.toContain("onload");
    expect(html).not.toContain("<script");
    expect(html).not.toContain("alert");
  });

  it("renders nothing for empty svg markup", () => {
    const w = mount(AppIcon, { props: { icon: { type: "svg", value: "" } } });
    // empty value → no icon at all (treated as not provided)
    expect(w.find("svg").exists()).toBe(false);
  });

  it("renders nothing when no icon is provided", () => {
    const w = mount(AppIcon, { props: { icon: null } });
    expect(w.find("svg").exists()).toBe(false);
    expect(w.find("img").exists()).toBe(false);
  });
});
