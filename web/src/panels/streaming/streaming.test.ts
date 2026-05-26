import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { installFetch } from "../../test/fetchMock";

const guacamoleTunnelUrls = vi.hoisted(() => [] as string[]);

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
  },
}));
vi.mock("@xterm/xterm/css/xterm.css", () => ({}));
vi.mock("@xterm/addon-fit", () => ({
  FitAddon: class {
    fit() {}
  },
}));
vi.mock("@xterm/addon-web-links", () => ({ WebLinksAddon: class {} }));
vi.mock("@xterm/addon-webgl", () => ({
  WebglAddon: class {
    onContextLoss() {}
    dispose() {}
  },
}));
vi.mock("monaco-editor", () => ({
  editor: {
    create: () => ({
      getValue: () => "",
      onDidChangeModelContent() {},
      dispose() {},
    }),
    defineTheme() {},
    setTheme() {},
  },
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
vi.mock("guacamole-common-js", () => {
  class FakeDisplay {
    element = document.createElement("div");
    onresize: null | (() => void) = null;
    getElement() {
      return this.element;
    }
    getWidth() {
      return 1024;
    }
    getHeight() {
      return 768;
    }
    scale() {}
  }
  class FakeClient {
    static State = {
      IDLE: 0,
      CONNECTING: 1,
      WAITING: 2,
      CONNECTED: 3,
      DISCONNECTING: 4,
      DISCONNECTED: 5,
    };
    display = new FakeDisplay();
    onstatechange: null | ((state: number) => void) = null;
    onerror = null;
    onrequired = null;
    constructor() {}
    getDisplay() {
      return this.display;
    }
    connect() {
      this.onstatechange?.(FakeClient.State.CONNECTED);
    }
    disconnect() {
      this.onstatechange?.(FakeClient.State.DISCONNECTED);
    }
    sendMouseState() {}
    sendKeyEvent() {}
    sendSize() {}
  }
  class FakeWebSocketTunnel {
    onerror = null;
    constructor(url: string) {
      guacamoleTunnelUrls.push(url);
    }
  }
  class FakeMouse {
    static Touchscreen = class {
      constructor() {}
      onEach() {}
    };
    constructor() {}
    onEach() {}
  }
  class FakeKeyboard {
    onkeydown: null | ((keysym: number) => boolean) = null;
    onkeyup: null | ((keysym: number) => void) = null;
    constructor() {}
    reset() {}
  }
  const Guacamole = {
    Client: FakeClient,
    Keyboard: FakeKeyboard,
    Mouse: FakeMouse,
    WebSocketTunnel: FakeWebSocketTunnel,
  };
  return { ...Guacamole, default: Guacamole };
});

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
  guacamoleTunnelUrls.length = 0;
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

  it("uses Monaco instead of the textarea fallback when the loader succeeds", async () => {
    const w = mount(QueryEditorPanel, { props });
    await flushPromises();

    expect(w.find(".shellcn-monaco-host").exists()).toBe(true);
    expect(w.find("textarea.resize-none").exists()).toBe(false);
    w.unmount();
  });

  it("renders the Guacamole engine through a lazy tunnel client", async () => {
    const w = mount(RemoteDesktopPanel, {
      props: {
        ...props,
        config: { engine: "guacamole" },
      },
    });
    await flushPromises();

    expect(w.text()).not.toContain("not available");
    await vi.waitFor(() => expect(guacamoleTunnelUrls).toHaveLength(1));
    expect(guacamoleTunnelUrls[0]).toContain("ticket=t1");
    expect(FakeWS.instances).toHaveLength(0);
    w.unmount();
  });
});
