import { mount, flushPromises } from "@vue/test-utils";
import { defineComponent, h, KeepAlive, nextTick, ref } from "vue";
import { afterEach, describe, expect, it } from "vitest";
import { installFetch } from "@/test/fetchMock";
import WasmPanel from "./WasmPanel.vue";
import WasmStage from "./WasmStage.vue";
import {
  deactivateWasmPanel,
  disposeWasmStage,
  registerWasmPanel,
  wasmStageEntries,
} from "./wasmStage";

describe("WasmPanel", () => {
  afterEach(() => {
    disposeWasmStage();
    document.body.innerHTML = "";
  });

  it("renders the iframe in the persistent root stage with sandboxing intact", async () => {
    mount(WasmStage);
    mount(WasmPanel, {
      attachTo: document.body,
      props: {
        connectionId: "c1",
        panelKey: "c1:wasm",
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
    await nextTick();

    const iframe = document.body.querySelector("iframe");
    expect(iframe).toBeTruthy();
    expect(iframe?.getAttribute("sandbox")).toBe("allow-scripts");
    expect(iframe?.getAttribute("sandbox")).not.toContain("allow-fullscreen");
    expect(iframe?.getAttribute("sandbox")).not.toContain("allow-same-origin");
    expect(iframe?.getAttribute("allow")).toBe("fullscreen; gamepad");
    expect(iframe?.getAttribute("srcdoc")).toContain("overflow:auto");
    expect(iframe?.getAttribute("srcdoc")).toContain("script-src");
    expect(iframe?.getAttribute("srcdoc")).toContain("blob:");
    expect(iframe?.getAttribute("srcdoc")).toContain("worker-src blob:");
    expect(iframe?.getAttribute("srcdoc")).toContain('entry: "app.wasm"');
    expect(iframe?.getAttribute("srcdoc")).toContain("theme:");
    expect(iframe?.getAttribute("srcdoc")).toContain("colors:");
    expect(iframe?.getAttribute("srcdoc")).toContain("onTheme(fn)");
    expect(iframe?.getAttribute("srcdoc")).toContain("reportError(error)");
    expect(iframe?.getAttribute("srcdoc")).toContain("hideStatus()");
    expect(iframe?.getAttribute("srcdoc")).toContain(
      "const autoHideAfterAssets = false;",
    );
    expect(iframe?.getAttribute("srcdoc")).toContain(
      "fn(msg.theme, window.shellcn.colors)",
    );
    expect(iframe?.getAttribute("srcdoc")).toContain('msg.type === "theme"');
    expect(iframe?.getAttribute("srcdoc")).toContain(
      'window.shellcn.asset("app.wasm")',
    );
  });

  it("lets generic boot scripts own startup for framework WASM", async () => {
    installFetch(() => ({
      body: "window.shellcn.asset(window.shellcn.entry);",
    }));
    mount(WasmStage);
    mount(WasmPanel, {
      attachTo: document.body,
      props: {
        connectionId: "c2",
        panelKey: "c2:wasm",
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
    await nextTick();

    const srcdoc = document.body
      .querySelector("iframe")
      ?.getAttribute("srcdoc");
    expect(srcdoc).toContain('entry: "app_bg.wasm"');
    expect(srcdoc).toContain("window.shellcn.asset(window.shellcn.entry)");
    expect(srcdoc).toContain("if (true) return;");
    expect(srcdoc).toContain("const autoHideAfterAssets = true;");
  });

  it("keeps the same iframe mounted when KeepAlive deactivates the panel", async () => {
    const Harness = defineComponent({
      setup() {
        const show = ref(true);
        return { show };
      },
      render() {
        return h("div", [
          h(WasmStage),
          h(KeepAlive, () =>
            this.show
              ? h(WasmPanel, {
                  connectionId: "c3",
                  panelKey: "c3:wasm",
                  config: {
                    entry: "app.wasm",
                    assets: [
                      { path: "app.wasm", source: { routeId: "wasm.asset" } },
                    ],
                  },
                })
              : null,
          ),
        ]);
      },
    });
    const wrapper = mount(Harness, { attachTo: document.body });
    await flushPromises();
    await nextTick();

    const firstIframe = document.body.querySelector("iframe");
    expect(firstIframe).toBeTruthy();

    wrapper.vm.show = false;
    await nextTick();
    await flushPromises();
    const hiddenIframe = document.body.querySelector("iframe");
    expect(hiddenIframe).toBe(firstIframe);
    expect(
      document.body.querySelector<HTMLElement>('[data-test="wasm-stage-entry"]')
        ?.style.visibility,
    ).toBe("hidden");

    wrapper.vm.show = true;
    await nextTick();
    await flushPromises();
    expect(document.body.querySelector("iframe")).toBe(firstIframe);
  });

  it("evicts inactive staged iframes beyond the bounded LRU cap", async () => {
    mount(WasmStage);
    for (const key of ["one", "two", "three", "four"]) {
      registerWasmPanel({
        key,
        connectionId: key,
        config: {
          entry: "app.wasm",
          assets: [{ path: "app.wasm", source: { routeId: "wasm.asset" } }],
        },
      });
      deactivateWasmPanel(key);
    }
    await flushPromises();
    await nextTick();

    expect(wasmStageEntries.value.map((entry) => entry.key)).toEqual([
      "two",
      "three",
      "four",
    ]);
    expect(document.body.querySelectorAll("iframe")).toHaveLength(3);
  });
});
