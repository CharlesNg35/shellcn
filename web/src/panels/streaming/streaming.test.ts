import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { installFetch } from "../../test/fetchMock";

vi.mock("@xterm/xterm", () => ({
  Terminal: class {
    open() {}
    write() {}
    onData() {}
    dispose() {}
  },
}));
vi.mock("@xterm/xterm/css/xterm.css", () => ({}));
vi.mock("monaco-editor", () => ({
  editor: { create: () => ({ getValue: () => "", dispose() {} }) },
}));
vi.mock("@novnc/novnc", () => ({ default: class {} }));

class FakeWS {
  static instances: FakeWS[] = [];
  closed = false;
  readonly url: string;
  constructor(url: string) {
    this.url = url;
    FakeWS.instances.push(this);
  }
  send() {}
  close() {
    this.closed = true;
  }
  addEventListener() {}
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
  vi.stubGlobal("WebSocket", FakeWS);
  installFetch((url) => {
    if (url.includes("/tickets"))
      return { status: 201, body: { ticket: "t1" } };
    return { body: { content: "config: true", columns: [], rows: [] } };
  });
});
afterEach(() => vi.unstubAllGlobals());

const panels = [
  { name: "terminal", comp: TerminalPanel, banner: true },
  { name: "logs", comp: LogStreamPanel, banner: true },
  { name: "metrics", comp: MetricsPanel, banner: true },
  { name: "remote desktop", comp: RemoteDesktopPanel, banner: true },
  { name: "query editor", comp: QueryEditorPanel, banner: true },
  { name: "code editor", comp: CodeEditorPanel, banner: false },
];

describe("streaming stub panels", () => {
  for (const p of panels) {
    it(`${p.name} mounts and unmounts without throwing`, async () => {
      const w = mount(p.comp, { props });
      await flushPromises();
      if (p.banner) expect(w.text()).toContain("Stub panel");
      expect(() => w.unmount()).not.toThrow();
    });
  }

  it("reuses the open channel on remount (stream survives navigation away/back)", async () => {
    const first = mount(TerminalPanel, { props });
    await flushPromises();
    expect(FakeWS.instances).toHaveLength(1);
    first.unmount(); // navigate away — channel must persist

    const second = mount(TerminalPanel, { props });
    await flushPromises();
    expect(FakeWS.instances).toHaveLength(1); // no new socket — resumed
    second.unmount();
  });
});
