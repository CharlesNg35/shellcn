import type { ConnectionFolder, ConnectionSummary } from "../types/projection";

export interface ConnectionFolderNode extends ConnectionFolder {
  kind: "folder";
  children: ConnectionTreeItem[];
}

export interface ConnectionNode {
  kind: "connection";
  connection: ConnectionSummary;
}

export type ConnectionTreeItem = ConnectionFolderNode | ConnectionNode;

export interface ConnectionFolderMenuAction {
  key: "new-child" | "rename" | "delete";
  folder: ConnectionFolder;
}

export interface ConnectionTreeDropPreference {
  connectionId?: string;
  folderId?: string;
  targetParentId?: string;
}

export function dedupeConnectionTree(
  items: ConnectionTreeItem[],
  preference: ConnectionTreeDropPreference = {},
): ConnectionTreeItem[] {
  const keepConnectionInParent = preferredParent(
    items,
    "connection",
    preference,
  );
  const keepFolderInParent = preferredParent(items, "folder", preference);
  const seenConnections = new Set<string>();
  const seenFolders = new Set<string>();

  function normalize(
    nodes: ConnectionTreeItem[],
    parentId: string | undefined,
  ): ConnectionTreeItem[] {
    const out: ConnectionTreeItem[] = [];
    for (const item of nodes) {
      if (item.kind === "connection") {
        const id = item.connection.id;
        const preferredParent = keepConnectionInParent.get(id);
        if (
          preferredParent !== undefined &&
          preferredParent !== (parentId ?? "")
        ) {
          continue;
        }
        if (seenConnections.has(id)) continue;
        seenConnections.add(id);
        out.push(item);
        continue;
      }

      const preferredParent = keepFolderInParent.get(item.id);
      if (
        preferredParent !== undefined &&
        preferredParent !== (parentId ?? "")
      ) {
        continue;
      }
      if (seenFolders.has(item.id)) continue;
      seenFolders.add(item.id);
      out.push({ ...item, children: normalize(item.children, item.id) });
    }
    return out;
  }

  return normalize(items, undefined);
}

function preferredParent(
  items: ConnectionTreeItem[],
  kind: "connection" | "folder",
  preference: ConnectionTreeDropPreference,
): Map<string, string> {
  const id =
    kind === "connection" ? preference.connectionId : preference.folderId;
  if (!id) return new Map();

  const targetParentId = preference.targetParentId ?? "";
  let found = false;
  function visit(
    nodes: ConnectionTreeItem[],
    parentId: string | undefined,
  ): void {
    for (const item of nodes) {
      if (kind === "connection" && item.kind === "connection") {
        if (item.connection.id === id && (parentId ?? "") === targetParentId) {
          found = true;
        }
      } else if (kind === "folder" && item.kind === "folder") {
        if (item.id === id && (parentId ?? "") === targetParentId) {
          found = true;
        }
        visit(item.children, item.id);
      } else if (item.kind === "folder") {
        visit(item.children, item.id);
      }
    }
  }
  visit(items, undefined);
  return found ? new Map([[id, targetParentId]]) : new Map();
}
