import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { setActivePinia, createPinia } from "pinia";
import Dialog from "primevue/dialog";
import { installFetch } from "@/test/fetchMock";
import { useDockStore } from "@/stores/dock";
import ActionBar from "./ActionBar.vue";
import { RiskLevel, type Action } from "@/types/projection";

const stop: Action = {
  id: "stop",
  label: "Stop",
  routeId: "docker.container.stop",
  method: "POST",
  risk: RiskLevel.Destructive,
  requiresConfirm: true,
  confirmText: "Really stop it?",
};
const snapshot: Action = {
  id: "snap",
  label: "Snapshot",
  routeId: "vm.snapshot",
  method: "POST",
  risk: RiskLevel.Write,
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

let posted: { url: string; body?: unknown; method?: string }[] = [];
let opened: string[] = [];
beforeEach(() => {
  setActivePinia(createPinia());
  posted = [];
  opened = [];
  vi.stubGlobal("open", (url: string) => {
    opened.push(url);
    return null;
  });
  installFetch((url, init) => {
    posted.push({
      url,
      method: init?.method,
      body: init?.body ? JSON.parse(init.body as string) : undefined,
    });
    return { body: { ok: true, url: "/proxy/opened/" } };
  });
});
afterEach(() => vi.unstubAllGlobals());

function bodyButton(text: string): HTMLButtonElement | undefined {
  return [...document.body.querySelectorAll("button")].find(
    (b) => b.textContent?.trim() === text,
  ) as HTMLButtonElement | undefined;
}

describe("ActionBar", () => {
  it("keeps action buttons visually small when overriding risk classes", () => {
    const w = mount(ActionBar, {
      props: {
        connectionId: "c1",
        actions: [stop],
      },
    });

    const button = w.get("button");
    expect(button.classes()).toContain("text-xs");
    expect(button.classes()).toContain("px-2.5");
    expect(button.classes()).toContain("py-1");
    expect(button.classes()).not.toContain("text-sm");
    expect(button.classes()).not.toContain("px-3");
    expect(button.classes()).not.toContain("py-1.5");
  });

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
      risk: RiskLevel.Safe,
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
      risk: RiskLevel.Safe,
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

  it("disables an action whose enabledWhen fails against the record (and runs it when it passes)", async () => {
    const start: Action = {
      id: "start",
      label: "Start",
      routeId: "docker.container.start",
      method: "POST",
      risk: RiskLevel.Write,
      requiresConfirm: false,
      params: { id: "${resource.uid}" },
      enabledWhen: { allOf: [{ field: "state", op: "eq", value: "exited" }] },
    };
    const resource = { kind: "container", name: "web", uid: "c-1" };

    const running = mount(ActionBar, {
      attachTo: document.body,
      props: {
        connectionId: "c1",
        actions: [start],
        resource,
        record: { state: "running" },
      },
    });
    const btn = running.find("button").element as HTMLButtonElement;
    expect(btn.disabled).toBe(true);
    await running.find("button").trigger("click");
    await flushPromises();
    expect(posted).toHaveLength(0); // disabled — never executed
    running.unmount();

    const stopped = mount(ActionBar, {
      attachTo: document.body,
      props: {
        connectionId: "c1",
        actions: [start],
        resource,
        record: { state: "exited" },
      },
    });
    expect((stopped.find("button").element as HTMLButtonElement).disabled).toBe(
      false,
    );
    await stopped.find("button").trigger("click");
    await flushPromises();
    expect(posted).toHaveLength(1);
    stopped.unmount();
  });

  it("hides an action whose visibleWhen fails against the record", () => {
    const clone: Action = {
      id: "clone",
      label: "Clone",
      routeId: "vm.clone",
      risk: RiskLevel.Write,
      requiresConfirm: false,
    };
    const start: Action = {
      id: "start",
      label: "Start",
      routeId: "vm.start",
      risk: RiskLevel.Write,
      requiresConfirm: false,
      visibleWhen: {
        allOf: [{ field: "template", op: "neq", value: true }],
      },
    };
    const w = mount(ActionBar, {
      attachTo: document.body,
      props: {
        connectionId: "c1",
        actions: [clone, start],
        record: { template: true },
      },
    });
    expect(bodyButton("Clone")).toBeTruthy();
    expect(bodyButton("Start")).toBeUndefined();
    w.unmount();
  });

  it("hides a row action requiring a non-empty field when the field is missing", () => {
    const deleteRow: Action = {
      id: "postgresql.table.row.delete",
      label: "Delete row",
      routeId: "postgresql.table.row.delete",
      risk: RiskLevel.Destructive,
      requiresConfirm: true,
      visibleWhen: { allOf: [{ field: "_key", op: "notEmpty" }] },
    };
    const w = mount(ActionBar, {
      attachTo: document.body,
      props: {
        connectionId: "c1",
        actions: [deleteRow],
        record: { name: "keyless" },
      },
    });

    expect(bodyButton("Delete row")).toBeUndefined();
    w.unmount();
  });

  it("clusters same-group actions into one dropdown and keeps ungrouped ones as buttons", () => {
    const grouped: Action[] = [
      {
        id: "open",
        label: "Open",
        routeId: "r.open",
        risk: RiskLevel.Safe,
        requiresConfirm: false,
      },
      {
        id: "start",
        label: "Start",
        routeId: "r.start",
        risk: RiskLevel.Write,
        requiresConfirm: false,
        group: "Lifecycle",
      },
      {
        id: "stop",
        label: "Stop",
        routeId: "r.stop",
        risk: RiskLevel.Destructive,
        requiresConfirm: false,
        group: "Lifecycle",
      },
    ];
    const w = mount(ActionBar, {
      attachTo: document.body,
      props: { connectionId: "c1", actions: grouped },
    });
    // One "Lifecycle" dropdown trigger + the standalone "Open" button; the
    // grouped actions are not rendered as their own bar buttons.
    expect(bodyButton("Lifecycle")).toBeTruthy();
    expect(bodyButton("Open")).toBeTruthy();
    expect(bodyButton("Start")).toBeUndefined();
    expect(bodyButton("Stop")).toBeUndefined();
    w.unmount();
  });

  it("collapses overflow buttons past the limit into a More menu", () => {
    const many: Action[] = Array.from({ length: 7 }, (_, i) => ({
      id: `a${i}`,
      label: `Act${i}`,
      routeId: `r.a${i}`,
      risk: RiskLevel.Safe,
      requiresConfirm: false,
    }));
    const w = mount(ActionBar, {
      attachTo: document.body,
      props: { connectionId: "c1", actions: many },
    });
    const more = [...document.body.querySelectorAll("button")].find(
      (b) => b.getAttribute("aria-label") === "More actions",
    );
    expect(more).toBeTruthy();
    // 4 inline action buttons + the More trigger = 5 chips; the rest move into
    // the overflow menu (not rendered until opened).
    expect(bodyButton("Act0")).toBeTruthy();
    expect(bodyButton("Act3")).toBeTruthy();
    expect(bodyButton("Act4")).toBeUndefined();
    w.unmount();
  });

  it("honors a tighter inline action limit", () => {
    const actions: Action[] = [
      {
        id: "inspect",
        label: "Inspect",
        routeId: "r.inspect",
        risk: RiskLevel.Safe,
        requiresConfirm: false,
      },
      {
        id: "rename",
        label: "Rename",
        routeId: "r.rename",
        risk: RiskLevel.Write,
        requiresConfirm: false,
      },
      {
        id: "delete",
        label: "Delete",
        routeId: "r.delete",
        risk: RiskLevel.Destructive,
        requiresConfirm: false,
      },
    ];
    const w = mount(ActionBar, {
      attachTo: document.body,
      props: { connectionId: "c1", actions, maxInline: 2 },
    });

    expect(bodyButton("Inspect")).toBeTruthy();
    expect(bodyButton("Rename")).toBeUndefined();
    expect(bodyButton("Delete")).toBeUndefined();
    expect(
      [...document.body.querySelectorAll("button")].some(
        (b) => b.getAttribute("aria-label") === "More actions",
      ),
    ).toBe(true);
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

  it("uses declarative action bodies and preserves raw row identity objects", async () => {
    const action: Action = {
      id: "postgresql.table.row.delete",
      label: "Delete row",
      routeId: "postgresql.table.row.delete",
      method: "DELETE",
      risk: RiskLevel.Destructive,
      requiresConfirm: false,
      params: {
        database: "${resource.scope}",
        schema: "${resource.namespace}",
        table: "${resource.name}",
      },
      body: { key: "${record._key}" },
    };
    const w = mount(ActionBar, {
      attachTo: document.body,
      props: {
        connectionId: "c1",
        actions: [action],
        resource: {
          kind: "table",
          scope: "app",
          namespace: "public",
          name: "users",
          uid: "app.public.users",
        },
        record: { id: 7, name: "alice", _key: { id: 7 } },
      },
    });

    await w.find("button").trigger("click");
    await flushPromises();

    bodyButton("Confirm")!.click();
    await flushPromises();

    const url = new URL(posted[0].url, "http://localhost");
    expect(url.searchParams.get("p.database")).toBe("app");
    expect(url.searchParams.get("p.schema")).toBe("public");
    expect(url.searchParams.get("p.table")).toBe("users");
    expect(posted[0].body).toEqual({ key: { id: 7 } });
    w.unmount();
  });

  it("hides single-row actions for multi-selection and keeps bulk actions", async () => {
    const rename: Action = {
      id: "postgresql.column.rename",
      label: "Rename",
      routeId: "postgresql.column.rename",
      method: "PATCH",
      risk: RiskLevel.Write,
      requiresConfirm: false,
      params: { name: "${record.name}" },
    };
    const deleteRow: Action = {
      id: "postgresql.table.row.delete",
      label: "Delete",
      routeId: "postgresql.table.row.delete",
      method: "DELETE",
      risk: RiskLevel.Destructive,
      requiresConfirm: false,
      params: { table: "${record.table}" },
      body: { key: "${record._key}" },
      bulk: true,
    };
    const w = mount(ActionBar, {
      attachTo: document.body,
      props: {
        connectionId: "c1",
        actions: [rename, deleteRow],
        records: [
          { table: "users", name: "id", _key: { id: 1 } },
          { table: "users", name: "name", _key: { id: 2 } },
        ],
      },
    });

    expect(bodyButton("Rename")).toBeUndefined();
    expect(bodyButton("Delete")).toBeTruthy();

    await w.find("button").trigger("click");
    await flushPromises();

    bodyButton("Confirm")!.click();
    await flushPromises();

    expect(posted).toHaveLength(2);
    expect(posted.map((p) => p.body)).toEqual([
      { key: { id: 1 } },
      { key: { id: 2 } },
    ]);
    w.unmount();
  });

  it("runs row actions from record context when the row has no resource identity", async () => {
    const action: Action = {
      id: "dns.update",
      label: "Edit",
      routeId: "cloudflare.dns.update",
      method: "PUT",
      risk: RiskLevel.Write,
      requiresConfirm: false,
      params: {
        zone: "${record.zone_id}",
        record: "${record.id}",
      },
      input: {
        groups: [
          {
            name: "DNS",
            fields: [
              {
                key: "ttl",
                label: "TTL",
                type: "number",
                default: "${record.ttl}",
              },
            ],
          },
        ],
      },
    };
    const w = mount(ActionBar, {
      attachTo: document.body,
      props: {
        connectionId: "c1",
        actions: [action],
        record: { id: "rec_1", zone_id: "zone_1", ttl: 300 },
      },
    });

    await w.find("button").trigger("click");
    await flushPromises();
    (document.body.querySelector("form") as HTMLFormElement).dispatchEvent(
      new Event("submit", { cancelable: true, bubbles: true }),
    );
    await flushPromises();

    const url = new URL(posted[0].url, "http://localhost");
    expect(url.searchParams.get("p.zone")).toBe("zone_1");
    expect(url.searchParams.get("p.record")).toBe("rec_1");
    expect((posted[0].body as { ttl: number }).ttl).toBe(300);
    w.unmount();
  });

  it("opens URL actions with input through the generic form and sends selected values as params", async () => {
    const action: Action = {
      id: "open",
      label: "Open",
      routeId: "docker.container.open",
      method: "GET",
      risk: RiskLevel.Safe,
      requiresConfirm: false,
      open: "url",
      params: { id: "${resource.uid}" },
      input: {
        groups: [
          {
            name: "Open",
            fields: [
              {
                key: "port",
                label: "Port",
                type: "select",
                required: true,
                options: [
                  { label: "HTTP 80/tcp", value: "80" },
                  { label: "HTTPS 8443/tcp", value: "https:8443" },
                ],
              },
            ],
          },
        ],
      },
    };
    const w = mount(ActionBar, {
      attachTo: document.body,
      props: {
        connectionId: "c1",
        actions: [action],
        resource: { kind: "container", name: "web", uid: "c-1" },
      },
    });

    await w.find("button").trigger("click");
    await flushPromises();
    expect(posted).toHaveLength(0);
    expect(document.body.textContent).toContain("Port");

    const form = document.body.querySelector("form") as HTMLFormElement;
    const select = w.findComponent({ name: "Select" });
    select.vm.$emit("update:modelValue", "https:8443");
    await flushPromises();
    form.dispatchEvent(
      new Event("submit", { cancelable: true, bubbles: true }),
    );
    await flushPromises();

    expect(posted).toHaveLength(1);
    expect(posted[0].method).toBe("GET");
    expect(posted[0].body).toBeUndefined();
    const url = new URL(posted[0].url, "http://localhost");
    expect(url.searchParams.get("p.id")).toBe("c-1");
    expect(url.searchParams.get("p.port")).toBe("https:8443");
    expect(opened).toEqual(["/proxy/opened/"]);
    w.unmount();
  });
});
