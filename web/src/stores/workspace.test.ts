import { describe, it, expect, beforeEach } from "vitest";
import { setActivePinia, createPinia } from "pinia";
import { useWorkspaceStore, MAX_WORKBENCH_TABS } from "./workspace";

beforeEach(() => {
  setActivePinia(createPinia());
});

const detail = (uid: string) => ({
  id: `detail:${uid}`,
  title: uid,
  kind: "detail" as const,
  ref: { kind: "container", name: uid, uid },
  row: { ref: { kind: "container", name: uid, uid } },
});

describe("workspace store", () => {
  it("tracks active connection and dedupes recents (most-recent first)", () => {
    const ws = useWorkspaceStore();
    ws.open("a");
    ws.open("b");
    ws.open("a");
    expect(ws.activeConnectionId).toBe("a");
    expect(ws.recent).toEqual(["a", "b"]);
  });

  it("caps recents at ten", () => {
    const ws = useWorkspaceStore();
    for (let i = 0; i < 15; i++) ws.open(`c${i}`);
    expect(ws.recent).toHaveLength(10);
    expect(ws.recent[0]).toBe("c14");
  });

  it("opens multiple views (deduped) as a tab strip and tracks the active one", () => {
    const ws = useWorkspaceStore();
    ws.open("a");
    ws.openView("a", detail("x1"));
    ws.openView("a", detail("x2"));
    ws.openView("a", detail("x1")); // dedupe + re-activate
    expect(ws.view("a").views.map((v) => v.id)).toEqual([
      "detail:x1",
      "detail:x2",
    ]);
    expect(ws.activeView("a")?.id).toBe("detail:x1");
  });

  it("closing the active view falls back to a neighbor", () => {
    const ws = useWorkspaceStore();
    ws.openView("a", detail("x1"));
    ws.openView("a", detail("x2"));
    ws.activateView("a", "detail:x1");
    ws.closeView("a", "detail:x1");
    expect(ws.activeView("a")?.id).toBe("detail:x2");
    ws.closeView("a", "detail:x2");
    expect(ws.activeView("a")).toBeUndefined();
    expect(ws.view("a").views).toHaveLength(0);
  });

  it("caps open views and auto-closes the oldest non-active tab", () => {
    const ws = useWorkspaceStore();
    for (let i = 0; i < MAX_WORKBENCH_TABS + 3; i++)
      ws.openView("a", detail(`x${i}`));
    const c = ws.view("a");
    expect(c.views).toHaveLength(MAX_WORKBENCH_TABS);
    // The newest view stays open and active; the oldest were evicted.
    expect(ws.activeView("a")?.id).toBe(`detail:x${MAX_WORKBENCH_TABS + 2}`);
    expect(c.views.some((v) => v.id === "detail:x0")).toBe(false);
  });

  it("keeps a list view with its scoping params", () => {
    const ws = useWorkspaceStore();
    ws.openView("a", {
      id: "list:pod:namespace=prod",
      title: "Pods",
      kind: "list",
      resourceKind: "pod",
      params: { namespace: "prod" },
    });
    expect(ws.activeView("a")?.params).toEqual({ namespace: "prod" });
  });

  it("isolates views per connection and survives re-reads (remount-safe)", () => {
    const ws = useWorkspaceStore();
    ws.openView("a", detail("x1"));
    ws.setActiveTab("a", "logs");
    expect(ws.view("b").views).toHaveLength(0);
    expect(ws.view("a").activeTab).toBe("logs");
    expect(ws.view("a").views).toHaveLength(1);
  });
});
