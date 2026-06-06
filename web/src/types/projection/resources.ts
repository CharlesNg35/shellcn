import type { DataSource, Icon } from "./core";
import type { Badge, Column, ResourceRef, Severity, Tab } from "./panels";

export interface FilterOption {
  value: string;
  label?: string;
}

export interface ScopeFilter {
  param: string;
  label: string;
  icon?: Icon;
  control?: string;
  optionsSource?: DataSource;
  options?: FilterOption[];
  valueField?: string;
  labelField?: string;
  allLabel?: string;
  defaultValue?: string;
}

export interface TreeGroup {
  key: string;
  label: string;
  icon?: Icon;
  source?: DataSource;
  resourceKind?: string;
  ref?: ResourceRef;
  badge?: Badge;
}

export interface TreeNode {
  key: string;
  label: string;
  icon?: Icon;
  ref?: ResourceRef;
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

export type EventType = "added" | "updated" | "deleted";

export interface ResourceEvent {
  type: EventType;
  ref: ResourceRef;
  resource?: unknown;
}
