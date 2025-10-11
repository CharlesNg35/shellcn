export const FOLDER_CONFIG = {
  /**
   * Maximum folder nesting depth supported in the UI.
   * 1 => Flat (no subfolders), 2 => One level of subfolders, n => unlimited.
   */
  maxDepth: 1,
  /**
   * Whether the UI should expose an option to create subfolders.
   */
  allowSubfolders: false,
  /**
   * Whether folder forms should allow assigning or changing a parent folder.
   */
  allowParentSelection: false,
} as const

export type FolderConfig = typeof FOLDER_CONFIG
