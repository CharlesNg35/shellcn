export type IconType = "lucide" | "url" | "base64" | "emoji" | "svg";

export interface Icon {
  type: IconType;
  value: string;
}

export interface PluginCategoryInfo {
  key: string;
  label: string;
  icon: Icon;
  order: number;
}

export type Method = "GET" | "POST" | "PUT" | "PATCH" | "DELETE" | "WS";

export const RiskLevel = {
  Safe: "safe",
  Write: "write",
  Destructive: "destructive",
  Privileged: "privileged",
} as const;
export type RiskLevel = (typeof RiskLevel)[keyof typeof RiskLevel];

export type Transport = "direct" | "agent";

export const TRANSPORT_DIRECT: Transport = "direct";
export const TRANSPORT_AGENT: Transport = "agent";

export const Layout = {
  Tabs: "tabs",
  SidebarTree: "sidebar_tree",
  Dashboard: "dashboard",
  Single: "single",
} as const;
export type Layout = (typeof Layout)[keyof typeof Layout];

export interface DataSource {
  routeId: string;
  method?: Method;
  params?: Record<string, string>;
}

export interface SortKey {
  field: string;
  desc?: boolean;
}

export interface PageRequest {
  cursor?: string;
  limit?: number;
  filter?: Record<string, string>;
  sort?: SortKey[];
}

export interface Page<T> {
  items: T[];
  nextCursor: string;
  total?: number;
}
