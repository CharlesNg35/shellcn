import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { nextTick } from "vue";
import { installFetch } from "../../test/fetchMock";
import FileBrowserPanel from "./FileBrowserPanel.vue";

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
    expect(w.text()).toContain("README.md");

    const file = items.find((b) => b.text().includes("README.md"));
    await file!.trigger("click");
    await flushPromises();
    expect(w.text()).toContain("# Hello");
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
    expect(document.body.textContent).toContain("# Hello");
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

    await panelButton(w, "Rename").trigger("click");
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
    const editor = w.get("textarea");
    await editor.setValue("# Updated");
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
    await panelButton(w, "Delete").trigger("click");
    bodyButton("Delete")!.click();
    await flushPromises();

    const del = calls.find((c) => c.url.includes("ssh.sftp.delete"))!;
    expect(del.url).toContain("p.path=%2FREADME.md");
    expect(del.init?.method).toBe("DELETE");
    expect(JSON.parse(String(del.init?.body))).toEqual({ path: "/README.md" });
  });
});
