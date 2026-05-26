import { describe, it, expect, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { installFetch } from "../../test/fetchMock";
import DocumentPanel from "./DocumentPanel.vue";

vi.mock("monaco-editor/min/vs/editor/editor.main.css", () => ({}));
vi.mock("monaco-editor/esm/vs/editor/editor.worker?worker", () => ({
  default: class {},
}));
vi.mock("monaco-editor/esm/vs/language/json/json.worker?worker", () => ({
  default: class {},
}));
vi.mock("monaco-editor/esm/vs/language/css/css.worker?worker", () => ({
  default: class {},
}));
vi.mock("monaco-editor/esm/vs/language/html/html.worker?worker", () => ({
  default: class {},
}));
vi.mock("monaco-editor/esm/vs/language/typescript/ts.worker?worker", () => ({
  default: class {},
}));
vi.mock("monaco-editor", () => ({
  editor: {
    create: () => ({
      getValue: () => "",
      setValue() {},
      getModel: () => ({}),
      onDidChangeModelContent() {},
      updateOptions() {},
      dispose() {},
    }),
    defineTheme() {},
    setTheme() {},
    setModelLanguage() {},
  },
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
    expect(w.find(".shellcn-monaco-host").exists()).toBe(true);
  });
});
