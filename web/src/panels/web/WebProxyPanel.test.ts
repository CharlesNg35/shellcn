import { mount } from "@vue/test-utils";
import { defineComponent, h, KeepAlive, nextTick, ref } from "vue";
import { afterEach, describe, expect, it, vi } from "vitest";
import WebProxyPanel from "./WebProxyPanel.vue";
import WebProxyStage from "./WebProxyStage.vue";
import {
  disposeWebProxyConnection,
  disposeWebProxyStage,
  registerWebProxyPanel,
  webProxyStageEntries,
} from "./webProxyStage";

function mountPanel(config: Record<string, unknown> = {}) {
  return mount(WebProxyPanel, {
    attachTo: document.body,
    props: { connectionId: "conn/1", config },
    global: {
      stubs: {
        AppIcon: true,
      },
    },
  });
}

describe("WebProxyPanel", () => {
  afterEach(() => {
    disposeWebProxyStage();
    document.body.innerHTML = "";
  });

  it("builds iframe URLs from the connection proxy mount", async () => {
    mount(WebProxyStage);
    mountPanel({ path: "/workspace/?q=one#files" });
    await nextTick();

    const iframe = document.body.querySelector("iframe");
    expect(iframe?.getAttribute("src")).toBe(
      "/api/connections/conn%2F1/proxy/workspace/?q=one#files",
    );
    expect(iframe?.getAttribute("title")).toBe("Proxied web surface");
  });

  it("rejects paths outside the connection proxy mount", async () => {
    mount(WebProxyStage);
    const wrapper = mountPanel({ path: "//example.test/" });
    await nextTick();

    expect(document.body.querySelector("iframe")).toBeNull();
    expect(wrapper.text()).toContain("invalid proxy path");
  });

  it("uses a conservative sandbox unless capabilities opt in", async () => {
    mount(WebProxyStage);
    mountPanel();
    await nextTick();

    const iframe = document.body.querySelector("iframe");
    expect(iframe?.getAttribute("sandbox")).toContain("allow-scripts");
    expect(iframe?.getAttribute("sandbox")).not.toContain("allow-same-origin");
    expect(iframe?.getAttribute("allow")).toBeNull();
  });

  it("maps declared capabilities to iframe policies", async () => {
    mount(WebProxyStage);
    mountPanel({
      capabilities: [
        "clipboard",
        "downloads",
        "fullscreen",
        "popups",
        "same_origin",
      ],
      ariaLabel: "Workspace",
    });
    await nextTick();

    const iframe = document.body.querySelector("iframe");
    expect(iframe?.getAttribute("sandbox")).toContain("allow-downloads");
    expect(iframe?.getAttribute("sandbox")).toContain("allow-popups");
    expect(iframe?.getAttribute("sandbox")).toContain("allow-same-origin");
    expect(iframe?.getAttribute("allow")).toBe(
      "clipboard-read; clipboard-write; fullscreen",
    );
    expect(iframe?.getAttribute("title")).toBe("Workspace");
    expect(iframe?.getAttribute("referrerpolicy")).toBe("no-referrer");
    expect(iframe?.hasAttribute("allowfullscreen")).toBe(true);
  });

  it("opens the proxied URL in a new tab when enabled", async () => {
    mount(WebProxyStage);
    const open = vi.spyOn(window, "open").mockReturnValue(null);
    const wrapper = mountPanel({ path: "/app", openExternal: true });
    await nextTick();

    await wrapper.get("button[aria-label='Open in new tab']").trigger("click");

    expect(open).toHaveBeenCalledWith(
      "/api/connections/conn%2F1/proxy/app",
      "_blank",
      "noopener,noreferrer",
    );
    open.mockRestore();
  });

  it("keeps the same iframe mounted when KeepAlive deactivates the panel", async () => {
    const Harness = defineComponent({
      setup() {
        const show = ref(true);
        return { show };
      },
      render() {
        return h("div", [
          h(WebProxyStage),
          h(KeepAlive, () =>
            this.show
              ? h(WebProxyPanel, {
                  connectionId: "conn/1",
                  panelKey: "conn/1:web",
                  config: { path: "/workspace/" },
                })
              : null,
          ),
        ]);
      },
    });
    const wrapper = mount(Harness, { attachTo: document.body });
    await nextTick();

    const firstIframe = document.body.querySelector("iframe");
    expect(firstIframe).toBeTruthy();

    wrapper.vm.show = false;
    await nextTick();
    const hiddenIframe = document.body.querySelector("iframe");
    expect(hiddenIframe).toBe(firstIframe);
    expect(
      document.body.querySelector<HTMLElement>(
        '[data-test="web-proxy-stage-entry"]',
      )?.style.visibility,
    ).toBe("hidden");

    wrapper.vm.show = true;
    await nextTick();
    expect(document.body.querySelector("iframe")).toBe(firstIframe);
  });

  it("disposes only staged iframes for a disconnected connection", async () => {
    mount(WebProxyStage);
    registerWebProxyPanel({
      key: "c1:web",
      connectionId: "c1",
      config: { path: "/" },
    });
    registerWebProxyPanel({
      key: "c2:web",
      connectionId: "c2",
      config: { path: "/" },
    });
    await nextTick();

    disposeWebProxyConnection("c1");
    await nextTick();

    expect(webProxyStageEntries.value.map((entry) => entry.key)).toEqual([
      "c2:web",
    ]);
    expect(document.body.querySelectorAll("iframe")).toHaveLength(1);
  });
});
