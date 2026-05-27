import { describe, it, expect } from "vitest";
import { mount } from "@vue/test-utils";
import AppIcon from "./AppIcon.vue";
import { iconExists, toPascalCase } from "./lucideIconRegistry";
import type { Icon } from "../types/projection";

describe("lucideIconRegistry", () => {
  it("normalizes any separator/casing to Lucide PascalCase", () => {
    for (const input of [
      "ellipsis-vertical",
      "EllipsisVertical",
      "Ellipsis Vertical",
      "ellipsis_vertical",
      "ellipsisVertical",
    ]) {
      expect(toPascalCase(input)).toBe("EllipsisVertical");
    }
    expect(toPascalCase("trash-2")).toBe("Trash2");
  });

  it("knows real Lucide names and rejects unknown ones", () => {
    expect(iconExists("ellipsis-vertical")).toBe(true);
    expect(iconExists("Terminal")).toBe(true);
    expect(iconExists("no-such-glyph")).toBe(false);
  });
});

describe("AppIcon", () => {
  it("renders a Lucide icon as inline svg", () => {
    const w = mount(AppIcon, {
      props: { icon: { type: "lucide", value: "terminal" } },
    });
    expect(w.find("svg").exists()).toBe(true);
  });

  it("resolves any-cased name the same way", () => {
    for (const value of ["ellipsis-vertical", "EllipsisVertical"]) {
      const w = mount(AppIcon, { props: { icon: { type: "lucide", value } } });
      expect(w.find("svg").exists()).toBe(true);
    }
  });

  it("accepts legacy name icons from persisted projections", () => {
    const legacy = mount(AppIcon, {
      props: {
        icon: { type: "name", value: "terminal" } as unknown as Icon,
      },
    });
    const lucide = mount(AppIcon, {
      props: { icon: { type: "lucide", value: "terminal" } },
    });
    expect(legacy.html()).toBe(lucide.html());
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
        props: { icon: { type: "lucide", value: "no-such-glyph" } },
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

  it("falls back for empty svg markup", () => {
    const w = mount(AppIcon, { props: { icon: { type: "svg", value: "" } } });
    expect(w.find("svg").exists()).toBe(true);
  });

  it("falls back when no icon is provided", () => {
    const w = mount(AppIcon, { props: { icon: null } });
    expect(w.find("svg").exists()).toBe(true);
    expect(w.find("img").exists()).toBe(false);
  });

  it("uses the shared loading glyph when loading", () => {
    const w = mount(AppIcon, {
      props: { icon: { type: "emoji", value: "🐳" }, loading: true },
    });
    expect(w.text()).not.toContain("🐳");
    expect(w.find("svg").exists()).toBe(true);
    expect(w.find("svg").classes()).toContain("animate-spin");
  });
});
