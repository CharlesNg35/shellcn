import { mount, flushPromises } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import { installFetch } from "../../test/fetchMock";
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
    expect(w.find('[data-test="panel-loader"]').exists()).toBe(true);
    expect(w.text()).not.toContain("Loading WebAssembly panel");
    await flushPromises();

    const iframe = w.get("iframe");
    expect(iframe.attributes("sandbox")).toBe("allow-scripts");
    expect(iframe.attributes("sandbox")).not.toContain("allow-fullscreen");
    expect(iframe.attributes("sandbox")).not.toContain("allow-same-origin");
    expect(iframe.attributes("allow")).toBe("fullscreen; gamepad");
    expect(iframe.attributes("srcdoc")).toContain("overflow:auto");
    expect(iframe.attributes("srcdoc")).toContain("script-src");
    expect(iframe.attributes("srcdoc")).toContain("blob:");
    expect(iframe.attributes("srcdoc")).toContain("worker-src blob:");
    expect(iframe.attributes("srcdoc")).toContain('entry: "app.wasm"');
    expect(iframe.attributes("srcdoc")).toContain("theme:");
    expect(iframe.attributes("srcdoc")).toContain("colors:");
    expect(iframe.attributes("srcdoc")).toContain("onTheme(fn)");
    expect(iframe.attributes("srcdoc")).toContain(
      "fn(msg.theme, window.shellcn.colors)",
    );
    expect(iframe.attributes("srcdoc")).toContain('msg.type === "theme"');
    expect(iframe.attributes("srcdoc")).toContain(
      'window.shellcn.asset("app.wasm")',
    );
  });

  it("lets generic boot scripts own startup for framework WASM", async () => {
    installFetch(() => ({
      body: "window.shellcn.asset(window.shellcn.entry);",
    }));
    const w = mount(WasmPanel, {
      props: {
        connectionId: "c1",
        config: {
          entry: "app_bg.wasm",
          runtime: "generic",
          boot: { scripts: ["boot.js"] },
          assets: [
            { path: "app_bg.wasm", source: { routeId: "wasm.asset" } },
            { path: "boot.js", source: { routeId: "wasm.asset" } },
          ],
        },
      },
    });
    await flushPromises();

    const srcdoc = w.get("iframe").attributes("srcdoc");
    expect(srcdoc).toContain('entry: "app_bg.wasm"');
    expect(srcdoc).toContain("window.shellcn.asset(window.shellcn.entry)");
    expect(srcdoc).toContain("if (true) return;");
  });
});
