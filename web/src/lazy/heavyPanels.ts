export const heavyPanelLoaders = {
  terminal: () => import("@xterm/xterm"),
  code_editor: () => import("monaco-editor"),
  remote_desktop: () => import("@novnc/novnc"),
} as const;

export type HeavyPanelKind = keyof typeof heavyPanelLoaders;
