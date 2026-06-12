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
  updateWasmPanelRect,
  wasmStageEntries,
  wasmStageFrameBoxStyle,
  wasmStageFrameStyle,
  wasmStageViewportClass,
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

  it("maps wasm scale modes to stable iframe layout styles", async () => {
    registerWasmPanel({
      key: "resize",
      connectionId: "resize",
      config: {
        entry: "app.wasm",
        scaleMode: "resize",
        assets: [{ path: "app.wasm", source: { routeId: "wasm.asset" } }],
      },
    });
    registerWasmPanel({
      key: "scroll-fluid",
      connectionId: "scroll-fluid",
      config: {
        entry: "app.wasm",
        scaleMode: "scroll",
        assets: [{ path: "app.wasm", source: { routeId: "wasm.asset" } }],
      },
    });
    registerWasmPanel({
      key: "scroll-fixed",
      connectionId: "scroll-fixed",
      config: {
        entry: "app.wasm",
        scaleMode: "scroll",
        width: 1200,
        height: 900,
        assets: [{ path: "app.wasm", source: { routeId: "wasm.asset" } }],
      },
    });
    registerWasmPanel({
      key: "fit",
      connectionId: "fit",
      config: {
        entry: "app.wasm",
        scaleMode: "fit",
        width: 1200,
        height: 800,
        assets: [{ path: "app.wasm", source: { routeId: "wasm.asset" } }],
      },
    });
    updateWasmPanelRect("fit", {
      top: 0,
      left: 0,
      width: 600,
      height: 500,
    });
    await flushPromises();

    const byKey = new Map(
      wasmStageEntries.value.map((entry) => [entry.key, entry]),
    );
    const resize = byKey.get("resize");
    const scrollFluid = byKey.get("scroll-fluid");
    const scrollFixed = byKey.get("scroll-fixed");
    const fit = byKey.get("fit");
    expect(resize).toBeTruthy();
    expect(scrollFluid).toBeTruthy();
    expect(scrollFixed).toBeTruthy();
    expect(fit).toBeTruthy();

    expect(wasmStageViewportClass(resize!.config)).toBe("overflow-hidden");
    expect(wasmStageFrameBoxStyle(resize!)).toEqual({
      width: "100%",
      height: "100%",
    });
    expect(wasmStageFrameStyle(resize!)).toEqual({
      width: "100%",
      height: "100%",
    });

    expect(wasmStageViewportClass(scrollFluid!.config)).toBe(
      "overflow-auto overscroll-contain",
    );
    expect(wasmStageFrameBoxStyle(scrollFluid!)).toEqual({
      width: "100%",
      height: "100%",
    });
    expect(wasmStageFrameStyle(scrollFluid!)).toEqual({
      width: "100%",
      height: "100%",
    });

    expect(wasmStageFrameBoxStyle(scrollFixed!)).toEqual({
      width: "1200px",
      height: "900px",
    });
    expect(wasmStageFrameStyle(scrollFixed!)).toEqual({
      width: "100%",
      height: "100%",
    });

    expect(wasmStageViewportClass(fit!.config)).toBe(
      "grid place-items-center overflow-hidden",
    );
    expect(wasmStageFrameBoxStyle(fit!)).toEqual({
      position: "relative",
      width: "600px",
      height: "400px",
      flex: "0 0 auto",
    });
    expect(wasmStageFrameStyle(fit!)).toEqual({
      width: "1200px",
      height: "800px",
      transform: "scale(0.5)",
      transformOrigin: "top left",
    });
  });
});
