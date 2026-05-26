import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import Button from "primevue/button";
import { installFetch } from "../../test/fetchMock";
import ConnectPanel from "./ConnectPanel.vue";
import type { ConnectionSummary } from "../../types/projection";

const direct: ConnectionSummary = {
  id: "c1",
  name: "prod-web",
  protocol: "ssh",
  transport: "direct",
};
const agent: ConnectionSummary = {
  id: "edge",
  name: "edge-docker",
  protocol: "docker",
  transport: "agent",
};

function connectBtn(w: ReturnType<typeof mount>) {
  return w.findAllComponents(Button).find((b) => b.text().includes("Connect"));
}
function connectDisabled(w: ReturnType<typeof mount>): boolean {
  const el = connectBtn(w)?.find("button").element as
    | HTMLButtonElement
    | undefined;
  return el?.disabled ?? false;
}

afterEach(() => vi.unstubAllGlobals());

describe("ConnectPanel", () => {
  beforeEach(() => {
    installFetch((url) => {
      if (url.includes("/agent/state")) return { body: { status: "pending" } };
      if (url.match(/\/connections\/[^/]+$/)) {
        return {
          body: {
            id: "c1",
            name: "prod-web",
            protocol: "ssh",
            transport: "direct",
            ownerId: "u1",
            config: { host: "10.0.0.5", port: 22, user: "root" },
            secrets: {},
          },
        };
      }
      return { body: {} };
    });
  });

  it("direct connection: Connect is enabled and shows details", async () => {
    const w = mount(ConnectPanel, {
      props: { connectionId: "c1", connection: direct },
    });
    await flushPromises();
    expect(w.text()).not.toContain("Agent");
    expect(w.text()).toContain("Host");
    expect(w.text()).toContain("10.0.0.5");
    expect(connectDisabled(w)).toBe(false);
    await connectBtn(w)?.trigger("click");
    expect(w.emitted("connect")).toBeTruthy();
  });

  it("agent offline: Connect is gated and Set up agent enrolls", async () => {
    const w = mount(ConnectPanel, {
      props: { connectionId: "edge", connection: agent },
    });
    await flushPromises();
    expect(w.text()).toContain("Waiting for agent");
    expect(connectDisabled(w)).toBe(true);
    const setup = w
      .findAllComponents(Button)
      .find((b) => b.text().includes("Set up agent"));
    expect(setup).toBeTruthy();
    await setup?.trigger("click");
    expect(w.emitted("enroll")).toBeTruthy();
  });

  it("agent online: Connect becomes enabled", async () => {
    let calls = 0;
    installFetch((url) => {
      if (url.includes("/agent/state")) {
        calls++;
        return { body: { status: calls <= 1 ? "pending" : "online" } };
      }
      return {
        body: {
          id: "edge",
          name: "edge",
          protocol: "docker",
          transport: "agent",
          ownerId: "u1",
          config: {},
          secrets: {},
        },
      };
    });
    vi.useFakeTimers();
    const w = mount(ConnectPanel, {
      props: { connectionId: "edge", connection: agent },
    });
    await flushPromises();
    expect(connectDisabled(w)).toBe(true);
    await vi.advanceTimersByTimeAsync(2000);
    await flushPromises();
    expect(w.text()).toContain("Agent connected");
    expect(connectDisabled(w)).toBe(false);
    vi.useRealTimers();
  });
});
