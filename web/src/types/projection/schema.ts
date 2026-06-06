import type { DataSource } from "./core";

export type FieldType =
  | "text"
  | "email"
  | "url"
  | "tel"
  | "number"
  | "stepper"
  | "slider"
  | "password"
  | "select"
  | "radio"
  | "multiselect"
  | "file"
  | "toggle"
  | "textarea"
  | "json"
  | "duration"
  | "credential_ref"
  | "object"
  | "array"
  | "autocomplete"
  | "map";

export interface Option {
  label: string;
  value: string | number | boolean;
}

export type CredentialKind = string;

export interface CredentialKindInfo {
  kind: CredentialKind;
  label: string;
  secretLabel: string;
  secretMultiline?: boolean;
  identityLabel?: string;
  compatibleProtocols?: string[];
}

export interface CredentialSelector {
  kinds: CredentialKind[];
  protocols?: string[];
  required?: boolean;
}

export type Operator = "eq" | "neq" | "in" | "nin" | "empty" | "notEmpty";

export interface Rule {
  field: string;
  op: Operator;
  value?: unknown;
}

export interface Condition {
  allOf?: Rule[];
  anyOf?: Rule[];
}

export type ValidatorType = "min" | "max" | "regex" | "oneOf";

export interface Validator {
  type: ValidatorType;
  value?: unknown;
  message?: string;
}

export interface Field {
  key: string;
  label: string;
  type: FieldType;
  required?: boolean;
  secret?: boolean;
  default?: unknown;
  placeholder?: string;
  help?: string;
  options?: Option[];
  optionsSource?: DataSource;
  credential?: CredentialSelector;
  visibleWhen?: Condition;
  validators?: Validator[];
  step?: number;
  fields?: Field[];
  item?: Field;
  minItems?: number;
  maxItems?: number;
  itemLabel?: string;
  addLabel?: string;
  keyLabel?: string;
  keyPlaceholder?: string;
}

export interface Group {
  name: string;
  fields: Field[];
}

export interface Schema {
  groups: Group[];
}
