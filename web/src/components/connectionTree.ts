import type { ConnectionFolder, ConnectionSummary } from "../types/projection";

export interface ConnectionFolderNode extends ConnectionFolder {
  children: ConnectionFolderNode[];
  connections: ConnectionSummary[];
}

export interface ConnectionFolderMenuAction {
  key: "new-child" | "rename" | "delete";
  folder: ConnectionFolder;
}
