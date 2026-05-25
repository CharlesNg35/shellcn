import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { installFetch } from "../test/fetchMock";
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
afterEach(() => vi.unstubAllGlobals());

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
    const dir = w.findAll("li button").find((b) => b.text().includes("etc"));
    await dir!.trigger("click");
    await flushPromises();
    expect(w.text()).toContain("hosts");
  });
});
