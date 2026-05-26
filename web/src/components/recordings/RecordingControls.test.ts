import { describe, it, expect, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { installFetch } from "../../test/fetchMock";
import RecordingControls from "./RecordingControls.vue";

afterEach(() => {
  document.body.innerHTML = "";
  vi.unstubAllGlobals();
});

describe("RecordingControls", () => {
  it("resumes manual recording when a stream reconnects", async () => {
    const calls: Array<{ url: string; body: unknown }> = [];
    installFetch((_url, init) => {
      calls.push({
        url: String(_url),
        body: JSON.parse(String(init?.body ?? "{}")) as unknown,
      });
      return { body: { ok: true } };
    });

    const w = mount(RecordingControls, {
      props: {
        connectionId: "c1",
        source: { routeId: "ssh.shell", params: { rows: "24" } },
        descriptor: {
          class: "terminal",
          policy: "manual",
          authoritative: true,
        },
        streamStatus: "open",
      },
    });

    await w.get("button").trigger("click");
    await flushPromises();
    expect(calls.at(-1)?.body).toMatchObject({ action: "start" });
    expect(w.text()).toContain("REC");

    await w.setProps({ streamStatus: "error" });
    await flushPromises();
    expect(w.text()).not.toContain("REC");

    await w.setProps({ streamStatus: "open" });
    await flushPromises();
    expect(calls.at(-1)?.body).toMatchObject({ action: "start" });
    expect(
      calls.filter(
        (call) => (call.body as { action?: string }).action === "start",
      ),
    ).toHaveLength(2);
  });

  it("does not resume after the user explicitly stops recording", async () => {
    const actions: string[] = [];
    installFetch((_url, init) => {
      actions.push(
        (JSON.parse(String(init?.body ?? "{}")) as { action: string }).action,
      );
      return { body: { ok: true } };
    });

    const w = mount(RecordingControls, {
      props: {
        connectionId: "c1",
        source: { routeId: "ssh.shell" },
        descriptor: {
          class: "terminal",
          policy: "manual",
          authoritative: true,
        },
        streamStatus: "open",
      },
    });

    await w.get("button").trigger("click");
    await flushPromises();
    await w
      .findAll("button")
      .find((button) => button.text().includes("Stop"))!
      .trigger("click");
    await flushPromises();

    await w.setProps({ streamStatus: "error" });
    await w.setProps({ streamStatus: "open" });
    await flushPromises();

    expect(actions).toEqual(["start", "stop"]);
  });
});
