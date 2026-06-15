import { describe, it, expect, beforeEach, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { ref, nextTick } from "vue";
import PrimeVue from "primevue/config";
import { primeVuePassthrough } from "@/primevue/preset";
import { FileTransferOperation } from "@/types/projection";
import FileTransferDialog from "./FileTransferDialog.vue";

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
  document.body.innerHTML = "";
  send.mockClear();
  reconnect.mockClear();
  onFrame = undefined;
  vi.stubGlobal("crypto", { randomUUID: () => "transfer-1" });
});

function bodyButton(text: string): HTMLButtonElement {
  const button = [...document.body.querySelectorAll("button")].find((b) =>
    b.textContent?.includes(text),
  );
  if (!button) throw new Error(`button ${text} not found`);
  return button as HTMLButtonElement;
}

describe("FileTransferDialog", () => {
  it("starts, cancels, and renders progress frames", async () => {
    const w = mount(FileTransferDialog, {
      props: {
        visible: true,
        connectionId: "c1",
        ctx: {},
        config: {
          source: { routeId: "sftp.transfer" },
          operations: [FileTransferOperation.Copy],
        },
        operation: FileTransferOperation.Copy,
        paths: ["/README.md"],
        defaultDestination: "/archive",
      },
      global: {
        plugins: [[PrimeVue, { unstyled: true, pt: primeVuePassthrough }]],
      },
    });

    await nextTick();
    const input = document.body.querySelector(
      'input[aria-label="Transfer destination"]',
    ) as HTMLInputElement;
    input.value = "/dst";
    input.dispatchEvent(new Event("input"));
    await nextTick();
    bodyButton("Copy").click();

    expect(JSON.parse(send.mock.calls[0]![0])).toEqual({
      type: "start",
      transferId: "transfer-1",
      operation: FileTransferOperation.Copy,
      paths: ["/README.md"],
      destination: "/dst",
    });

    onFrame?.(
      JSON.stringify({
        type: "progress",
        transferId: "transfer-1",
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
      transferId: "transfer-1",
    });

    onFrame?.(
      JSON.stringify({
        type: "complete",
        transferId: "transfer-1",
        message: "Copied",
      }),
    );
    await nextTick();
    expect(w.emitted("complete")).toHaveLength(1);
  });
});
