import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { defineComponent, ref } from "vue";
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

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("ConnectPanel", () => {
  beforeEach(() => {
    installFetch((url) => {
      if (url.includes("/agent/state")) return { body: { status: "pending" } };
      return { body: {} };
    });
  });

  it("direct connection: Connect is enabled without fetching raw config", async () => {
    const calls: string[] = [];
    vi.unstubAllGlobals();
    installFetch((url) => {
      calls.push(url);
      if (url.includes("/agent/state")) return { body: { status: "pending" } };
      return { body: {} };
    });
    const w = mount(ConnectPanel, {
      props: { connectionId: "c1", connection: direct },
    });
    await flushPromises();
    expect(w.text()).not.toContain("Agent");
    expect(w.text()).not.toContain("Direct connection");
    expect(w.text()).not.toContain("Credential id");
    expect(calls.some((url) => url.match(/\/connections\/[^/]+$/))).toBe(false);
    expect(connectDisabled(w)).toBe(false);
    await connectBtn(w)?.trigger("click");
    expect(w.emitted("connect")).toBeTruthy();
  });

  it("agent: shows a neutral checking state before the first state resolves (no flash)", () => {
    const w = mount(ConnectPanel, {
      props: { connectionId: "edge", connection: agent },
    });
    // Synchronously after mount the first /agent/state hasn't resolved yet.
    expect(w.text()).toContain("Checking agent");
    expect(w.text()).not.toContain("Waiting for agent");
    expect(
      w
        .findAllComponents(Button)
        .find((b) => b.text().includes("Set up agent")),
    ).toBeFalsy();
    w.unmount();
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
      return { body: {} };
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
    w.unmount();
  });

  it("agent offline after being online disables Connect again", async () => {
    let calls = 0;
    installFetch((url) => {
      if (url.includes("/agent/state")) {
        calls++;
        return {
          body: { status: calls <= 1 ? "online" : "offline" },
        };
      }
      return { body: {} };
    });
    vi.useFakeTimers();

    const w = mount(ConnectPanel, {
      props: { connectionId: "edge", connection: agent },
    });
    await flushPromises();
    expect(w.text()).toContain("Agent connected");
    expect(connectDisabled(w)).toBe(false);

    await vi.advanceTimersByTimeAsync(2000);
    await flushPromises();
    expect(w.text()).toContain("Agent offline");
    expect(connectDisabled(w)).toBe(true);
    w.unmount();
  });

  it("shows the backend connect error inline", () => {
    const w = mount(ConnectPanel, {
      props: {
        connectionId: "c1",
        connection: direct,
        errorMessage: "ssh: unable to authenticate",
      },
    });

    expect(w.find('[role="alert"]').text()).toContain("Could not connect");
    expect(w.find('[role="alert"]').text()).toContain(
      "ssh: unable to authenticate",
    );
  });

  it("stops polling an old agent connection when the panel is reused", async () => {
    const calls: string[] = [];
    installFetch((url) => {
      if (url.includes("/agent/state")) {
        calls.push(url);
        return { body: { status: "pending" } };
      }
      return { body: {} };
    });
    vi.useFakeTimers();

    const w = mount(ConnectPanel, {
      props: { connectionId: "edge", connection: agent },
    });
    await flushPromises();
    expect(calls).toHaveLength(1);

    await w.setProps({ connectionId: "c1", connection: direct });
    await flushPromises();
    await vi.advanceTimersByTimeAsync(4000);
    await flushPromises();

    expect(calls).toHaveLength(1);
    w.unmount();
  });

  it("pauses agent polling while deactivated under KeepAlive", async () => {
    const calls: string[] = [];
    installFetch((url) => {
      if (url.includes("/agent/state")) {
        calls.push(url);
        return { body: { status: "pending" } };
      }
      return { body: {} };
    });
    vi.useFakeTimers();

    const Host = defineComponent({
      components: { ConnectPanel },
      setup() {
        const show = ref(true);
        return { show, agent };
      },
      template:
        "<KeepAlive><ConnectPanel v-if='show' connection-id='edge' :connection='agent' /></KeepAlive>",
    });
    const w = mount(Host);
    await flushPromises();
    expect(calls).toHaveLength(1);

    w.vm.show = false;
    await flushPromises();
    await vi.advanceTimersByTimeAsync(4000);
    await flushPromises();
    expect(calls).toHaveLength(1);

    w.vm.show = true;
    await flushPromises();
    expect(calls).toHaveLength(2);

    w.unmount();
  });
});
