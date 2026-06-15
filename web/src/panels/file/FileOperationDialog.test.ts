import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { nextTick } from "vue";
import { createPinia, setActivePinia } from "pinia";
import PrimeVue from "primevue/config";
import { primeVuePassthrough } from "@/primevue/preset";
import { installFetch } from "@/test/fetchMock";
import { FileOperation } from "@/types/projection";
import FileOperationDialog from "./FileOperationDialog.vue";

beforeEach(() => {
  setActivePinia(createPinia());
  document.body.innerHTML = "";
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

function mountDialog(props = {}) {
  return mount(FileOperationDialog, {
    props: {
      visible: true,
      connectionId: "c1",
      ctx: {},
      routeId: "sftp.copy",
      operation: FileOperation.Copy,
      paths: ["/README.md"],
      defaultDestination: "/archive",
      ...props,
    },
    global: {
      plugins: [
        createPinia(),
        [PrimeVue, { unstyled: true, pt: primeVuePassthrough }],
      ],
    },
  });
}

describe("FileOperationDialog", () => {
  it("runs a destination operation through its action route", async () => {
    const calls: { url: string; init?: RequestInit }[] = [];
    installFetch((url, init) => {
      calls.push({ url, init });
      return { body: { ok: true } };
    });

    const w = mountDialog();
    await nextTick();
    bodyButton("Copy").click();
    await flushPromises();

    expect(calls[0]?.url).toContain("sftp.copy");
    expect(JSON.parse(String(calls[0]?.init?.body))).toEqual({
      paths: ["/README.md"],
      destination: "/archive",
    });
    expect(w.emitted("complete")).toHaveLength(1);
  });

  it("selects a destination from the folder tree", async () => {
    const calls: { url: string; init?: RequestInit }[] = [];
    installFetch((url, init) => {
      calls.push({ url, init });
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

    mountDialog({
      defaultDestination: "/",
      folderSource: { routeId: "sftp.list", params: { path: "/" } },
      pathParam: "path",
    });

    await flushPromises();

    const treeWrapper = document.body.querySelector(
      '[data-pc-section="wrapper"]',
    );
    expect(treeWrapper?.classList.contains("max-h-56")).toBe(true);

    treeNode("etc").click();
    await nextTick();

    const input = document.body.querySelector(
      'input[aria-label="Operation destination"]',
    ) as HTMLInputElement;
    expect(input.value).toBe("/etc");

    bodyButton("Copy").click();
    await flushPromises();

    const actionCall = calls.find((call) => call.url.includes("sftp.copy"));
    expect(JSON.parse(String(actionCall?.init?.body))).toMatchObject({
      destination: "/etc",
    });
  });
});
