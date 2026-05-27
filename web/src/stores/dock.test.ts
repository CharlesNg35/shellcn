import { describe, it, expect, beforeEach } from "vitest";
import { setActivePinia, createPinia } from "pinia";
import { useDockStore, type DockItem } from "./dock";

const item = (id: string): DockItem => ({
  id,
  title: id,
  panel: "log_stream",
  source: { routeId: "x.logs", method: "WS" },
});

beforeEach(() => setActivePinia(createPinia()));

describe("dock store", () => {
  it("opens items (deduped) and tracks the active one", () => {
    const dock = useDockStore();
    dock.open("c1", item("a"));
    dock.open("c1", item("b"));
    dock.open("c1", item("a"));
    const s = dock.state("c1");
    expect(s.items.map((i) => i.id)).toEqual(["a", "b"]);
    expect(s.activeId).toBe("a");
    expect(s.collapsed).toBe(false);
  });

  it("closing the active item falls back to a neighbor", () => {
    const dock = useDockStore();
    dock.open("c1", item("a"));
    dock.open("c1", item("b"));
    dock.activate("c1", "a");
    dock.close("c1", "a");
    expect(dock.state("c1").activeId).toBe("b");
    dock.close("c1", "b");
    expect(dock.state("c1").activeId).toBeUndefined();
    expect(dock.state("c1").items).toHaveLength(0);
  });

  it("clamps the height and toggles collapse", () => {
    const dock = useDockStore();
    dock.setHeight("c1", 5);
    expect(dock.state("c1").height).toBe(120);
    dock.setHeight("c1", 99999);
    expect(dock.state("c1").height).toBe(800);
    dock.toggleCollapsed("c1");
    expect(dock.state("c1").collapsed).toBe(true);
  });

  it("keeps a separate dialog slot", () => {
    const dock = useDockStore();
    dock.openDialog("c1", item("yaml"));
    expect(dock.state("c1").dialog?.id).toBe("yaml");
    dock.closeDialog("c1");
    expect(dock.state("c1").dialog).toBeNull();
  });

  it("isolates state per connection", () => {
    const dock = useDockStore();
    dock.open("c1", item("a"));
    expect(dock.state("c2").items).toHaveLength(0);
  });
});
