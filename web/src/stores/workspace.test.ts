import { describe, it, expect, beforeEach } from "vitest";
import { setActivePinia, createPinia } from "pinia";
import { useWorkspaceStore } from "./workspace";

beforeEach(() => {
  setActivePinia(createPinia());
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

  it("keeps per-connection tab/selection state across reads (remount-safe)", () => {
    const ws = useWorkspaceStore();
    ws.open("a");
    ws.setActiveTab("a", "logs");
    ws.selectRef("a", { kind: "container", name: "x", uid: "x1" });
    // A remounting component re-reads the store; state is still there.
    expect(ws.view("a").activeTab).toBe("logs");
    expect(ws.view("a").selectedRef?.uid).toBe("x1");
  });

  it("selecting a group clears the selected resource", () => {
    const ws = useWorkspaceStore();
    ws.open("a");
    ws.selectRef("a", { kind: "container", name: "x", uid: "x1" });
    ws.selectGroup("a", "images");
    expect(ws.view("a").selectedGroup).toBe("images");
    expect(ws.view("a").selectedRef).toBeNull();
  });
});
