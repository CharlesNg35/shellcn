import { describe, expect, it } from "vitest";
import {
  dedupeConnectionTree,
  type ConnectionTreeItem,
} from "./connectionTree";

describe("connectionTree", () => {
  it("keeps the dragged connection in the drop target when Sortable leaves a duplicate", () => {
    const item = {
      kind: "connection" as const,
      connection: {
        id: "c1",
        name: "Prod",
        protocol: "ssh",
        transport: "direct" as const,
      },
    };
    const tree: ConnectionTreeItem[] = [
      {
        kind: "folder",
        id: "f1",
        name: "Production",
        color: "blue",
        sortOrder: 0,
        children: [item],
      },
      item,
    ];

    const normalized = dedupeConnectionTree(tree, {
      connectionId: "c1",
      targetParentId: "f1",
    });

    expect(normalized).toHaveLength(1);
    expect(normalized[0]).toMatchObject({ kind: "folder", id: "f1" });
    expect(
      normalized[0].kind === "folder" && normalized[0].children,
    ).toHaveLength(1);
  });
});
