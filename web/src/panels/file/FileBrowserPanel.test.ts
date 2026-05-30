import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { nextTick } from "vue";
import { createPinia, setActivePinia } from "pinia";
import { installFetch } from "../../test/fetchMock";
import FileBrowserPanel from "./FileBrowserPanel.vue";

const codeMirrorEditors = vi.hoisted(
  () =>
    [] as Array<{
      value: string;
      setValue(value: string): void;
      emitChange(): void;
    }>,
);

vi.mock("../../codemirror", () => ({
  createCodeMirrorEditor: (
    _container: HTMLElement,
    options: { value: string; onChange?: (value: string) => void },
  ) => {
    let value = options.value;
    const editor = {
      get value() {
        return value;
      },
      setValue(next: string) {
        value = next;
      },
      emitChange() {
        options.onChange?.(value);
      },
      view: {
        destroy() {},
      },
    };
    codeMirrorEditors.push(editor);
    return editor;
  },
  editorValue: (editor: { value: string } | null) => editor?.value ?? "",
  setEditorValue: (
    editor: { setValue(value: string): void } | null,
    value: string,
  ) => editor?.setValue(value),
  setEditorLanguage: () => {},
  setEditorReadOnly: () => {},
  syncCodeMirrorTheme: () => {},
}));

const rootEntries = [
  { name: "etc", path: "/etc", isDir: true },
  {
    name: "README.md",
    path: "/README.md",
    isDir: false,
    size: 100,
    mime: "text/markdown",
  },
];

beforeEach(() => {
  setActivePinia(createPinia());
  codeMirrorEditors.length = 0;
  installFetch((url) => {
    const u = new URL(url, "http://h");
    if (url.includes("sftp.list")) {
      const path = u.searchParams.get("p.path");
      if (path === "/etc")
        return {
          body: {
            items: [
              {
                name: "hosts",
                path: "/etc/hosts",
                isDir: false,
                mime: "text/plain",
              },
            ],
            nextCursor: "",
          },
        };
      return { body: { items: rootEntries, nextCursor: "" } };
    }
    if (url.includes("sftp.read")) {
      return {
        body: {
          path: "/README.md",
          mime: "text/plain",
          encoding: "utf8",
          content: "# Hello",
        },
      };
    }
    return { body: {} };
  });
});
afterEach(() => {
  document.body.innerHTML = "";
  vi.unstubAllGlobals();
});

function writableConfig() {
  return {
    pathParam: "path",
    readRouteId: "ssh.sftp.read",
    downloadRouteId: "ssh.sftp.download",
    writeRouteId: "ssh.sftp.write",
    uploadRouteId: "ssh.sftp.upload",
    mkdirRouteId: "ssh.sftp.mkdir",
    renameRouteId: "ssh.sftp.rename",
    deleteRouteId: "ssh.sftp.delete",
    writable: true,
  };
}

function bodyButton(text: string): HTMLButtonElement | undefined {
  return [...document.body.querySelectorAll("button")].find(
    (b) => b.textContent?.trim() === text,
  ) as HTMLButtonElement | undefined;
}

function panelButton(w: ReturnType<typeof mount>, text: string) {
  return w.findAll("button").find((b) => b.text().trim() === text)!;
}

function panelButtonByLabel(w: ReturnType<typeof mount>, label: string) {
  return w.findAll("button").find((b) => b.attributes("aria-label") === label)!;
}

async function setBodyInput(placeholder: string, value: string): Promise<void> {
  const input = document.body.querySelector(
    `input[placeholder="${placeholder}"]`,
  ) as HTMLInputElement;
  input.value = value;
  input.dispatchEvent(new Event("input"));
  await nextTick();
}

