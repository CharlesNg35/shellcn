import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { defineComponent, ref } from "vue";
import { installFetch } from "@/test/fetchMock";
import EnrollPanel from "./EnrollPanel.vue";

let stateCalls = 0;
beforeEach(() => {
  stateCalls = 0;
  installFetch((url) => {
    if (url.includes("/agent/state")) {
      stateCalls++;
      // pending on first (onMounted) check, online afterwards
      return {
        body:
          stateCalls <= 1
            ? { status: "pending", message: "waiting" }
            : { status: "online" },
      };
    }
    if (url.includes("/agent/enrollments")) {
      return {
        body: {
          enrollmentId: "enr-1",
          expiresAt: "",
          artifacts: [
            {
              label: "Container",
              kind: "container-run",
              command: "run shellcn-agent",
            },
          ],
        },
      };
    }
    return { body: {} };
  });
});
afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("EnrollPanel", () => {
  it("generates an install command and transitions to online", async () => {
    const w = mount(EnrollPanel, { props: { connectionId: "edge" } });
    await flushPromises();
    expect(w.text()).toContain("Connect the agent");

    const generate = w
      .findAll("button")
      .find((b) => b.text().includes("Generate"));
    await generate!.trigger("click");
    await flushPromises();

    expect(w.text()).toContain("run shellcn-agent");
    // the immediate post-enroll status check returns online
    expect(w.emitted("online")).toBeTruthy();
    expect(w.text()).toContain("Agent online");
  });

  it("exposes a copy button per artifact", async () => {
    const w = mount(EnrollPanel, { props: { connectionId: "edge" } });
    await flushPromises();
    await w
      .findAll("button")
      .find((b) => b.text().includes("Generate"))!
      .trigger("click");
    await flushPromises();
    expect(w.findAll("button").some((b) => b.text().includes("Copy"))).toBe(
      true,
    );
  });

  it("keeps polling after the agent first comes online", async () => {
    vi.useFakeTimers();
    installFetch((url) => {
      if (url.includes("/agent/state")) {
        stateCalls++;
        return {
          body:
            stateCalls <= 1
              ? { status: "pending", message: "waiting" }
              : stateCalls <= 2
                ? { status: "online", message: "connected" }
                : { status: "offline", message: "offline" },
        };
      }
      return { body: {} };
    });

    const w = mount(EnrollPanel, { props: { connectionId: "edge" } });
    await flushPromises();
    expect(w.text()).toContain("Connect the agent");

    await vi.advanceTimersByTimeAsync(2000);
    await flushPromises();
    expect(w.text()).toContain("Agent online");
    expect(w.emitted("online")).toBeTruthy();

    await vi.advanceTimersByTimeAsync(2000);
    await flushPromises();
    expect(w.text()).toContain("Agent disconnected");
    expect(w.text()).toContain("offline");

    w.unmount();
  });

  it("pauses polling while deactivated under KeepAlive", async () => {
    vi.useFakeTimers();
    const Host = defineComponent({
      components: { EnrollPanel },
      setup() {
        const show = ref(true);
        return { show };
      },
      template:
        "<KeepAlive><EnrollPanel v-if='show' connection-id='edge' /></KeepAlive>",
    });
    const w = mount(Host);
    await flushPromises();
    expect(stateCalls).toBe(1);

    w.vm.show = false;
    await flushPromises();
    await vi.advanceTimersByTimeAsync(4000);
    await flushPromises();
    expect(stateCalls).toBe(1);

    w.vm.show = true;
    await flushPromises();
    expect(stateCalls).toBe(2);

    w.unmount();
  });
});
