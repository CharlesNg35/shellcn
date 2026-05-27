import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { setActivePinia, createPinia } from "pinia";
import Dialog from "primevue/dialog";
import { installFetch } from "../../test/fetchMock";
import { useDockStore } from "../../stores/dock";
import ActionBar from "./ActionBar.vue";
import type { Action } from "../../types/projection";

const stop: Action = {
  id: "stop",
  label: "Stop",
  routeId: "docker.container.stop",
  method: "POST",
  risk: "destructive",
  requiresConfirm: true,
  confirmText: "Really stop it?",
};
const snapshot: Action = {
  id: "snap",
  label: "Snapshot",
  routeId: "vm.snapshot",
  method: "POST",
  risk: "write",
  requiresConfirm: false,
  input: {
    groups: [
      {
        name: "Snapshot",
        fields: [{ key: "name", label: "Name", type: "text", required: true }],
      },
    ],
  },
};

let posted: { url: string; body?: unknown }[] = [];
beforeEach(() => {
  setActivePinia(createPinia());
  posted = [];
  installFetch((url, init) => {
    posted.push({
      url,
      body: init?.body ? JSON.parse(init.body as string) : undefined,
    });
    return { body: { ok: true } };
  });
});
afterEach(() => vi.unstubAllGlobals());

function bodyButton(text: string): HTMLButtonElement | undefined {
  return [...document.body.querySelectorAll("button")].find(
    (b) => b.textContent?.trim() === text,
  ) as HTMLButtonElement | undefined;
}

describe("ActionBar", () => {
  it("uses the bounded dialog root for action forms", () => {
    const w = mount(ActionBar, {
      props: {
        connectionId: "c1",
        actions: [snapshot],
      },
    });
    const pt = w.findComponent(Dialog).props("pt") as { root: string };
    expect(pt.root).toContain("max-w-2xl");
    expect(pt.root).toContain("max-h-[calc(100vh-2rem)]");
    expect(pt.root).toContain("flex-col");
    w.unmount();
  });

  it("gates a destructive action behind a confirm dialog", async () => {
    const w = mount(ActionBar, {
      attachTo: document.body,
      props: {
        connectionId: "c1",
        actions: [stop],
        resource: { kind: "container", name: "x", uid: "c-1" },
      },
    });
    await w.find("button").trigger("click");
    await flushPromises();
    // PrimeVue Dialog teleports to body.
    expect(document.body.textContent).toContain("Really stop it?");
    expect(posted).toHaveLength(0); // not yet run

    bodyButton("Confirm")!.click();
    await flushPromises();
    expect(posted).toHaveLength(1);
    const url = new URL(posted[0].url, "http://localhost");
    expect(url.searchParams.get("p.uid")).toBe("c-1");
    expect(url.searchParams.get("p.name")).toBe("x");
    expect(w.emitted("done")).toBeTruthy();
    w.unmount();
  });

  it("renders an input form for an action with input and submits the body", async () => {
    const w = mount(ActionBar, {
      attachTo: document.body,
      props: {
        connectionId: "c1",
        actions: [snapshot],
        resource: { kind: "vm", name: "v", uid: "101" },
      },
    });
    await w.find("button").trigger("click");
    await flushPromises();
    const input = document.body.querySelector("input") as HTMLInputElement;
    input.value = "nightly";
    input.dispatchEvent(new Event("input"));
    (document.body.querySelector("form") as HTMLFormElement).dispatchEvent(
      new Event("submit", { cancelable: true, bubbles: true }),
    );
    await flushPromises();
    expect(posted).toHaveLength(1);
    expect((posted[0].body as { name: string }).name).toBe("nightly");
    w.unmount();
  });

  it("routes an open=dock action into the dock store instead of running it", async () => {
    const dockAction: Action = {
      id: "logs",
      label: "Logs in dock",
      routeId: "docker.container.logs",
      method: "WS",
      risk: "safe",
      requiresConfirm: false,
      open: "dock",
      panel: "log_stream",
      params: { id: "${resource.uid}" },
    };
    const w = mount(ActionBar, {
      attachTo: document.body,
      props: {
        connectionId: "c1",
        actions: [dockAction],
        resource: { kind: "container", name: "web", uid: "c-9" },
      },
    });
    await w.find("button").trigger("click");
    await flushPromises();
    const dock = useDockStore();
    const items = dock.state("c1").items;
    expect(posted).toHaveLength(0); // the route is NOT executed
    expect(items).toHaveLength(1);
    expect(items[0].panel).toBe("log_stream");
    expect(items[0].source.routeId).toBe("docker.container.logs");
    w.unmount();
  });

  it("routes an open=dialog action into the dock dialog slot", async () => {
    const dialogAction: Action = {
      id: "peek",
      label: "Peek logs",
      routeId: "docker.container.logs",
      method: "WS",
      risk: "safe",
      requiresConfirm: false,
      open: "dialog",
      panel: "log_stream",
    };
    const w = mount(ActionBar, {
      attachTo: document.body,
      props: {
        connectionId: "c1",
        actions: [dialogAction],
        resource: { kind: "container", name: "web", uid: "c-9" },
      },
    });
    await w.find("button").trigger("click");
    await flushPromises();
    const dock = useDockStore();
    expect(posted).toHaveLength(0);
    expect(dock.state("c1").dialog?.panel).toBe("log_stream");
    w.unmount();
  });

  it("uses declarative action params when provided", async () => {
    const action: Action = {
      ...snapshot,
      input: undefined,
      params: { node: "${resource.namespace}", vmid: "${resource.uid}" },
    };
    const w = mount(ActionBar, {
      attachTo: document.body,
      props: {
        connectionId: "c1",
        actions: [action],
        resource: { kind: "vm", namespace: "pve1", name: "web", uid: "101" },
      },
    });
    await w.find("button").trigger("click");
    await flushPromises();
    const url = new URL(posted[0].url, "http://localhost");
    expect(url.searchParams.get("p.node")).toBe("pve1");
    expect(url.searchParams.get("p.vmid")).toBe("101");
    w.unmount();
  });
});
