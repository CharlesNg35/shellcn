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
import { useStream } from "../../composables/useStream";

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

class FakePath2D {
  constructor(path?: string) {
    if (path === "bad path") throw new Error("invalid path");
  }
  rect() {}
  roundRect() {}
  moveTo() {}
  lineTo() {}
  arc() {}
  closePath() {}
}

class FakeWS {
  static instances: FakeWS[] = [];
  readyState = 0;
  closed = false;
  sent: string[] = [];
  readonly url: string;
  private handlers: Record<string, ((ev: unknown) => void)[]> = {};
  constructor(url: string) {
    this.url = url;
    FakeWS.instances.push(this);
  }
  send(data: string) {
    this.sent.push(data);
  }
  close() {
    this.readyState = 3;
    this.closed = true;
  }
  addEventListener(type: string, fn: (ev: unknown) => void) {
    (this.handlers[type] ??= []).push(fn);
  }
  emit(type: string, ev?: unknown) {
    if (type === "open") this.readyState = 1;
    if (type === "error") this.readyState = 3;
    if (type === "close") this.readyState = 3;
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
import CanvasPanel from "./CanvasPanel.vue";

const props = {
  connectionId: "c1",
  source: { routeId: "docker.container.exec", method: "WS" as const },
};

function streamSockets(): FakeWS[] {
  return FakeWS.instances.filter((ws) =>
    ws.url.includes("/api/connections/c1/x/docker.container.exec"),
  );
}

let pinia: ReturnType<typeof createPinia>;
let canvasOps: string[];

beforeEach(() => {
  pinia = createPinia();
  setActivePinia(pinia);
  useStreamChannelsStore().closeForConnection("c1");
  FakeWS.instances = [];
  mockCodeMirror.value = "";
  mockCodeMirror.onChange = null;
  mockCodeMirror.diffOptions = null;
  canvasOps = [];
  const gradient = {
    addColorStop: (offset: number, color: string) =>
      canvasOps.push(`colorStop:${offset}:${color}`),
  };
  let textAlign: CanvasTextAlign = "start";
  const canvasContext = {
    setTransform: () => canvasOps.push("setTransform"),
    transform: () => canvasOps.push("transform"),
    clearRect: () => canvasOps.push("clearRect"),
    fillRect: () => canvasOps.push("fillRect"),
    beginPath: () => canvasOps.push("beginPath"),
    rect: () => canvasOps.push("rect"),
    roundRect: () => canvasOps.push("roundRect"),
    moveTo: () => canvasOps.push("moveTo"),
    lineTo: () => canvasOps.push("lineTo"),
    arc: () => canvasOps.push("arc"),
    ellipse: () => canvasOps.push("ellipse"),
    quadraticCurveTo: () => canvasOps.push("quadraticCurveTo"),
    bezierCurveTo: () => canvasOps.push("bezierCurveTo"),
    closePath: () => canvasOps.push("closePath"),
    clip: () => canvasOps.push("clip"),
    fill: () => canvasOps.push("fill"),
    stroke: () => canvasOps.push("stroke"),
    save: () => canvasOps.push("save"),
    restore: () => canvasOps.push("restore"),
    fillText: (text: string, x: number, y: number) =>
      canvasOps.push(`fillText:${text}:${x}:${y}`),
    strokeText: (text: string, x: number, y: number) =>
      canvasOps.push(`strokeText:${text}:${x}:${y}`),
    drawImage: () => canvasOps.push("drawImage"),
    putImageData: () => canvasOps.push("putImageData"),
    setLineDash: (segments: number[]) =>
      canvasOps.push(`lineDash:${segments.join(",")}`),
    createLinearGradient: () => {
      canvasOps.push("linearGradient");
      return gradient;
    },
    createRadialGradient: () => {
      canvasOps.push("radialGradient");
      return gradient;
    },
    createConicGradient: () => {
      canvasOps.push("conicGradient");
      return gradient;
    },
    measureText: (text: string) => {
      canvasOps.push(`measureText:${text}`);
      return { width: text.length * 8 };
    },
    isPointInPath: () => true,
  };
  Object.defineProperty(canvasContext, "textAlign", {
    get: () => textAlign,
    set: (value: CanvasTextAlign) => {
      textAlign = value;
      canvasOps.push(`textAlign:${value}`);
    },
  });
  Object.defineProperty(canvasContext, "textBaseline", {
    set: (value) => canvasOps.push(`textBaseline:${value}`),
  });
  Object.defineProperty(canvasContext, "globalAlpha", {
    set: (value) => canvasOps.push(`globalAlpha:${value}`),
  });
  Object.defineProperty(canvasContext, "globalCompositeOperation", {
    set: (value) => canvasOps.push(`composite:${value}`),
  });
  Object.defineProperty(canvasContext, "filter", {
    set: (value) => canvasOps.push(`filter:${value}`),
  });
  vi.spyOn(HTMLCanvasElement.prototype, "toDataURL").mockReturnValue(
    "data:image/png;base64,test",
  );
  vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue(
    canvasContext as unknown as CanvasRenderingContext2D,
  );
  vi.stubGlobal("ResizeObserver", FakeResizeObserver);
  vi.stubGlobal("Path2D", FakePath2D);
  vi.stubGlobal("WebSocket", FakeWS);
  installFetch((url) => {
    if (url.includes("/tickets"))
      return { status: 201, body: { ticket: "t1" } };
    return { body: { content: "config: true", columns: [], rows: [] } };
  });
});
afterEach(() => {
  useStreamChannelsStore().closeForConnection("c1");
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

const panels = [
  { name: "terminal", comp: TerminalPanel, status: true },
  { name: "terminal grid", comp: TerminalGridPanel, status: true },
  { name: "logs", comp: LogStreamPanel, status: true },
  { name: "metrics", comp: MetricsPanel, status: true },
  { name: "remote desktop", comp: RemoteDesktopPanel, status: true },
  { name: "query editor", comp: QueryEditorPanel, status: true },
  { name: "canvas", comp: CanvasPanel, status: true },
  { name: "code editor", comp: CodeEditorPanel, status: false },
];

describe("streaming stub panels", () => {
  for (const p of panels) {
    it(`${p.name} mounts and unmounts without throwing`, async () => {
      const w = mount(p.comp, { props, global: { plugins: [pinia] } });
      await flushPromises();
      expect(w.text()).not.toContain("Stub panel");
      if (p.status) expect(w.text()).toContain("Connecting");
      expect(() => w.unmount()).not.toThrow();
    });
  }

  it("reuses the open channel on remount (stream survives navigation away/back)", async () => {
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

  it("sends terminal resize controls with the current theme", async () => {
    const w = mount(TerminalPanel, {
      props,
      global: { plugins: [pinia] },
    });
    await flushPromises();
    const socket = streamSockets()[0];
    socket.emit("open");
    await nextTick();
    await flushPromises();
    expect(
      socket.sent.some(
        (msg) =>
          msg.startsWith("\0") &&
          msg.includes('"type":"resize"') &&
          msg.includes('"theme":'),
      ),
    ).toBe(true);
    w.unmount();
  });

  it("renders canvas frames and sends pointer input", async () => {
    const w = mount(CanvasPanel, {
      props: {
        ...props,
        config: { interactive: true, pointer: true, keyboard: true },
      },
      global: { plugins: [pinia] },
    });
    await flushPromises();
    const socket = streamSockets()[0];
    socket.emit("open");
    socket.emit("message", {
      data: JSON.stringify({
        commands: [
          { type: "clear", color: "#000" },
          { type: "rect", x: 1, y: 2, width: 3, height: 4, fill: "#fff" },
          {
            type: "text",
            x: 50,
            y: 20,
            text: "centered",
            textAlign: "center",
            textBaseline: "middle",
          },
          { type: "text", x: 10, y: 40, text: "normal" },
        ],
        regions: [{ id: "button", x: 0, y: 0, width: 100, height: 100 }],
      }),
    });
    await nextTick();
    expect(canvasOps).toContain("rect");
    expect(canvasOps).toContain("textAlign:center");
    expect(canvasOps.filter((op) => op.startsWith("textBaseline:"))).toContain(
      "textBaseline:middle",
    );

    const canvas = w.get('[data-test="canvas-panel-canvas"]');
    vi.spyOn(canvas.element, "getBoundingClientRect").mockReturnValue({
      left: 0,
      top: 0,
      width: 100,
      height: 100,
      right: 100,
      bottom: 100,
      x: 0,
      y: 0,
      toJSON: () => ({}),
    } as DOMRect);
    canvas.element.dispatchEvent(
      new MouseEvent("pointerdown", {
        clientX: 10,
        clientY: 10,
        button: 0,
        buttons: 1,
        bubbles: true,
      }),
    );
    canvas.element.dispatchEvent(
      new MouseEvent("click", {
        clientX: 10,
        clientY: 10,
        button: 0,
        buttons: 0,
        bubbles: true,
      }),
    );
    await nextTick();
    expect(socket.sent.some((msg) => msg.includes('"type":"pointer"'))).toBe(
      true,
    );
    expect(socket.sent.some((msg) => msg.includes('"regionId":"button"'))).toBe(
      true,
    );
    expect(
      socket.sent.some(
        (msg) => msg.includes('"type":"ready"') && msg.includes('"theme":'),
      ),
    ).toBe(true);
    expect(socket.sent.some((msg) => msg.includes('"event":"click"'))).toBe(
      false,
    );
    w.unmount();
  });

  it("treats non-interactive canvas panels as visualizations", async () => {
    const w = mount(CanvasPanel, {
      props: {
        ...props,
        config: { resizeEvents: true, ariaLabel: "Kubernetes service flow" },
      },
      global: { plugins: [pinia] },
    });
    await flushPromises();
    const canvas = w.get('[data-test="canvas-panel-canvas"]');
    expect(canvas.attributes("role")).toBe("img");
    expect(canvas.attributes("tabindex")).toBe("-1");
    expect(canvas.attributes("aria-label")).toBe("Kubernetes service flow");
    w.unmount();
  });

  it("uses declared canvas dimensions as a scrollable drawing surface", async () => {
    const w = mount(CanvasPanel, {
      props: {
        ...props,
        config: { width: 1600, height: 900, scrollable: true },
      },
      global: { plugins: [pinia] },
    });
    await flushPromises();
    const socket = streamSockets()[0];
    socket.emit("open");
    await nextTick();
    await flushPromises();
    await nextTick();

    const canvas = w.get<HTMLCanvasElement>(
      '[data-test="canvas-panel-canvas"]',
    ).element;
    expect(canvas.style.width).toBe("1600px");
    expect(canvas.style.height).toBe("900px");
    expect(
      socket.sent.some(
        (msg) =>
          msg.includes('"type":"ready"') &&
          msg.includes('"width":1600') &&
          msg.includes('"height":900'),
      ),
    ).toBe(true);
    w.unmount();
  });

  it("does not leak canvas alpha between frames", async () => {
    const w = mount(CanvasPanel, {
      props: {
        ...props,
        config: { interactive: true },
      },
      global: { plugins: [pinia] },
    });
    await flushPromises();
    const socket = streamSockets()[0];
    socket.emit("open");
    socket.emit("message", {
      data: JSON.stringify({
        commands: [
          { type: "clear", color: "#020617" },
          { type: "circle", x: 10, y: 10, radius: 5, fill: "#fff", alpha: 0.2 },
        ],
      }),
    });
    socket.emit("message", {
      data: JSON.stringify({
        commands: [
          { type: "clear", color: "#020617" },
          { type: "rect", x: 1, y: 1, width: 10, height: 10, fill: "#fff" },
        ],
      }),
    });
    await nextTick();
    const alphaOps = canvasOps.filter((op) => op.startsWith("globalAlpha:"));
    expect(alphaOps).toContain("globalAlpha:0.2");
    expect(alphaOps.at(-1)).toBe("globalAlpha:1");
    w.unmount();
  });

  it("ignores invalid canvas paths without breaking later commands", async () => {
    const w = mount(CanvasPanel, {
      props: { ...props, config: { interactive: true } },
      global: { plugins: [pinia] },
    });
    await flushPromises();
    const socket = streamSockets()[0];
    socket.emit("open");
    expect(() =>
      socket.emit("message", {
        data: JSON.stringify({
          commands: [
            { type: "path", d: "bad path", fill: "#fff" },
            { type: "rect", x: 1, y: 1, width: 10, height: 10, fill: "#fff" },
          ],
          regions: [{ id: "bad", shape: "path", d: "bad path" }],
        }),
      }),
    ).not.toThrow();
    await nextTick();
    expect(canvasOps).toContain("rect");
    w.unmount();
  });

  it("rerenders the latest canvas frame when an image finishes loading", async () => {
    let imageComplete = false;
    let imageOnload: (() => void) | undefined;
    vi.stubGlobal(
      "Image",
      class {
        get complete() {
          return imageComplete;
        }
        naturalWidth = 16;
        naturalHeight = 16;
        width = 16;
        height = 16;
        get onload() {
          return imageOnload;
        }
        set onload(fn: (() => void) | undefined) {
          imageOnload = fn;
        }
        onerror?: () => void;
        crossOrigin = "";
        src = "";
      },
    );
    const w = mount(CanvasPanel, {
      props: { ...props, config: { interactive: true } },
      global: { plugins: [pinia] },
    });
    await flushPromises();
    const socket = streamSockets()[0];
    socket.emit("open");
    socket.emit("message", {
      data: JSON.stringify({
        commands: [
          { type: "clear", color: "#020617" },
          { type: "image", src: "https://example.test/image.png", x: 4, y: 5 },
        ],
      }),
    });
    await nextTick();
    expect(canvasOps).not.toContain("drawImage");
    expect(imageOnload).toBeDefined();
    imageComplete = true;
    imageOnload?.();
    await nextTick();
    expect(canvasOps).toContain("drawImage");
    w.unmount();
  });

  it("renders expanded canvas 2d commands and sends renderer responses", async () => {
    vi.stubGlobal(
      "ImageData",
      class {
        data: Uint8ClampedArray;
        width: number;
        height: number;
        constructor(data: Uint8ClampedArray, width: number, height: number) {
          this.data = data;
          this.width = width;
          this.height = height;
        }
      },
    );
    const w = mount(CanvasPanel, {
      props: {
        ...props,
        config: { interactive: true, pointer: true, keyboard: true },
      },
      global: { plugins: [pinia] },
    });
    await flushPromises();
    const socket = streamSockets()[0];
    socket.emit("open");
    socket.emit("message", {
      data: JSON.stringify({
        commands: [
          {
            type: "gradient",
            id: "g1",
            kind: "linear",
            x0: 0,
            y0: 0,
            x1: 100,
            y1: 0,
            stops: [
              { offset: 0, color: "#000" },
              { offset: 1, color: "#fff" },
            ],
          },
          { type: "style", fillId: "g1", stroke: "#fff", lineWidth: 2 },
          { type: "lineDash", segments: [4, 2], offset: 1 },
          { type: "shadow", color: "#000", blur: 8, offsetX: 2, offsetY: 3 },
          { type: "clip", shape: "rect", x: 0, y: 0, width: 200, height: 120 },
          {
            type: "arc",
            x: 50,
            y: 50,
            radius: 20,
            startAngle: 0,
            endAngle: Math.PI,
          },
          {
            type: "quadraticCurve",
            x0: 0,
            y0: 0,
            cpx: 20,
            cpy: 60,
            x: 80,
            y: 10,
            stroke: "#fff",
            fill: false,
          },
          {
            type: "bezierCurve",
            x0: 0,
            y0: 0,
            cp1x: 10,
            cp1y: 20,
            cp2x: 40,
            cp2y: 30,
            x: 80,
            y: 20,
            stroke: "#fff",
            fill: false,
          },
          {
            type: "textBox",
            x: 10,
            y: 10,
            width: 80,
            text: "Wrapped canvas label",
            lineHeight: 16,
          },
          {
            type: "textBox",
            x: 20,
            y: 60,
            width: 160,
            text: "Centered",
            textAlign: "center",
          },
          { type: "measureText", requestId: "m1", text: "Measure me" },
          {
            type: "imageData",
            x: 1,
            y: 1,
            width: 1,
            height: 1,
            data: [255, 0, 0, 255],
          },
          { type: "snapshot", requestId: "s1", mime: "image/png" },
        ],
      }),
    });
    await nextTick();

    for (const op of [
      "linearGradient",
      "lineDash:4,2",
      "clip",
      "arc",
      "quadraticCurveTo",
      "bezierCurveTo",
      "measureText:Measure me",
      "fillText:Centered:100:60",
      "putImageData",
    ]) {
      expect(canvasOps).toContain(op);
    }
    expect(
      socket.sent.some((msg) => msg.includes('"type":"textMetrics"')),
    ).toBe(true);
    expect(socket.sent.some((msg) => msg.includes('"requestId":"m1"'))).toBe(
      true,
    );
    expect(socket.sent.some((msg) => msg.includes('"type":"snapshot"'))).toBe(
      true,
    );
    expect(socket.sent.some((msg) => msg.includes('"requestId":"s1"'))).toBe(
      true,
    );
    w.unmount();
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

  it("ignores a stale pending ticket when reconnect is forced", async () => {
    const ticketResolvers: ((ticket: string) => void)[] = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = typeof input === "string" ? input : input.toString();
        if (url.includes("/tickets")) {
          const ticket = await new Promise<string>((resolve) =>
            ticketResolvers.push(resolve),
          );
          return new Response(JSON.stringify({ ticket }), {
            status: 201,
            headers: { "Content-Type": "application/json" },
          });
        }
        return new Response(JSON.stringify({}), {
          headers: { "Content-Type": "application/json" },
        });
      }),
    );
    const Host = defineComponent({
      setup() {
        const stream = useStream("c1", props.source, {});
        return { reconnect: stream.reconnect };
      },
      template: '<button type="button" @click="reconnect">Reconnect</button>',
    });
    const w = mount(Host, { global: { plugins: [pinia] } });
    await nextTick();
    expect(ticketResolvers).toHaveLength(1);

    await w.get("button").trigger("click");
    expect(ticketResolvers).toHaveLength(2);

    ticketResolvers[0]("stale");
    await flushPromises();
    expect(streamSockets()).toHaveLength(0);

    ticketResolvers[1]("fresh");
    await flushPromises();
    expect(streamSockets()).toHaveLength(1);
    expect(streamSockets()[0].url).toContain("ticket=fresh");
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
