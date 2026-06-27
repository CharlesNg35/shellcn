import { mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import WebProxyPanel from "./WebProxyPanel.vue";

function mountPanel(config: Record<string, unknown> = {}) {
  return mount(WebProxyPanel, {
    props: { connectionId: "conn/1", config },
    global: {
      stubs: {
        AppIcon: true,
      },
    },
  });
}

describe("WebProxyPanel", () => {
  it("builds iframe URLs from the connection proxy mount", () => {
    const wrapper = mountPanel({ path: "/workspace/?q=one#files" });

    const iframe = wrapper.get("iframe");
    expect(iframe.attributes("src")).toBe(
      "/api/connections/conn%2F1/proxy/workspace/?q=one#files",
    );
    expect(iframe.attributes("title")).toBe("Proxied web surface");
  });

  it("rejects paths outside the connection proxy mount", () => {
    const wrapper = mountPanel({ path: "//example.test/" });

    expect(wrapper.find("iframe").exists()).toBe(false);
    expect(wrapper.text()).toContain("invalid proxy path");
  });

  it("uses a conservative sandbox unless capabilities opt in", () => {
    const wrapper = mountPanel();

    const iframe = wrapper.get("iframe");
    expect(iframe.attributes("sandbox")).toContain("allow-scripts");
    expect(iframe.attributes("sandbox")).not.toContain("allow-same-origin");
    expect(iframe.attributes("allow")).toBeUndefined();
  });

  it("maps declared capabilities to iframe policies", () => {
    const wrapper = mountPanel({
      capabilities: [
        "clipboard",
        "downloads",
        "fullscreen",
        "popups",
        "same_origin",
      ],
      ariaLabel: "Workspace",
    });

    const iframe = wrapper.get("iframe");
    expect(iframe.attributes("sandbox")).toContain("allow-downloads");
    expect(iframe.attributes("sandbox")).toContain("allow-popups");
    expect(iframe.attributes("sandbox")).toContain("allow-same-origin");
    expect(iframe.attributes("allow")).toBe(
      "clipboard-read; clipboard-write; fullscreen",
    );
    expect(iframe.attributes("title")).toBe("Workspace");
    expect(iframe.attributes("referrerpolicy")).toBe("no-referrer");
    expect(iframe.attributes()).toHaveProperty("allowfullscreen");
  });

  it("opens the proxied URL in a new tab when enabled", async () => {
    const open = vi.spyOn(window, "open").mockReturnValue(null);
    const wrapper = mountPanel({ path: "/app", openExternal: true });

    await wrapper.get("button[aria-label='Open in new tab']").trigger("click");

    expect(open).toHaveBeenCalledWith(
      "/api/connections/conn%2F1/proxy/app",
      "_blank",
      "noopener,noreferrer",
    );
    open.mockRestore();
  });
});
