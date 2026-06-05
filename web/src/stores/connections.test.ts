import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { setActivePinia, createPinia } from "pinia";
import { installFetch } from "../test/fetchMock";
import { useConnectionsStore } from "./connections";
import { Layout } from "../types/projection";

const connections = [
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
});