describe("FileBrowserPanel", () => {
  it("lists a directory (dirs first) and previews a selected text file", async () => {
    const w = mount(FileBrowserPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "ssh.sftp.list", params: { path: "/" } },
        config: { pathParam: "path", readRouteId: "ssh.sftp.read" },
      },
    });
    await flushPromises();
    const items = w.findAll("li button");
    expect(items[0].text()).toContain("etc"); // directory sorts first
    expect(items[0].attributes("title")).toBe("/etc");
    expect(w.text()).toContain("README.md");

    const file = items.find((b) => b.text().includes("README.md"));
    await file!.trigger("click");
    await flushPromises();
    await vi.waitFor(() =>
      expect(w.find(".shellcn-codemirror-host").exists()).toBe(true),
    );
  });

  it("filters the listing by name and shows a no-match empty state", async () => {
    const w = mount(FileBrowserPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "ssh.sftp.list", params: { path: "/" } },
        config: { pathParam: "path", readRouteId: "ssh.sftp.read" },
      },
    });
    await flushPromises();
    expect(w.text()).toContain("etc");
    expect(w.text()).toContain("README.md");

    const filter = w.get('input[aria-label="Filter files"]');
    await filter.setValue("readme");
    expect(w.text()).toContain("README.md");
    expect(w.text()).not.toContain("etc");

    await filter.setValue("zzz-nomatch");
    expect(w.text()).toContain("No items match your filter.");
  });

  it("disables Rename until the name actually changes", async () => {
    const w = mount(FileBrowserPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "ssh.sftp.list", params: { path: "/" } },
        config: writableConfig(),
      },
    });
    await flushPromises();

    await w
      .findAll("li button")
      .find((b) => b.text().includes("README.md"))!
      .trigger("click");
    await flushPromises();
    await panelButtonByLabel(w, "Rename selected item").trigger("click");
    await flushPromises();

    // Pre-filled with the current name → no change → disabled.
    expect(bodyButton("Rename")!.disabled).toBe(true);

    await setBodyInput("Name", "NOTES.md");
    expect(bodyButton("Rename")!.disabled).toBe(false);

    // Reverting to the original name disables it again.
    await setBodyInput("Name", "README.md");
    expect(bodyButton("Rename")!.disabled).toBe(true);
  });

  it("shows the selected file's metadata and a download action in the preview header", async () => {
    const w = mount(FileBrowserPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "ssh.sftp.list", params: { path: "/" } },
        config: writableConfig(),
      },
    });
    await flushPromises();
    await w
      .findAll("li button")
      .find((b) => b.text().includes("README.md"))!
      .trigger("click");
    await flushPromises();
    // Header download link points at the download route for the selected path.
    const dl = w
      .findAll("a")
      .find((a) => a.attributes("href")?.includes("ssh.sftp.download"));
    expect(dl?.attributes("href")).toContain("p.path=%2FREADME.md");
  });

  it("streams media via an inline URL and skips the read fetch", async () => {
    const calls: string[] = [];
    vi.unstubAllGlobals();
    installFetch((url) => {
      calls.push(url);
      if (url.includes("sftp.list"))
        return {
          body: {
            items: [
              {
                name: "clip.mp4",
                path: "/clip.mp4",
                isDir: false,
                size: 999,
                mime: "video/mp4",
              },
            ],
            nextCursor: "",
          },
        };
      return { body: {} };
    });
    const w = mount(FileBrowserPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "ssh.sftp.list", params: { path: "/" } },
        config: writableConfig(),
      },
    });
    await flushPromises();
    await w
      .findAll("li button")
      .find((b) => b.text().includes("clip.mp4"))!
      .trigger("click");
    await flushPromises();

    const video = w.find("video");
    expect(video.exists()).toBe(true);
    expect(video.attributes("src")).toContain("ssh.sftp.download");
    expect(video.attributes("src")).toContain("inline=1");
    expect(calls.some((u) => u.includes("sftp.read"))).toBe(false);
  });

  it("shows an inline file preview error with retry", async () => {
    let failRead = true;
    vi.unstubAllGlobals();
    installFetch((url) => {
      if (url.includes("sftp.read") && failRead) {
        return { status: 500, body: { error: "read failed" } };
      }
      if (url.includes("sftp.read")) {
        return {
          body: {
            path: "/README.md",
            mime: "text/plain",
            encoding: "utf8",
            content: "# Hello",
          },
        };
      }
      return { body: { items: rootEntries, nextCursor: "" } };
    });

    const w = mount(FileBrowserPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "ssh.sftp.list", params: { path: "/" } },
        config: { pathParam: "path", readRouteId: "ssh.sftp.read" },
      },
    });
    await flushPromises();

    await w
      .findAll("li button")
      .find((b) => b.text().includes("README.md"))!
      .trigger("click");
    await flushPromises();

    expect(w.text()).toContain("read failed");
    failRead = false;
    await panelButton(w, "Retry").trigger("click");
    await flushPromises();
    await vi.waitFor(() =>
      expect(w.find(".shellcn-codemirror-host").exists()).toBe(true),
    );
  });

  it("navigates into a directory", async () => {
    const w = mount(FileBrowserPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "ssh.sftp.list", params: { path: "/" } },
        config: { pathParam: "path", readRouteId: "ssh.sftp.read" },
      },
    });
    await flushPromises();
    await w.get('[aria-label="Open etc"]').trigger("click");
    await flushPromises();
    expect(w.text()).toContain("hosts");
  });

  it("uses the resolved server path for the initial home directory", async () => {
    vi.unstubAllGlobals();
    installFetch((url) => {
      if (url.includes("sftp.list")) {
        return {
          body: {
            path: "/home/deploy",
            items: [
              {
                name: "app.json",
                path: "/home/deploy/app.json",
                isDir: false,
                mime: "application/json",
              },
            ],
            nextCursor: "",
          },
        };
      }
      return { body: {} };
    });

    const w = mount(FileBrowserPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "ssh.sftp.list", params: { path: "." } },
        config: { pathParam: "path", readRouteId: "ssh.sftp.read" },
      },
    });
    await flushPromises();

    expect(w.text()).toContain("home");
    expect(w.text()).toContain("deploy");
    expect(w.text()).toContain("app.json");
    expect(w.text()).not.toContain("/ /");
  });

  it("offers a grid view that opens file previews in a dialog", async () => {
    const w = mount(FileBrowserPanel, {
      attachTo: document.body,
      props: {
        connectionId: "c1",
        source: { routeId: "ssh.sftp.list", params: { path: "/" } },
        config: { pathParam: "path", readRouteId: "ssh.sftp.read" },
      },
    });
    await flushPromises();
    await w
      .findAll("button")
      .find((b) => b.text().includes("Grid view"))!
      .trigger("click");
    await w
      .findAll("button")
      .find((b) => b.text().includes("README.md"))!
      .trigger("dblclick");
    await flushPromises();

    expect(document.body.textContent).toContain("README.md");
    await vi.waitFor(() =>
      expect(
        document.body.querySelector(".shellcn-codemirror-host"),
      ).toBeTruthy(),
    );
    w.unmount();
  });

  it("wires declared file operations to route IDs and path params", async () => {
    const calls: { url: string; init?: RequestInit }[] = [];
    vi.unstubAllGlobals();
    installFetch((url, init) => {
      calls.push({ url, init });
      if (init?.method && init.method !== "GET") {
        return { body: { ok: true } };
      }
      if (url.includes("sftp.read")) {
        return {
          body: {
            path: "/README.md",
            mime: "text/plain",
            encoding: "utf8",
            content: "# Hello",
          },
        };
      }
      return { body: { items: rootEntries, nextCursor: "" } };
    });

    const w = mount(FileBrowserPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "ssh.sftp.list", params: { path: "/" } },
        config: writableConfig(),
      },
    });
    await flushPromises();

    await panelButton(w, "New folder").trigger("click");
    await setBodyInput("Folder name", "logs");
    bodyButton("Create")!.click();
    await flushPromises();

    const mkdir = calls.find((c) => c.url.includes("ssh.sftp.mkdir"))!;
    expect(mkdir.url).toContain("p.path=%2F");
    expect(mkdir.init?.method).toBe("POST");
    expect(JSON.parse(String(mkdir.init?.body))).toEqual({ name: "logs" });

    const file = w
      .findAll("li button")
      .find((b) => b.text().includes("README.md"));
    await file!.trigger("click");
    await flushPromises();
    expect(w.get("a").attributes("href")).toContain(
      "/api/connections/c1/x/ssh.sftp.download?p.path=%2FREADME.md",
    );

    await panelButtonByLabel(w, "Rename selected item").trigger("click");
    await setBodyInput("Name", "NOTES.md");
    bodyButton("Rename")!.click();
    await flushPromises();

    const rename = calls.find((c) => c.url.includes("ssh.sftp.rename"))!;
    expect(rename.url).toContain("p.path=%2FREADME.md");
    expect(rename.init?.method).toBe("PATCH");
    expect(JSON.parse(String(rename.init?.body))).toEqual({ name: "NOTES.md" });

    const fileAgain = w
      .findAll("li button")
      .find((b) => b.text().includes("README.md"));
    await fileAgain!.trigger("click");
    await flushPromises();
    await vi.waitFor(() => expect(codeMirrorEditors.length).toBeGreaterThan(0));
    const editor = codeMirrorEditors.at(-1)!;
    editor.setValue("# Updated");
    editor.emitChange();
    await nextTick();
    await panelButton(w, "Save").trigger("click");
    await flushPromises();

    const write = calls.find((c) => c.url.includes("ssh.sftp.write"))!;
    expect(write.url).toContain("p.path=%2FREADME.md");
    expect(write.init?.method).toBe("PUT");
    expect(JSON.parse(String(write.init?.body))).toEqual({
      content: "# Updated",
    });

    const fileForDelete = w
      .findAll("li button")
      .find((b) => b.text().includes("README.md"));
    await fileForDelete!.trigger("click");
    await panelButtonByLabel(w, "Delete selected item").trigger("click");
    bodyButton("Delete")!.click();
    await flushPromises();

    const del = calls.find((c) => c.url.includes("ssh.sftp.delete"))!;
    expect(del.url).toContain("p.path=%2FREADME.md");
    expect(del.init?.method).toBe("DELETE");
    expect(JSON.parse(String(del.init?.body))).toEqual({ path: "/README.md" });
  });
});
