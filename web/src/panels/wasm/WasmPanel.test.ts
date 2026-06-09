import { mount, flushPromises } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import WasmPanel from "./WasmPanel.vue";

describe("WasmPanel", () => {
  it("keeps the iframe opaque-origin sandboxed and enables scroll mode inside the app", async () => {
    const w = mount(WasmPanel, {
      props: {
        connectionId: "c1",
        config: {
          entry: "app.wasm",
          scaleMode: "scroll",
          capabilities: { fullscreen: true, gamepad: true },
          assets: [
            {
              path: "app.wasm",
              source: { routeId: "wasm.asset" },
            },
          ],
        },
      },
    });
    await flushPromises();

    const iframe = w.get("iframe");
    expect(iframe.attributes("sandbox")).toBe("allow-scripts allow-fullscreen");
    expect(iframe.attributes("sandbox")).not.toContain("allow-same-origin");
    expect(iframe.attributes("allow")).toBe("fullscreen; gamepad");
    expect(iframe.attributes("srcdoc")).toContain("overflow:auto");
  });
});
