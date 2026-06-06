/* eslint-disable vue/one-component-per-file */
import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { defineComponent, nextTick, ref } from "vue";
import { mount, flushPromises } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import PrimeVue from "primevue/config";
import Dialog from "primevue/dialog";
import { installFetch } from "../../test/fetchMock";
import { primeVuePassthrough } from "../../primevue/preset";
import { useStreamChannelsStore } from "../../stores/streamChannels";

vi.mock("@xterm/xterm", () => ({
  Terminal: class {
    cols = 80;
    rows = 24;
    options = {};
    open() {}
    write() {}
    onData() {}
    loadAddon() {}
    focus() {}
    dispose() {}
    attachCustomKeyEventHandler() {}
  },
}));
vi.mock("@xterm/xterm/css/xterm.css", () => ({}));
vi.mock("@xterm/addon-fit", () => ({
  FitAddon: class {
    fit() {}
  },
}));
vi.mock("@xterm/addon-search", () => ({
  SearchAddon: class {
    findNext() {}
    findPrevious() {}
    clearDecorations() {}
    onDidChangeResults() {
      return { dispose() {} };
    }
  },
}));
vi.mock("@xterm/addon-web-links", () => ({ WebLinksAddon: class {} }));
vi.mock("@xterm/addon-webgl", () => ({
  WebglAddon: class {
    onContextLoss() {}
    dispose() {}
  },
}));
const mockCodeMirror = vi.hoisted(() => ({
  value: "",
  onChange: null as ((value: string) => void) | null,
  diffOptions: null as unknown,
}));
vi.mock("../../codemirror", () => ({
  createCodeMirrorEditor: (
    _parent: HTMLElement,
    options: { onChange?: (value: string) => void },
  ) => {
    mockCodeMirror.onChange = options.onChange ?? null;
    return { view: { destroy() {} } };
  },
  createCodeMirrorDiffView: (_parent: HTMLElement, options: unknown) => {
    mockCodeMirror.diffOptions = options;
    return { destroy() {}, syncTheme() {} };
  },
  editorValue: () => mockCodeMirror.value,
  setEditorValue: () => {},
  setEditorCompletions: () => {},
  setEditorLanguage: () => {},
  setEditorReadOnly: () => {},
  syncCodeMirrorTheme: () => {},
}));
vi.mock("@novnc/novnc", () => ({
  default: class {
    scaleViewport = false;
    clipViewport = false;
    resizeSession = false;
    background = "";
    addEventListener() {}
    disconnect() {}
  },
}));

class FakeResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}
vi.stubGlobal("ResizeObserver", FakeResizeObserver);

class FakeWS {
  static instances: FakeWS[] = [];
  closed = false;
  readonly url: string;
  private handlers: Record<string, ((ev: unknown) => void)[]> = {};
  constructor(url: string) {
    this.url = url;
    FakeWS.instances.push(this);
  }
  send() {}
  close() {
    this.closed = true;
  }
  addEventListener(type: string, fn: (ev: unknown) => void) {
    (this.handlers[type] ??= []).push(fn);
  }
  emit(type: string, ev?: unknown) {
    for (const fn of this.handlers[type] ?? []) fn(ev);
  }
}

import TerminalPanel from "./TerminalPanel.vue";
import LogStreamPanel from "./LogStreamPanel.vue";
import MetricsPanel from "./MetricsPanel.vue";
import CodeEditorPanel from "./CodeEditorPanel.vue";
import QueryEditorPanel from "./QueryEditorPanel.vue";
import RemoteDesktopPanel from "./RemoteDesktopPanel.vue";
import StreamStatusBar from "./StreamStatusBar.vue";
import TerminalGridPanel from "./TerminalGridPanel.vue";

const props = {
  connectionId: "c1",
  source: { routeId: "docker.container.exec", method: "WS" as const },
};

function streamSockets(): FakeWS[] {
  return FakeWS.instances.filter((ws) =>
    ws.url.includes("/api/connections/c1/x/docker.container.exec"),
  );
}

