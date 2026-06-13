export const IconType = {
  Lucide: "lucide",
  Url: "url",
  Base64: "base64",
  Emoji: "emoji",
  Svg: "svg",
} as const;
export type IconType = (typeof IconType)[keyof typeof IconType];

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

export const Method = {
  Get: "GET",
  Post: "POST",
  Put: "PUT",
  Patch: "PATCH",
  Delete: "DELETE",
  WS: "WS",
} as const;
export type Method = (typeof Method)[keyof typeof Method];

export const RiskLevel = {
  Safe: "safe",
  Write: "write",
  Destructive: "destructive",
  Privileged: "privileged",
} as const;
export type RiskLevel = (typeof RiskLevel)[keyof typeof RiskLevel];

export const Transport = {
  Direct: "direct",
  Agent: "agent",
} as const;
export type Transport = (typeof Transport)[keyof typeof Transport];

export const TRANSPORT_DIRECT: Transport = Transport.Direct;
export const TRANSPORT_AGENT: Transport = Transport.Agent;

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
