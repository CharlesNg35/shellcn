import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { h, nextTick } from "vue";
import { createPinia, setActivePinia } from "pinia";
import ConfirmDialog from "primevue/confirmdialog";
import { installFetch } from "@/test/fetchMock";
import type { FileBrowserConfig } from "@/types/projection";
import FileBrowserPanel from "./FileBrowserPanel.vue";
import FileToolbar from "./FileToolbar.vue";

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

type TestFileBrowserConfig = FileBrowserConfig & Record<string, unknown>;

function writableConfig(): TestFileBrowserConfig {
  return {
    pathParam: "path",
    routes: {
      read: "ssh.sftp.read",
      download: "ssh.sftp.download",
      write: "ssh.sftp.write",
      mkdir: "ssh.sftp.mkdir",
      rename: "ssh.sftp.rename",
      delete: "ssh.sftp.delete",
    },
    upload: {
      routeId: "ssh.sftp.upload",
      fieldName: "files",
      multiple: true,
    },
    writable: true,
  } as TestFileBrowserConfig;
}

function bulkConfig(): TestFileBrowserConfig {
  return {
    ...writableConfig(),
    routes: {
      ...writableConfig().routes,
      move: "ssh.sftp.move",
      copy: "ssh.sftp.copy",
      chmod: "ssh.sftp.chmod",
      archive: "ssh.sftp.archive",
    },
  } as TestFileBrowserConfig;
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

function mountFileBrowserWithConfirm(
  props: InstanceType<typeof FileBrowserPanel>["$props"],
) {
  const host = document.createElement("div");
  document.body.appendChild(host);
  return mount(
    {
      render: () => h("div", [h(FileBrowserPanel, props), h(ConfirmDialog)]),
    },
    { attachTo: host },
  );
}

async function setBodyInput(placeholder: string, value: string): Promise<void> {
  const input = document.body.querySelector(
    `input[placeholder="${placeholder}"]`,
  ) as HTMLInputElement;
  input.value = value;
  input.dispatchEvent(new Event("input"));
  await nextTick();
}

async function dirtyReadme(w: ReturnType<typeof mount>): Promise<void> {
  await w
    .findAll("li button")
    .find((b) => b.text().includes("README.md"))!
    .trigger("click");
  await flushPromises();
  await vi.waitFor(() => expect(codeMirrorEditors.length).toBeGreaterThan(0));
  const editor = codeMirrorEditors.at(-1)!;
  editor.setValue("# Unsaved");
  editor.emitChange();
  await nextTick();
  expect(w.text()).toContain("Unsaved");
}

describe("FileBrowserPanel", () => {
  it("lists a directory (dirs first) and previews a selected text file", async () => {
    const w = mount(FileBrowserPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "ssh.sftp.list", params: { path: "/" } },
        config: { pathParam: "path", routes: { read: "ssh.sftp.read" } },
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
        config: { pathParam: "path", routes: { read: "ssh.sftp.read" } },
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
        config: { pathParam: "path", routes: { read: "ssh.sftp.read" } },
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
        config: { pathParam: "path", routes: { read: "ssh.sftp.read" } },
      },
    });
    await flushPromises();
    await w.get('[aria-label="Open etc"]').trigger("click");
    await flushPromises();
    expect(w.text()).toContain("hosts");
  });

  it("keeps editing when directory navigation is canceled with unsaved file changes", async () => {
    const w = mountFileBrowserWithConfirm({
      connectionId: "c1",
      source: { routeId: "ssh.sftp.list", params: { path: "/" } },
      config: writableConfig(),
    });
    await flushPromises();
    await dirtyReadme(w);

    await w.get('[aria-label="Open etc"]').trigger("click");
    await flushPromises();
    bodyButton("Keep editing")!.click();
    await flushPromises();

    expect(w.text()).toContain("README.md");
    expect(w.text()).toContain("Unsaved");
    expect(w.text()).not.toContain("hosts");
    w.unmount();
  });

  it("discards unsaved file changes before directory navigation", async () => {
    const w = mountFileBrowserWithConfirm({
      connectionId: "c1",
      source: { routeId: "ssh.sftp.list", params: { path: "/" } },
      config: writableConfig(),
    });
    await flushPromises();
    await dirtyReadme(w);

    await w.get('[aria-label="Open etc"]').trigger("click");
    await flushPromises();
    bodyButton("Discard changes")!.click();
    await flushPromises();

    expect(w.text()).toContain("hosts");
    expect(w.text()).not.toContain("Unsaved");
    w.unmount();
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
        config: { pathParam: "path", routes: { read: "ssh.sftp.read" } },
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
        config: { pathParam: "path", routes: { read: "ssh.sftp.read" } },
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

  it("blocks oversized uploads in the file browser UI instead of PrimeVue's upload message area", async () => {
    const calls: { url: string; init?: RequestInit }[] = [];
    vi.unstubAllGlobals();
    installFetch((url, init) => {
      calls.push({ url, init });
      return { body: { items: rootEntries, nextCursor: "" } };
    });

    const config = writableConfig();
    config.upload = { ...config.upload, maxBytes: 50 };
    const w = mount(FileBrowserPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "ssh.sftp.list", params: { path: "/" } },
        config,
      },
    });
    await flushPromises();

    const largeFile = new File(["x".repeat(64)], "UpworkSetup64.exe", {
      type: "application/octet-stream",
    });
    w.findComponent(FileToolbar).vm.$emit("upload", { files: [largeFile] });
    await flushPromises();

    expect(w.text()).toContain("Upload blocked");
    expect(w.text()).toContain("UpworkSetup64.exe is 64 B");
    expect(w.text()).toContain("Maximum upload size is 50 B");
    expect(calls.some((c) => c.url.includes("ssh.sftp.upload"))).toBe(false);

    await w
      .findComponent({ name: "AppAlert" })
      .vm.$emit("close", new Event("close"));
    await nextTick();
    expect(w.text()).not.toContain("Upload blocked");
  });

  it("shows the selection bar once an entry is selected via its checkbox", async () => {
    const w = mount(FileBrowserPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "ssh.sftp.list", params: { path: "/" } },
        config: bulkConfig(),
      },
    });
    await flushPromises();

    // No selection yet → no selection bar.
    expect(w.text()).not.toContain("selected");

    await w
      .findAll('input[type="checkbox"]')
      .find((c) => c.attributes("aria-label") === "Select README.md")!
      .trigger("change");
    await flushPromises();

    expect(w.text()).toContain("1 selected");
    // Bulk buttons gated on configured slots are present.
    expect(w.findAll("button").some((b) => b.text().includes("Move"))).toBe(
      true,
    );
    expect(w.findAll("button").some((b) => b.text().includes("Copy"))).toBe(
      true,
    );
    expect(
      w.findAll("button").some((b) => b.text().includes("Download zip")),
    ).toBe(true);
  });

  it("hides the toolbar single-delete while a multi-selection is active (no double delete)", async () => {
    const w = mount(FileBrowserPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "ssh.sftp.list", params: { path: "/" } },
        config: bulkConfig(),
      },
    });
    await flushPromises();

    // Focus an entry so the toolbar's single-delete would otherwise show.
    await w
      .findAll('input[type="checkbox"]')
      .find((c) => c.attributes("aria-label") === "Select README.md")!
      .trigger("change");
    await flushPromises();

    const deleteButtons = w
      .findAll("button")
      .filter(
        (b) =>
          b.attributes("aria-label") === "Delete selected item" ||
          b.text().trim() === "Delete",
      );
    expect(deleteButtons).toHaveLength(1);
    // The remaining one is the selection bar's bulk delete.
    expect(
      w
        .findAll("button")
        .some((b) => b.attributes("aria-label") === "Delete selected item"),
    ).toBe(false);
  });

  it("hides a bulk button when its route id is absent", async () => {
    const config = bulkConfig();
    config.routes!.move = undefined;
    const w = mount(FileBrowserPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "ssh.sftp.list", params: { path: "/" } },
        config,
      },
    });
    await flushPromises();
    await w
      .findAll('input[type="checkbox"]')
      .find((c) => c.attributes("aria-label") === "Select README.md")!
      .trigger("change");
    await flushPromises();

    expect(w.text()).toContain("1 selected");
    // Move route removed → no Move button; Copy route still present.
    expect(w.findAll("button").some((b) => b.text().includes("Move"))).toBe(
      false,
    );
    expect(w.findAll("button").some((b) => b.text().includes("Copy"))).toBe(
      true,
    );
  });

  it("runs bulk delete once per selected entry", async () => {
    const calls: { url: string; init?: RequestInit }[] = [];
    vi.unstubAllGlobals();
    installFetch((url, init) => {
      calls.push({ url, init });
      if (init?.method && init.method !== "GET") return { body: { ok: true } };
      return { body: { items: rootEntries, nextCursor: "" } };
    });

    const w = mount(FileBrowserPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "ssh.sftp.list", params: { path: "/" } },
        config: bulkConfig(),
      },
    });
    await flushPromises();

    // Select both entries via the select-all checkbox in the toolbar... instead
    // toggle each entry's checkbox.
    for (const label of ["Select etc", "Select README.md"]) {
      await w
        .findAll('input[type="checkbox"]')
        .find((c) => c.attributes("aria-label") === label)!
        .trigger("change");
    }
    await flushPromises();
    expect(w.text()).toContain("2 selected");

    await w
      .findAll("button")
      .find((b) => b.text().includes("Delete"))!
      .trigger("click");
    await flushPromises();
    bodyButton("Delete")!.click();
    await flushPromises();

    const deletes = calls.filter(
      (c) => c.url.includes("ssh.sftp.delete") && c.init?.method === "DELETE",
    );
    expect(deletes.length).toBe(2);
    const bodies = deletes.map((d) => JSON.parse(String(d.init?.body)).path);
    expect(bodies.sort()).toEqual(["/README.md", "/etc"]);
  });

  it("offers permission presets before applying chmod", async () => {
    const calls: { url: string; init?: RequestInit }[] = [];
    vi.unstubAllGlobals();
    installFetch((url, init) => {
      calls.push({ url, init });
      if (init?.method && init.method !== "GET") return { body: { ok: true } };
      return { body: { items: rootEntries, nextCursor: "" } };
    });

    const w = mount(FileBrowserPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "ssh.sftp.list", params: { path: "/" } },
        config: bulkConfig(),
      },
    });
    await flushPromises();

    await w
      .findAll('input[type="checkbox"]')
      .find((c) => c.attributes("aria-label") === "Select README.md")!
      .trigger("change");
    await flushPromises();

    await w
      .findAll("button")
      .find((b) => b.text().includes("Permissions"))!
      .trigger("click");
    await flushPromises();

    expect(document.body.textContent).toContain(
      "Owner read/write, everyone read",
    );
    bodyButton("Apply")!.click();
    await flushPromises();

    const chmod = calls.find((c) => c.url.includes("ssh.sftp.chmod"))!;
    expect(JSON.parse(String(chmod.init?.body))).toEqual({
      paths: ["/README.md"],
      mode: "0644",
    });
  });

  it("opens a move dialog with the current folder as destination", async () => {
    vi.unstubAllGlobals();
    installFetch((_url, init) => {
      if (init?.method && init.method !== "GET") return { body: { ok: true } };
      return { body: { items: rootEntries, nextCursor: "" } };
    });

    const w = mount(FileBrowserPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "ssh.sftp.list", params: { path: "/" } },
        config: bulkConfig(),
      },
    });
    await flushPromises();
    await w
      .findAll('input[type="checkbox"]')
      .find((c) => c.attributes("aria-label") === "Select README.md")!
      .trigger("change");
    await flushPromises();

    await w
      .findAll("button")
      .find((b) => b.text().includes("Move"))!
      .trigger("click");
    await flushPromises();

    expect(document.body.textContent).toContain("Move 1 item");
    const input = document.body.querySelector(
      'input[aria-label="Operation destination"]',
    ) as HTMLInputElement;
    expect(input.value).toBe("/");
  });
});
