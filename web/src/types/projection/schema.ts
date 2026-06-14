import type { DataSource } from "./core";

export const FieldType = {
  Text: "text",
  Email: "email",
  Url: "url",
  Tel: "tel",
  Number: "number",
  Stepper: "stepper",
  Slider: "slider",
  Password: "password",
  Select: "select",
  Radio: "radio",
  MultiSelect: "multiselect",
  File: "file",
  Toggle: "toggle",
  Textarea: "textarea",
  Json: "json",
  Duration: "duration",
  CredentialRef: "credential_ref",
  Object: "object",
  Array: "array",
  AutoComplete: "autocomplete",
  Map: "map",
} as const;
export type FieldType = (typeof FieldType)[keyof typeof FieldType];

export interface Option {
  label: string;
  value: string | number | boolean;
}

export type CredentialKind = string;

export interface CredentialKindInfo {
  kind: CredentialKind;
  label: string;
  fields: Field[];
  compatibleProtocols?: string[];
}

export interface CredentialSelector {
  kind: CredentialKind;
  protocols?: string[];
}

export const Operator = {
  Eq: "eq",
  Neq: "neq",
  In: "in",
  Nin: "nin",
  Empty: "empty",
  NotEmpty: "notEmpty",
} as const;
export type Operator = (typeof Operator)[keyof typeof Operator];

export interface Rule {
  field: string;
  op: Operator;
  value?: unknown;
}

export interface Condition {
  allOf?: Rule[];
  anyOf?: Rule[];
}

export const ValidatorType = {
  Min: "min",
  Max: "max",
  Regex: "regex",
  OneOf: "oneOf",
} as const;
export type ValidatorType = (typeof ValidatorType)[keyof typeof ValidatorType];

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
  public?: boolean;
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
