import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { installFetch } from "../../test/fetchMock";
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
afterEach(() => vi.unstubAllGlobals());

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
});
