import type { DataSource, Icon } from "./core";
import type { Badge, Column, ResourceIdentity, Severity, Tab } from "./panels";

export interface FilterOption {
  value: string;
  label?: string;
}

export interface ScopeFilter {
  param: string;
  label: string;
  icon?: Icon;
  control?: ScopeControl;
  multiple?: boolean;
  allowCustom?: boolean;
  optionsSource?: DataSource;
  watchSource?: DataSource;
  options?: FilterOption[];
  valueField?: string;
  labelField?: string;
  allLabel?: string;
  defaultValue?: string;
}

export const ScopeControl = {
  Select: "select",
  AutoComplete: "autocomplete",
  Search: "search",
  Toggle: "toggle",
} as const;
export type ScopeControl = (typeof ScopeControl)[keyof typeof ScopeControl];

export interface TreeGroup {
  key: string;
  label: string;
  icon?: Icon;
  source?: DataSource;
  resourceKind?: string;
  ref?: ResourceIdentity;
  badge?: Badge;
}

export interface TreeNode {
  key: string;
  label: string;
  icon?: Icon;
  ref?: ResourceIdentity;
  leaf?: boolean;
  childrenSource?: DataSource;
  badge?: Badge;
  resourceKind?: string;
  listParams?: Record<string, string>;
  data?: Record<string, unknown>;
}

export interface HeaderSpec {
  title?: string;
  statusField?: string;
  severities?: Record<string, Severity>;
}

export interface ResourceActions {
  toolbar?: string[];
  row?: string[];
  detail?: string[];
  selectable?: boolean;
}

export interface DetailView {
  header: HeaderSpec;
  defaultTab?: string;
  tabs: Tab[];
}

export interface ResourceType {
  kind: string;
  title: string;
  list: DataSource;
  watch?: DataSource;
  columns: Column[];
  columnsSource?: DataSource;
  actions?: ResourceActions;
  detail: DetailView;
}

export const EventType = {
  Added: "added",
  Updated: "updated",
  Deleted: "deleted",
} as const;
export type EventType = (typeof EventType)[keyof typeof EventType];

export interface ResourceEvent {
  type: EventType;
  ref: ResourceIdentity;
  resource?: unknown;
}
