import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { installFetch } from "../../test/fetchMock";

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
}));
vi.mock("../../codemirror", () => ({
  createCodeMirrorEditor: () => ({ view: { destroy() {} } }),
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

const props = {
  connectionId: "c1",
  source: { routeId: "docker.container.exec", method: "WS" as const },
};

beforeEach(() => {
  setActivePinia(createPinia());
  FakeWS.instances = [];
  mockCodeMirror.value = "";
  vi.stubGlobal("ResizeObserver", FakeResizeObserver);
  vi.stubGlobal("WebSocket", FakeWS);
  installFetch((url) => {
    if (url.includes("/tickets"))
      return { status: 201, body: { ticket: "t1" } };
    return { body: { content: "config: true", columns: [], rows: [] } };
  });
});
afterEach(() => vi.unstubAllGlobals());

const panels = [
  { name: "terminal", comp: TerminalPanel, status: true },
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
    const first = mount(TerminalPanel, { props });
    await flushPromises();
    expect(FakeWS.instances).toHaveLength(1);
    FakeWS.instances[0].emit("open");
    first.unmount(); // navigate away — channel must persist

    const second = mount(TerminalPanel, { props });
    await flushPromises();
    expect(FakeWS.instances).toHaveLength(1); // no new socket — resumed
    second.unmount();
  });

  it("replaces a failed channel with a fresh ticket on remount", async () => {
    const first = mount(TerminalPanel, { props });
    await flushPromises();
    expect(FakeWS.instances).toHaveLength(1);
    FakeWS.instances[0].emit("error");
    first.unmount();

    const second = mount(TerminalPanel, { props });
    await flushPromises();
    expect(FakeWS.instances).toHaveLength(2);
    expect(FakeWS.instances[0].closed).toBe(true);
    second.unmount();
  });

  it("reconnects a failed stream from the status bar", async () => {
    const w = mount(TerminalPanel, { props });
    await flushPromises();
    expect(FakeWS.instances).toHaveLength(1);
    FakeWS.instances[0].emit("error");
    await flushPromises();

    await w
      .findAll("button")
      .find((button) => button.text().includes("Reconnect"))!
      .trigger("click");
    await flushPromises();

    expect(FakeWS.instances).toHaveLength(2);
    expect(FakeWS.instances[0].closed).toBe(true);
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