beforeEach(() => {
  setActivePinia(createPinia());
  FakeWS.instances = [];
  mockCodeMirror.value = "";
  mockCodeMirror.onChange = null;
  mockCodeMirror.diffOptions = null;
  vi.stubGlobal("ResizeObserver", FakeResizeObserver);
  vi.stubGlobal("WebSocket", FakeWS);
  installFetch((url) => {
    if (url.includes("/tickets"))
      return { status: 201, body: { ticket: "t1" } };
    return { body: { content: "config: true", columns: [], rows: [] } };
  });
});
afterEach(() => {
  useStreamChannelsStore().closeForConnection("c1");
  vi.unstubAllGlobals();
});

const panels = [
  { name: "terminal", comp: TerminalPanel, status: true },
  { name: "terminal grid", comp: TerminalGridPanel, status: true },
  { name: "logs", comp: LogStreamPanel, status: true },
  { name: "metrics", comp: MetricsPanel, status: true },
  { name: "remote desktop", comp: RemoteDesktopPanel, status: true },
  { name: "query editor", comp: QueryEditorPanel, status: true },
  { name: "code editor", comp: CodeEditorPanel, status: false },
];

describe("streaming stub panels", () => {
  for (const p of panels) {
    it(`${p.name} mounts and unmounts without throwing`, async () => {
      const w = mount(p.comp, { props });
      await flushPromises();
      expect(w.text()).not.toContain("Stub panel");
      if (p.status) expect(w.text()).toContain("Connecting");
      expect(() => w.unmount()).not.toThrow();
    });
  }

  it("reuses the open channel on remount (stream survives navigation away/back)", async () => {
    const pinia = createPinia();
    const first = mount(TerminalPanel, {
      props,
      global: { plugins: [pinia] },
    });
    await flushPromises();
    expect(streamSockets()).toHaveLength(1);
    streamSockets().at(-1)!.emit("open");
    first.unmount(); // navigate away — channel must persist

    const second = mount(TerminalPanel, {
      props,
      global: { plugins: [pinia] },
    });
    await flushPromises();
    expect(streamSockets()).toHaveLength(1); // no new socket — resumed
    second.unmount();
  });

  it("replaces a failed channel with a fresh ticket on remount", async () => {
    const first = mount(TerminalPanel, { props });
    await flushPromises();
    expect(streamSockets()).toHaveLength(1);
    streamSockets()[0].emit("error");
    first.unmount();

    const second = mount(TerminalPanel, { props });
    await flushPromises();
    expect(streamSockets()).toHaveLength(2);
    expect(streamSockets()[0].closed).toBe(true);
    second.unmount();
  });

  it("opens independent terminal channels for split panes on the same stream route", async () => {
    const w = mount(TerminalGridPanel, {
      props: { ...props, config: { maxPanes: 2, zoom: true, search: true } },
    });
    await flushPromises();
    expect(w.findAll("[data-terminal-grid-pane]")).toHaveLength(1);
    expect(streamSockets()).toHaveLength(1);

    await w
      .findAll("button")
      .find((button) => button.text().includes("Split right"))!
      .trigger("click");
    await flushPromises();

    expect(w.findAll("[data-terminal-grid-pane]")).toHaveLength(2);
    expect(streamSockets()).toHaveLength(2);
    expect(new Set(streamSockets().map((ws) => ws.url)).size).toBe(1);
    expect(
      w
        .findAll("button")
        .filter((button) =>
          button.attributes("aria-label")?.startsWith("Split active pane"),
        ),
    ).toHaveLength(2);

    await w
      .findAll("button")
      .find(
        (button) => button.attributes("aria-label") === "Close active pane",
      )!
      .trigger("click");
    await flushPromises();

    expect(w.findAll("[data-terminal-grid-pane]")).toHaveLength(1);
    expect(streamSockets().filter((ws) => ws.closed)).toHaveLength(1);
    w.unmount();
  });

  it("shows one terminal and disables split controls when terminal recording is mandatory", async () => {
    const w = mount(TerminalGridPanel, {
      props: {
        ...props,
        recording: {
          class: "terminal",
          policy: "auto",
          authoritative: true,
        },
      },
    });
    await flushPromises();

    expect(w.text()).not.toContain(
      "Split terminal workspaces are disabled when terminal recording is mandatory.",
    );
    expect(w.findAll("[data-terminal-grid-pane]")).toHaveLength(1);
    expect(w.text()).toContain("REC");
    expect(
      w
        .get('button[aria-label="Split active pane right"]')
        .attributes("disabled"),
    ).toBeDefined();
    expect(streamSockets()).toHaveLength(1);
    w.unmount();
  });

  it("auto split chooses the direction that makes panes closest to square", async () => {
    const w = mount(TerminalGridPanel, {
      props: { ...props, config: { maxPanes: 3 } },
    });
    await flushPromises();
    const pane = w.get("[data-terminal-grid-pane]").element as HTMLElement;
    vi.spyOn(pane, "getBoundingClientRect").mockReturnValue({
      width: 1200,
      height: 400,
      x: 0,
      y: 0,
      top: 0,
      left: 0,
      right: 1200,
      bottom: 400,
      toJSON: () => ({}),
    } as DOMRect);

    await w
      .findAll("button")
      .find(
        (button) =>
          button.attributes("aria-label") === "Auto split active pane",
      )!
      .trigger("click");
    await flushPromises();

    expect(
      w
        .get("[data-terminal-grid-split]")
        .attributes("data-terminal-grid-split"),
    ).toBe("horizontal");
    w.unmount();

    const tall = mount(TerminalGridPanel, {
      props: { ...props, config: { maxPanes: 3 } },
    });
    await flushPromises();
    const tallPane = tall.get("[data-terminal-grid-pane]")
      .element as HTMLElement;
    vi.spyOn(tallPane, "getBoundingClientRect").mockReturnValue({
      width: 400,
      height: 1200,
      x: 0,
      y: 0,
      top: 0,
      left: 0,
      right: 400,
      bottom: 1200,
      toJSON: () => ({}),
    } as DOMRect);

    await tall
      .findAll("button")
      .find(
        (button) =>
          button.attributes("aria-label") === "Auto split active pane",
      )!
      .trigger("click");
    await flushPromises();

    expect(
      tall
        .get("[data-terminal-grid-split]")
        .attributes("data-terminal-grid-split"),
    ).toBe("vertical");
    tall.unmount();
  });

  it("keeps repeated same-axis splits evenly distributed", async () => {
    const horizontal = mount(TerminalGridPanel, {
      props: { ...props, config: { maxPanes: 4 } },
    });
    await flushPromises();

    const splitRight = horizontal
      .findAll("button")
      .find(
        (button) =>
          button.attributes("aria-label") === "Split active pane right",
      )!;

    await splitRight.trigger("click");
    await flushPromises();
    await splitRight.trigger("click");
    await flushPromises();

    expect(horizontal.findAll("[data-terminal-grid-pane]")).toHaveLength(3);
    expect(
      horizontal
        .findAll("[data-terminal-grid-panel-size]")
        .map((panel) => panel.attributes("data-terminal-grid-panel-size")),
    ).toEqual(["33.3333", "33.3333", "33.3334"]);
    horizontal.unmount();

    const vertical = mount(TerminalGridPanel, {
      props: { ...props, config: { maxPanes: 4 } },
    });
    await flushPromises();

    const splitDown = vertical
      .findAll("button")
      .find(
        (button) =>
          button.attributes("aria-label") === "Split active pane down",
      )!;

    await splitDown.trigger("click");
    await flushPromises();
    await splitDown.trigger("click");
    await flushPromises();

    expect(vertical.findAll("[data-terminal-grid-pane]")).toHaveLength(3);
    expect(
      vertical
        .findAll("[data-terminal-grid-panel-size]")
        .map((panel) => panel.attributes("data-terminal-grid-panel-size")),
    ).toEqual(["33.3333", "33.3333", "33.3334"]);
    vertical.unmount();
  });

  it("preserves manually resized split sizes", async () => {
    const w = mount(TerminalGridPanel, {
      props: { ...props, config: { maxPanes: 2 } },
    });
    await flushPromises();

    await w
      .findAll("button")
      .find(
        (button) =>
          button.attributes("aria-label") === "Split active pane right",
      )!
      .trigger("click");
    await flushPromises();

    w.findComponent({ name: "Splitter" }).vm.$emit("resizeend", {
      originalEvent: new Event("mouseup"),
      sizes: [30, 70],
    });
    await flushPromises();

    expect(
      w
        .findAll("[data-terminal-grid-panel-size]")
        .map((panel) => panel.attributes("data-terminal-grid-panel-size")),
    ).toEqual(["30", "70"]);
    w.unmount();
  });

  it("keeps stream error details controls vertically centered", () => {
    const w = mount(StreamStatusBar, {
      props: { status: "disconnected", error: "connection closed" },
      global: {
        plugins: [[PrimeVue, { unstyled: true, pt: primeVuePassthrough }]],
      },
    });

    const details = w.get('button[aria-label="Show error details"]');
    expect(details.classes()).toContain("inline-flex");
    expect(details.classes()).toContain("items-center");
    expect(details.classes()).toContain("h-7");
    w.unmount();
  });

  it("disables split controls while a single terminal pane is recording", async () => {
    const w = mount(TerminalGridPanel, {
      props: {
        ...props,
        config: { maxPanes: 2 },
        recording: {
          class: "terminal",
          policy: "manual",
          authoritative: true,
        },
      },
    });
    await flushPromises();

    expect(
      w
        .get('button[aria-label="Split active pane right"]')
        .attributes("disabled"),
    ).toBeUndefined();

    await w
      .findAll("button")
      .find((button) => button.text().includes("Record"))!
      .trigger("click");
    await flushPromises();

    expect(w.text()).toContain("REC");
    expect(
      w
        .get('button[aria-label="Split active pane right"]')
        .attributes("disabled"),
    ).toBeDefined();
    w.unmount();
  });

  it("does not show the split recording notice when connection recording is disabled", async () => {
    const w = mount(TerminalGridPanel, {
      props: {
        ...props,
        config: { maxPanes: 2 },
        recording: {
          class: "terminal",
          policy: "disabled",
          authoritative: true,
        },
      },
    });
    await flushPromises();

    await w
      .findAll("button")
      .find((button) => button.text().includes("Split right"))!
      .trigger("click");
    await flushPromises();

    expect(w.findAll("[data-terminal-grid-pane]")).toHaveLength(2);
    expect(w.text()).not.toContain("Recording disabled for split view");
    expect(w.text()).not.toContain("Recording off");
    w.unmount();
  });

  it("disables terminal recording controls for multi-pane workspaces", async () => {
    const w = mount(TerminalGridPanel, {
      props: {
        ...props,
        config: { maxPanes: 2 },
        recording: {
          class: "terminal",
          policy: "manual",
          authoritative: true,
        },
      },
    });
    await flushPromises();

    await w
      .findAll("button")
      .find((button) => button.text().includes("Split right"))!
      .trigger("click");
    await flushPromises();

    expect(w.text()).not.toContain("Recording disabled for split view");
    expect(w.text()).not.toContain("Recording off");
    expect(w.text()).not.toContain("Start recording");
    const recordButtons = w
      .findAll("button")
      .filter((button) => button.text().includes("Record"));
    expect(recordButtons).toHaveLength(1);
    expect(recordButtons[0].attributes("disabled")).toBeDefined();
    w.unmount();
  });

  it("reconnects a failed stream from the status bar", async () => {
    const w = mount(TerminalPanel, { props });
    await flushPromises();
    expect(streamSockets()).toHaveLength(1);
    streamSockets()[0].emit("error");
    await flushPromises();

    await w
      .findAll("button")
      .find((button) => button.text().includes("Reconnect"))!
      .trigger("click");
    await flushPromises();

    expect(streamSockets()).toHaveLength(2);
    expect(streamSockets()[0].closed).toBe(true);
    w.unmount();
  });

  it("keeps log streams scrolled to bottom after KeepAlive reactivation", async () => {
    const original = Object.getOwnPropertyDescriptor(
      HTMLElement.prototype,
      "scrollHeight",
    );
    Object.defineProperty(HTMLElement.prototype, "scrollHeight", {
      configurable: true,
      get: () => 500,
    });
    const Host = defineComponent({
      components: { LogStreamPanel },
      setup() {
        const show = ref(true);
        return { show, props };
      },
      template:
        '<KeepAlive><LogStreamPanel v-if="show" v-bind="props" /></KeepAlive>',
    });
    const w = mount(Host);
    await flushPromises();

    FakeWS.instances[0].emit("message", { data: "first line" });
    await flushPromises();
    let viewport = w.get('[data-test="log-viewport"]').element as HTMLElement;
    expect(viewport.scrollTop).toBe(500);

    viewport.scrollTop = 0;
    (w.vm as unknown as { show: boolean }).show = false;
    await flushPromises();
    (w.vm as unknown as { show: boolean }).show = true;
    await flushPromises();

    viewport = w.get('[data-test="log-viewport"]').element as HTMLElement;
    expect(viewport.scrollTop).toBe(500);

    if (original) {
      Object.defineProperty(HTMLElement.prototype, "scrollHeight", original);
    } else {
      delete (HTMLElement.prototype as { scrollHeight?: number }).scrollHeight;
    }
    w.unmount();
  });

  it("does not force log scroll on reactivation when following is off", async () => {
    const original = Object.getOwnPropertyDescriptor(
      HTMLElement.prototype,
      "scrollHeight",
    );
    Object.defineProperty(HTMLElement.prototype, "scrollHeight", {
      configurable: true,
      get: () => 500,
    });
    const Host = defineComponent({
      components: { LogStreamPanel },
      setup() {
        const show = ref(true);
        return { show, props };
      },
      template:
        '<KeepAlive><LogStreamPanel v-if="show" v-bind="props" /></KeepAlive>',
    });
    const w = mount(Host);
    await flushPromises();
    FakeWS.instances[0].emit("message", { data: "first line" });
    await flushPromises();

    await w
      .findAll("button")
      .find((button) => button.text().includes("Following"))!
      .trigger("click");
    const viewport = w.get('[data-test="log-viewport"]').element as HTMLElement;
    viewport.scrollTop = 0;

    (w.vm as unknown as { show: boolean }).show = false;
    await flushPromises();
    (w.vm as unknown as { show: boolean }).show = true;
    await flushPromises();

    expect(
      (w.get('[data-test="log-viewport"]').element as HTMLElement).scrollTop,
    ).toBe(0);

    if (original) {
      Object.defineProperty(HTMLElement.prototype, "scrollHeight", original);
    } else {
      delete (HTMLElement.prototype as { scrollHeight?: number }).scrollHeight;
    }
    w.unmount();
  });

  it("shows zoom and search controls only when the manifest enables them", async () => {
    const plain = mount(TerminalPanel, { props });
    await flushPromises();
    expect(plain.find('[aria-label="Search terminal"]').exists()).toBe(false);
    expect(plain.find('[aria-label="Zoom in"]').exists()).toBe(false);
    plain.unmount();

    const w = mount(TerminalPanel, {
      props: { ...props, config: { zoom: true, search: true } },
    });
    await flushPromises();
    expect(w.find('[aria-label="Zoom in"]').exists()).toBe(true);
    expect(w.find('[aria-label="Search terminal"]').exists()).toBe(true);

    await w.find('[aria-label="Search terminal"]').trigger("click");
    expect(w.find('[aria-label="Find in terminal"]').exists()).toBe(true);
    w.unmount();
  });

  it("uses CodeMirror instead of the textarea fallback when the loader succeeds", async () => {
    const w = mount(QueryEditorPanel, { props });
    await flushPromises();

    expect(w.find(".shellcn-codemirror-host").exists()).toBe(true);
    expect(w.find("textarea.resize-none").exists()).toBe(false);
    w.unmount();
  });

  it("shows a skeleton while the query editor engine is loading", async () => {
    const w = mount(QueryEditorPanel, { props });
    expect(w.find('[data-test="skeleton-list"]').exists()).toBe(true);

    await flushPromises();

    expect(w.find('[data-test="skeleton-list"]').exists()).toBe(false);
    expect(w.find(".shellcn-codemirror-host").isVisible()).toBe(true);
    w.unmount();
  });

  it("shows a loader while the terminal engine is loading", async () => {
    const w = mount(TerminalPanel, { props });
    expect(w.find('[data-test="panel-loader"]').exists()).toBe(true);

    await flushPromises();

    expect(w.find('[data-test="panel-loader"]').exists()).toBe(false);
    w.unmount();
  });

  it("keeps single terminal recording controls in the existing terminal header", async () => {
    const w = mount(TerminalPanel, {
      props: {
        ...props,
        recording: {
          class: "terminal",
          policy: "manual",
          authoritative: true,
        },
      },
    });
    await flushPromises();

    expect(w.find(".border-b").text()).toContain("Record");
    expect(w.find('[aria-label="Split active pane right"]').exists()).toBe(
      false,
    );
    w.unmount();
  });

  it("shows a loader while the remote desktop engine is connecting", () => {
    const w = mount(RemoteDesktopPanel, { props });
    expect(w.find('[data-test="panel-loader"]').exists()).toBe(true);
    expect(w.text()).not.toContain(
      "Remote desktop session is waiting for a stream route.",
    );
    w.unmount();
  });

  it("shows a skeleton while a code editor document is loading", async () => {
    let resolveFetch: () => void = () => {};
    vi.stubGlobal("ResizeObserver", FakeResizeObserver);
    vi.stubGlobal(
      "fetch",
      vi.fn(
        () =>
          new Promise<Response>((resolve) => {
            resolveFetch = () =>
              resolve(
                new Response(JSON.stringify("apiVersion: v1\nkind: Pod\n"), {
                  status: 200,
                  headers: { "Content-Type": "application/json" },
                }),
              );
          }),
      ),
    );

    const w = mount(CodeEditorPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "kubernetes.resource.yaml" },
        config: { language: "yaml" },
      },
    });
    await flushPromises();
    expect(w.find('[data-test="skeleton-list"]').exists()).toBe(true);

    resolveFetch();
    await flushPromises();
    await flushPromises();
    expect(w.find('[data-test="skeleton-list"]').exists()).toBe(false);
    expect(w.find(".shellcn-codemirror-host").exists()).toBe(true);
    w.unmount();
  });

  it("saves initial code editor content under a configured JSON body key", async () => {
    const calls: { url: string; method?: string; body: unknown }[] = [];
    vi.unstubAllGlobals();
    vi.stubGlobal("ResizeObserver", FakeResizeObserver);
    installFetch((url, init) => {
      calls.push({
        url,
        method: init?.method,
        body: init?.body ? JSON.parse(init.body as string) : undefined,
      });
      return { body: { ok: true } };
    });
    mockCodeMirror.value = '{"id":"ada","name":"Ada"}';

    const w = mount(CodeEditorPanel, {
      props: {
        connectionId: "c1",
        config: {
          language: "json",
          initialContent: '{\n  "id": "example"\n}',
          saveRouteId: "search.document.upsert",
          saveMethod: "POST",
          saveParams: { index: "people" },
          saveBodyKey: "document",
          saveExtra: { action: "upsert" },
        },
      },
    });
    await flushPromises();

    await w
      .findAll("button")
      .find((button) => button.text().includes("Save"))!
      .trigger("click");
    await flushPromises();

    expect(calls).toEqual([
      {
        url: expect.stringContaining("search.document.upsert"),
        method: "POST",
        body: {
          action: "upsert",
          document: { id: "ada", name: "Ada" },
        },
      },
    ]);
    w.unmount();
  });

  it("opens a code editor diff only after content changes", async () => {
    const w = mount(CodeEditorPanel, {
      props: {
        connectionId: "c1",
        config: {
          language: "yaml",
          initialContent: "apiVersion: v1\nkind: Pod\n",
          saveRouteId: "kubernetes.resource.apply",
          saveMethod: "POST",
        },
      },
    });
    await flushPromises();

    expect(w.findAll("button").some((button) => button.text() === "Diff")).toBe(
      false,
    );

    mockCodeMirror.value = "apiVersion: v1\nkind: Service\n";
    mockCodeMirror.onChange?.(mockCodeMirror.value);
    await nextTick();

    await w
      .findAll("button")
      .find((button) => button.text() === "Diff")!
      .trigger("click");
    await flushPromises();

    expect(mockCodeMirror.diffOptions).toMatchObject({
      original: "apiVersion: v1\nkind: Pod\n",
      modified: "apiVersion: v1\nkind: Service\n",
      language: "yaml",
      collapseUnchanged: true,
    });
    const dialog = w.findComponent(Dialog);
    const dialogPt = dialog.props("pt") as {
      root: string;
      content: string;
    };
    expect((dialog.vm.$attrs.style as { width?: string }).width).toBe("88vw");
    expect(dialog.props("breakpoints")).toMatchObject({
      "1199px": "94vw",
      "575px": "100vw",
    });
    expect(dialog.props("closeButtonProps")).toMatchObject({
      "aria-label": "Close diff review",
      title: "Close diff review",
    });
    expect(dialog.props("maximizeButtonProps")).toMatchObject({
      "aria-label": "Maximize or restore diff review",
      title: "Maximize or restore diff review",
    });
    expect(dialogPt.root).toContain("max-w-6xl");
    expect(dialogPt.content).toContain("overflow-hidden");
    expect(dialogPt.content).toContain("p-0");
    w.unmount();
  });

  it("truncates long query history chips and keeps the full query as title", async () => {
    const text =
      "SELECT * FROM public.github_app_installation_repositories LIMIT 100;";
    mockCodeMirror.value = text;
    const w = mount(QueryEditorPanel, { props });
    await flushPromises();

    await w
      .findAll("button")
      .find((button) => button.text().includes("Execute"))!
      .trigger("click");
    await flushPromises();

    const chip = w.get(`button[title="${text}"]`);
    expect(chip.classes()).toContain("max-w-72");
    expect(chip.classes()).toContain("overflow-hidden");
    expect(chip.text()).toBe(text);
    w.unmount();
  });

  it("keeps query result export at the beginning of the result toolbar", async () => {
    const w = mount(QueryEditorPanel, {
      props: { ...props, config: { exportable: true } },
    });
    await flushPromises();

    FakeWS.instances[0].emit("open");
    FakeWS.instances[0].emit("message", {
      data: JSON.stringify({
        columns: ["repository_with_a_long_column_name"],
        rows: [["shellcn"]],
        rowCount: 1,
      }),
    });
    await flushPromises();

    const toolbar = w.get('[data-test="query-result-toolbar"]');
    expect(toolbar.element.firstElementChild?.textContent).toContain("Export");
    expect(w.get('[data-test="query-export-button"]').classes()).not.toContain(
      "ml-auto",
    );
    w.unmount();
  });

  it("clears a previous query error after a successful result", async () => {
    const w = mount(QueryEditorPanel, { props });
    await flushPromises();

    FakeWS.instances[0].emit("open");
    FakeWS.instances[0].emit("message", {
      data: JSON.stringify({ error: "bad query" }),
    });
    await flushPromises();
    expect(w.text()).toContain("bad query");

    FakeWS.instances[0].emit("message", {
      data: JSON.stringify({ columns: ["ok"], rows: [[1]], rowCount: 1 }),
    });
    await flushPromises();
    expect(w.text()).not.toContain("bad query");
    expect(w.text()).toContain("1 row");
    w.unmount();
  });

  it("resets query editor state when the query context changes", async () => {
    const w = mount(QueryEditorPanel, {
      props: {
        ...props,
        source: {
          routeId: "postgresql.query",
          method: "WS" as const,
          params: { database: "a" },
        },
        config: { initialQuery: "select * from a;" },
        resource: { kind: "table", name: "a", uid: "a.public.t" },
      },
    });
    await flushPromises();

    FakeWS.instances[0].emit("open");
    FakeWS.instances[0].emit("message", {
      data: JSON.stringify({ error: "context a failed" }),
    });
    await flushPromises();
    expect(w.text()).toContain("context a failed");

    await w.setProps({
      source: {
        routeId: "postgresql.query",
        method: "WS" as const,
        params: { database: "b" },
      },
      config: { initialQuery: "select * from b;" },
      resource: { kind: "table", name: "b", uid: "b.public.t" },
    });
    await flushPromises();

    expect(w.text()).not.toContain("context a failed");
    w.unmount();
  });
});
