import { describe, it, expect, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { installFetch } from "../../test/fetchMock";
import DocumentPanel from "./DocumentPanel.vue";

vi.mock("../../codemirror", () => ({
  createCodeMirrorEditor: () => ({ view: { destroy() {} } }),
  setEditorValue: () => {},
  setEditorLanguage: () => {},
  setEditorReadOnly: () => {},
  syncCodeMirrorTheme: () => {},
}));

afterEach(() => vi.unstubAllGlobals());

describe("DocumentPanel", () => {
  it("renders fetched JSON as an expandable tree and can switch to raw mode", async () => {
    installFetch(() => ({ body: { State: { Status: "running" } } }));

    const w = mount(DocumentPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "docker.container.inspect" },
      },
    });
    await flushPromises();

    expect(w.text()).toContain("State");
    expect(w.text()).toContain("Status");

    await w
      .findAll("button")
      .find((b) => b.text() === "Raw")!
      .trigger("click");
    await flushPromises();
    expect(w.find(".shellcn-codemirror-host").exists()).toBe(true);
  });

  it("keeps the current document visible during refresh", async () => {
    let calls = 0;
    let resolveRefresh: ((value: Response) => void) | undefined;
    vi.stubGlobal(
      "fetch",
      vi.fn(() => {
        calls += 1;
        if (calls === 1) {
          return Promise.resolve(
            new Response(JSON.stringify({ State: { Status: "running" } }), {
              headers: { "Content-Type": "application/json" },
            }),
          );
        }
        return new Promise((resolve) => {
          resolveRefresh = resolve;
        });
      }),
    );

    const w = mount(DocumentPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "docker.container.inspect" },
      },
    });
    await flushPromises();

    await w
      .findAll("button")
      .find((button) => button.text().includes("Refresh"))!
      .trigger("click");
    await flushPromises();

    expect(w.find('[data-test="skeleton-list"]').exists()).toBe(false);
    expect(w.text()).toContain("running");

    resolveRefresh?.(
      new Response(JSON.stringify({ State: { Status: "healthy" } }), {
        headers: { "Content-Type": "application/json" },
      }),
    );
    await flushPromises();

    expect(w.text()).toContain("healthy");
  });
});
