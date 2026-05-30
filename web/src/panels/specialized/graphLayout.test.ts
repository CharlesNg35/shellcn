import { describe, it, expect } from "vitest";
import { buildGraph, mergeGraph, edgeColor } from "./graphLayout";

describe("buildGraph", () => {
  it("renders fielded nodes as record boxes and plain nodes as default", () => {
    const { nodes } = buildGraph({
      nodes: [
        {
          id: "orders",
          label: "orders",
          fields: [{ name: "id" }, { name: "customer_id", key: "fk" }],
        },
        { id: "n1", label: "Person" },
      ],
    });
    const orders = nodes.find((n) => n.id === "orders")!;
    const plain = nodes.find((n) => n.id === "n1")!;
    expect(orders.type).toBe("record");
    expect(plain.type).toBe("default");
    // Layout assigns finite positions so nodes never stack at the origin.
    expect(Number.isFinite(orders.position.x)).toBe(true);
    expect(Number.isFinite(orders.position.y)).toBe(true);
  });

  it("maps edges with arrow markers and keeps the label", () => {
    const { edges } = buildGraph({
      nodes: [{ id: "a" }, { id: "b" }],
      edges: [{ source: "a", target: "b", label: "fk_col" }],
    });
    expect(edges).toHaveLength(1);
    expect(edges[0].source).toBe("a");
    expect(edges[0].target).toBe("b");
    expect(edges[0].label).toBe("fk_col");
    expect(edges[0].markerEnd).toBeTruthy();
  });

  it("drops edges whose endpoints are missing so none dangle", () => {
    const { edges } = buildGraph({
      nodes: [{ id: "a" }],
      edges: [{ source: "a", target: "ghost" }],
    });
    // The edge still maps (Vue Flow tolerates it), but layout ignored the ghost.
    expect(edges).toHaveLength(1);
  });

  it("handles an empty payload", () => {
    const { nodes, edges } = buildGraph({});
    expect(nodes).toEqual([]);
    expect(edges).toEqual([]);
  });

  it("colors edges of the same label identically and differently across labels", () => {
    expect(edgeColor("KNOWS")).toBe(edgeColor("KNOWS"));
    expect(edgeColor(undefined)).toBe("#94a3b8");
  });
});

describe("mergeGraph", () => {
  it("merges an expanded neighbourhood without duplicating nodes or edges", () => {
    const base = {
      nodes: [{ id: "a" }, { id: "b" }],
      edges: [{ id: "e1", source: "a", target: "b" }],
    };
    const incoming = {
      nodes: [{ id: "b" }, { id: "c" }],
      edges: [
        { id: "e1", source: "a", target: "b" },
        { id: "e2", source: "b", target: "c" },
      ],
    };
    const merged = mergeGraph(base, incoming);
    expect(merged.nodes.map((n) => n.id).sort()).toEqual(["a", "b", "c"]);
    expect(merged.edges.map((e) => e.id).sort()).toEqual(["e1", "e2"]);
  });

  it("dedupes edges without ids by endpoints + label", () => {
    const merged = mergeGraph(
      { edges: [{ source: "a", target: "b", label: "fk" }] },
      { edges: [{ source: "a", target: "b", label: "fk" }] },
    );
    expect(merged.edges).toHaveLength(1);
  });
});
