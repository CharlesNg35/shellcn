import { describe, it, expect, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { installFetch } from "@/test/fetchMock";
import FormPanel from "./FormPanel.vue";

afterEach(() => vi.unstubAllGlobals());

describe("FormPanel", () => {
  it("submits schema values through the manifest-declared route", async () => {
    const calls: { url: string; init?: RequestInit }[] = [];
    installFetch((url, init) => {
      calls.push({ url, init });
      if (init?.method && init.method !== "GET") return { body: { ok: true } };
      return {
        body: {
          groups: [
            {
              name: "Hardware",
              fields: [
                {
                  key: "name",
                  label: "Name",
                  type: "text",
                  required: true,
                  default: "web",
                },
              ],
            },
          ],
        },
      };
    });

    const w = mount(FormPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "proxmox.vm.config", params: { node: "pve1" } },
        resource: { kind: "vm", namespace: "pve1", name: "web", uid: "101" },
        config: {
          submitRouteId: "proxmox.vm.config.update",
          submitMethod: "PATCH",
          params: { node: "${resource.namespace}", vmid: "${resource.uid}" },
          submitLabel: "Apply",
        },
      },
    });
    await flushPromises();

    await w.get("input").setValue("api");
    await w.get("form").trigger("submit");
    await flushPromises();

    const submit = calls.find((c) =>
      c.url.includes("proxmox.vm.config.update"),
    )!;
    expect(submit.url).toContain("p.node=pve1");
    expect(submit.url).toContain("p.vmid=101");
    expect(submit.init?.method).toBe("PATCH");
    expect(JSON.parse(String(submit.init?.body))).toEqual({ name: "api" });
  });
});
