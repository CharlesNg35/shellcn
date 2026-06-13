import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { setActivePinia, createPinia } from "pinia";
import { installFetch } from "../test/fetchMock";
import { useConnectionsStore } from "./connections";
import { Layout } from "../types/projection";
import type { ConnectionSummary } from "../types/projection";

const connections: ConnectionSummary[] = [
  {
    id: "a",
    name: "alpha",
    protocol: "ssh",
    transport: "direct",
    online: true,
  },
  {
    id: "b",
    name: "beta",
    protocol: "docker",
    transport: "agent",
    online: false,
  },
];
const plugins = [
  {
    name: "ssh",
    title: "SSH",
    icon: { type: "lucide", value: "terminal" },
    category: {
      key: "shell",
      label: "Shell & terminal",
      icon: { type: "lucide", value: "terminal" },
      order: 10,
    },
  },
];
const sshProjection = {
  apiVersion: 1,
  name: "ssh",
  version: "0.1.0",
  title: "SSH",
  description: "",
  icon: { type: "lucide", value: "terminal" },
  category: plugins[0].category,
  config: { groups: [] },
  capabilities: [],
  supportedTransports: ["direct"],
  layout: Layout.Tabs,
};

beforeEach(() => {
  setActivePinia(createPinia());
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("connections store", () => {
  it("loads connections and plugins", async () => {
    installFetch((url) => {
      if (url.endsWith("/api/connections")) return { body: connections };
      if (url.endsWith("/api/connection-folders")) return { body: [] };
      if (url.endsWith("/api/plugins")) return { body: plugins };
      return { status: 404, body: { error: "nope" } };
    });
    const store = useConnectionsStore();
    await store.load();
    expect(store.loaded).toBe(true);
    expect(store.connections).toHaveLength(2);
    expect(store.byId("b")?.name).toBe("beta");
  });

  it("caches a projection so it is fetched once", async () => {
    const fetchFn = installFetch((url) => {
      if (url.endsWith("/api/plugins/ssh")) return { body: sshProjection };
      return { body: [] };
    });
    const store = useConnectionsStore();
    const first = await store.projection("ssh");
    const second = await store.projection("ssh");
    expect(first).toBe(second);
    const projectionCalls = fetchFn.mock.calls.filter(([u]) =>
      String(u).endsWith("/api/plugins/ssh"),
    );
    expect(projectionCalls).toHaveLength(1);
  });

  it("refreshes plugin summaries and clears cached projections", async () => {
    const dockerPlugin = {
      ...plugins[0],
      name: "docker",
      title: "Docker",
    };
    installFetch((url) => {
      if (url.endsWith("/api/plugins/ssh")) return { body: sshProjection };
      if (url.endsWith("/api/plugins")) return { body: [dockerPlugin] };
      return { body: [] };
    });
    const store = useConnectionsStore();

    await store.projection("ssh");
    expect(store.projections.ssh).toBeTruthy();

    await store.refreshPlugins();
    expect(store.plugins).toEqual([dockerPlugin]);
    expect(store.projections).toEqual({});
  });

  it("patches a renamed folder in local sidebar state", async () => {
    installFetch((url, init) => {
      if (
        url.endsWith("/api/connection-folders/f1") &&
        init?.method === "PUT"
      ) {
        return {
          body: {
            id: "f1",
            name: "Production EU",
            color: "teal",
            sortOrder: 9,
          },
        };
      }
      return { status: 404, body: { error: "nope" } };
    });
    const store = useConnectionsStore();
    store.folders = [
      {
        id: "f1",
        parentId: "root",
        name: "Production",
        color: "blue",
        sortOrder: 9,
      },
    ];

    const folder = await store.updateFolder("f1", {
      name: "Production EU",
      color: "teal",
    });

    expect(folder).toMatchObject({
      id: "f1",
      parentId: "root",
      name: "Production EU",
      color: "teal",
      sortOrder: 9,
    });
    expect(store.folders[0]).toEqual(folder);
  });

  it("patches a renamed connection in local sidebar state", async () => {
    installFetch((url, init) => {
      if (url.endsWith("/api/connections/a") && init?.method === "PUT") {
        return {
          body: {
            id: "a",
            name: "alpha prod",
            protocol: "ssh",
            transport: "agent",
            config: {},
            secrets: {},
            recording: { terminal: "manual" },
            aiMode: "read_only",
            aiAllowDestructive: false,
          },
        };
      }
      return { status: 404, body: { error: "nope" } };
    });
    const store = useConnectionsStore();
    store.connections = [
      {
        ...connections[0],
        folderId: "f1",
        sortOrder: 2,
        canManage: true,
      },
    ];

    await store.updateConnection("a", {
      name: "alpha prod",
      transport: "agent",
      config: {},
      preserveCredentials: [],
      recording: { terminal: "manual" },
      aiMode: "read_only",
      aiAllowDestructive: false,
    });

    expect(store.byId("a")).toMatchObject({
      id: "a",
      name: "alpha prod",
      protocol: "ssh",
      transport: "agent",
      folderId: "f1",
      sortOrder: 2,
      canManage: true,
      config: {},
      recording: { terminal: "manual" },
      aiMode: "read_only",
      aiAllowDestructive: false,
    });
  });
});
