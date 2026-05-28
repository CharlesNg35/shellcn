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
