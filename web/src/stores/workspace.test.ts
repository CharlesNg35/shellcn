import { describe, it, expect, beforeEach } from "vitest";
import { setActivePinia, createPinia } from "pinia";
import { KEEP_ALIVE_WORKBENCH_TABS_MAX } from "./sessionLimits";
import {
  DEFAULT_TREE_SIDEBAR_WIDTH,
  MAX_TREE_SIDEBAR_WIDTH,
  MIN_TREE_SIDEBAR_WIDTH,
  TREE_SIDEBAR_COLLAPSE_THRESHOLD,
  useWorkspaceStore,
} from "./workspace";

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

  it("tracks connected ids in least-recent to most-recent order", () => {
    const ws = useWorkspaceStore();
    ws.setConnected("a", true);
    ws.setConnected("b", true);
    ws.setConnected("a", true);
    expect(ws.connectedIds()).toEqual(["b", "a"]);

    ws.setConnected("b", false);
    expect(ws.connectedIds()).toEqual(["a"]);
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

  it("reuses the active preview tab until it is pinned", () => {
    const ws = useWorkspaceStore();
    ws.openPreviewView("a", detail("x1"));
    ws.openPreviewView("a", detail("x2"));
    expect(ws.view("a").views.map((v) => v.id)).toEqual(["detail:x2"]);
    expect(ws.activeView("a")?.preview).toBe(true);

    ws.pinView("a", "detail:x2");
    ws.openPreviewView("a", detail("x3"));
    expect(ws.view("a").views.map((v) => v.id)).toEqual([
      "detail:x2",
      "detail:x3",
    ]);
    expect(ws.view("a").views.find((v) => v.id === "detail:x2")?.preview).toBe(
      false,
    );
    expect(ws.activeView("a")?.id).toBe("detail:x3");
    expect(ws.activeView("a")?.preview).toBe(true);
  });

  it("activates an already-open pinned tab instead of replacing preview state", () => {
    const ws = useWorkspaceStore();
    ws.openPreviewView("a", detail("x1"));
    ws.pinView("a", "detail:x1");
    ws.openPreviewView("a", detail("x2"));
    ws.openPreviewView("a", detail("x1"));
    expect(ws.view("a").views.map((v) => v.id)).toEqual([
      "detail:x1",
      "detail:x2",
    ]);
    expect(ws.activeView("a")?.id).toBe("detail:x1");
    expect(ws.activeView("a")?.preview).toBe(false);
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
    for (let i = 0; i < KEEP_ALIVE_WORKBENCH_TABS_MAX + 3; i++)
      ws.openView("a", detail(`x${i}`));
    const c = ws.view("a");
    expect(c.views).toHaveLength(KEEP_ALIVE_WORKBENCH_TABS_MAX);
    // The newest view stays open and active; the oldest were evicted.
    expect(ws.activeView("a")?.id).toBe(
      `detail:x${KEEP_ALIVE_WORKBENCH_TABS_MAX + 2}`,
    );
    expect(c.views.some((v) => v.id === "detail:x0")).toBe(false);
  });

  it("accepts a reordered view list (drag-to-reorder via v-model)", () => {
    const ws = useWorkspaceStore();
    ws.openView("a", detail("x1"));
    ws.openView("a", detail("x2"));
    ws.openView("a", detail("x3"));
    const reordered = [...ws.view("a").views].reverse();
    ws.setViews("a", reordered);
    expect(ws.view("a").views.map((v) => v.id)).toEqual([
      "detail:x3",
      "detail:x2",
      "detail:x1",
    ]);
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

  it("keeps tree sidebar layout in memory per connection", () => {
    const ws = useWorkspaceStore();

    expect(ws.layout("a").treeSidebarWidth).toBe(DEFAULT_TREE_SIDEBAR_WIDTH);
    ws.setTreeSidebarWidth("a", DEFAULT_TREE_SIDEBAR_WIDTH + 40);

    expect(ws.layout("a").treeSidebarWidth).toBe(
      DEFAULT_TREE_SIDEBAR_WIDTH + 40,
    );
    expect(ws.layout("b").treeSidebarWidth).toBe(DEFAULT_TREE_SIDEBAR_WIDTH);
  });

  it("collapses tree sidebar width at the midpoint threshold and clamps the maximum", () => {
    const ws = useWorkspaceStore();

    ws.setTreeSidebarWidth("a", TREE_SIDEBAR_COLLAPSE_THRESHOLD);
    expect(ws.layout("a").treeSidebarWidth).toBe(0);

    ws.setTreeSidebarWidth("a", TREE_SIDEBAR_COLLAPSE_THRESHOLD + 1);
    expect(ws.layout("a").treeSidebarWidth).toBe(MIN_TREE_SIDEBAR_WIDTH);

    ws.setTreeSidebarWidth("a", MIN_TREE_SIDEBAR_WIDTH - 1);
    expect(ws.layout("a").treeSidebarWidth).toBe(MIN_TREE_SIDEBAR_WIDTH);

    ws.setTreeSidebarWidth("a", MIN_TREE_SIDEBAR_WIDTH);
    expect(ws.layout("a").treeSidebarWidth).toBe(MIN_TREE_SIDEBAR_WIDTH);

    ws.setTreeSidebarWidth("a", 9999);
    expect(ws.layout("a").treeSidebarWidth).toBe(MAX_TREE_SIDEBAR_WIDTH);
  });

  it("keeps table query state by stable table identity", () => {
    const ws = useWorkspaceStore();
    const defaults = {
      filterText: "",
      sortField: "name",
      sortOrder: 1,
      first: 0,
      pageSize: 50,
    };
    expect(ws.tableState("c1|pods", defaults)).toEqual(defaults);

    ws.setTableState("c1|pods", {
      filterText: "nginx",
      sortField: "createdAt",
      sortOrder: -1,
      first: 100,
      pageSize: 100,
    });

    expect(ws.tableState("c1|pods", defaults)).toEqual({
      filterText: "nginx",
      sortField: "createdAt",
      sortOrder: -1,
      first: 100,
      pageSize: 100,
    });
    expect(ws.tableState("c1|deployments", defaults)).toEqual(defaults);
  });
});
