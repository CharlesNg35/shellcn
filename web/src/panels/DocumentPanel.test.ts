import { describe, it, expect, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { installFetch } from "../test/fetchMock";
import DocumentPanel from "./DocumentPanel.vue";

afterEach(() => vi.unstubAllGlobals());

describe("DocumentPanel", () => {
  it("renders fetched JSON as an expandable tree and can switch to raw mode", async () => {
    installFetch(() => ({ body: { State: { Status: "running" } } }));

    const w = mount(DocumentPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "docker.container.inspect" },
      },
    });
    await flushPromises();

    expect(w.text()).toContain("State");
    expect(w.text()).toContain("Status");

    await w
      .findAll("button")
      .find((b) => b.text() === "Raw")!
      .trigger("click");
    expect(w.text()).toContain('"Status": "running"');
  });
});
