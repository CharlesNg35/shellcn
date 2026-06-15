import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { ref, nextTick } from "vue";
import { createPinia, setActivePinia } from "pinia";
import PrimeVue from "primevue/config";
import { primeVuePassthrough } from "@/primevue/preset";
import { installFetch } from "@/test/fetchMock";
import { FileJobOperation } from "@/types/projection";
import FileJobDialog from "./FileJobDialog.vue";

const send = vi.hoisted(() => vi.fn());
const reconnect = vi.hoisted(() => vi.fn());
let onFrame: ((data: string) => void) | undefined;

vi.mock("@/composables/useStream", () => ({
  useStream: (
    _connectionId: string,
    _source: unknown,
    _ctx: unknown,
    frameHandler?: (data: string) => void,
  ) => {
    onFrame = frameHandler;
    return {
      status: ref("open"),
      error: ref(null),
      send,
      reconnect,
    };
  },
}));

beforeEach(() => {
  setActivePinia(createPinia());
  document.body.innerHTML = "";
  send.mockClear();
  reconnect.mockClear();
  onFrame = undefined;
  vi.stubGlobal("crypto", { randomUUID: () => "file-job-1" });
});

afterEach(() => {
  vi.unstubAllGlobals();
});

function bodyButton(text: string): HTMLButtonElement {
  const button = [...document.body.querySelectorAll("button")].find((b) =>
    b.textContent?.includes(text),
  );
  if (!button) throw new Error(`button ${text} not found`);
  return button as HTMLButtonElement;
}

function treeNode(text: string): HTMLElement {
  const label = [
    ...document.body.querySelectorAll('[data-pc-section="nodelabel"]'),
  ].find((el) => el.textContent?.trim() === text);
  if (!label) throw new Error(`tree label ${text} not found`);
  const node = label.closest('[data-pc-section="nodecontent"]');
  if (!node) throw new Error(`tree node ${text} not found`);
  return node as HTMLElement;
}

describe("FileJobDialog", () => {
  it("starts, cancels, and renders progress frames", async () => {
    const w = mount(FileJobDialog, {
      props: {
        visible: true,
        connectionId: "c1",
        ctx: {},
        config: {
          source: { routeId: "sftp.jobs" },
          operations: [FileJobOperation.Copy],
        },
        operation: FileJobOperation.Copy,
        paths: ["/README.md"],
        defaultDestination: "/archive",
      },
      global: {
        plugins: [
          createPinia(),
          [PrimeVue, { unstyled: true, pt: primeVuePassthrough }],
        ],
      },
    });

    await nextTick();
    const input = document.body.querySelector(
      'input[aria-label="Job destination"]',
    ) as HTMLInputElement;
    input.value = "/dst";
    input.dispatchEvent(new Event("input"));
    await nextTick();
    bodyButton("Copy").click();

    expect(JSON.parse(send.mock.calls[0]![0])).toEqual({
      type: "start",
      jobId: "file-job-1",
      operation: FileJobOperation.Copy,
      paths: ["/README.md"],
      destination: "/dst",
    });

    onFrame?.(
      JSON.stringify({
        type: "progress",
        jobId: "file-job-1",
        status: "Copying",
        bytesDone: 512,
        bytesTotal: 1024,
        filesDone: 1,
        filesTotal: 2,
        percent: 50,
      }),
    );
    await nextTick();
    expect(document.body.textContent).toContain("Copying");
    expect(document.body.textContent).toContain("1/2 files");
    expect(document.body.textContent).toContain("512 B / 1.0 KB");

    bodyButton("Cancel").click();
    expect(JSON.parse(send.mock.calls[1]![0])).toEqual({
      type: "cancel",
      jobId: "file-job-1",
    });

    onFrame?.(
      JSON.stringify({
        type: "complete",
        jobId: "file-job-1",
        message: "Copied",
      }),
    );
    await nextTick();
    expect(w.emitted("complete")).toHaveLength(1);
  });

  it("selects a destination from the folder tree", async () => {
    installFetch((url) => {
      const u = new URL(url, "http://h");
      const path = u.searchParams.get("p.path");
      if (path === "/") {
        return {
          body: {
            items: [
              { name: "etc", path: "/etc", isDir: true },
              { name: "README.md", path: "/README.md", isDir: false },
            ],
            nextCursor: "",
          },
        };
      }
      return { body: { items: [], nextCursor: "" } };
    });

    mount(FileJobDialog, {
      props: {
        visible: true,
        connectionId: "c1",
        ctx: {},
        config: {
          source: { routeId: "sftp.jobs" },
          operations: [FileJobOperation.Copy],
        },
        operation: FileJobOperation.Copy,
        paths: ["/README.md"],
        defaultDestination: "/",
        folderSource: { routeId: "sftp.list", params: { path: "/" } },
        pathParam: "path",
      },
      global: {
        plugins: [
          createPinia(),
          [PrimeVue, { unstyled: true, pt: primeVuePassthrough }],
        ],
      },
    });

    await flushPromises();
    treeNode("etc").click();
    await nextTick();

    const input = document.body.querySelector(
      'input[aria-label="Job destination"]',
    ) as HTMLInputElement;
    expect(input.value).toBe("/etc");

    bodyButton("Copy").click();
    expect(JSON.parse(send.mock.calls[0]![0])).toMatchObject({
      type: "start",
      destination: "/etc",
    });
  });
});
