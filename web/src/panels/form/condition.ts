import {
  Operator,
  ValidatorType,
  type Condition,
  type Field,
  type Rule,
  type Validator,
} from "@/types/projection";

type Values = Record<string, unknown>;

function isEmpty(v: unknown): boolean {
  return (
    v === undefined ||
    v === null ||
    v === "" ||
    (Array.isArray(v) && v.length === 0)
  );
}

// resolveField reads a rule field, supporting dotted paths (e.g. "can.delete").
// An exact key wins first, so existing flat keys keep working unchanged.
export function resolveField(values: Values, path: string): unknown {
  if (path in values) return values[path];
  let cur: unknown = values;
  for (const seg of path.split(".")) {
    if (!cur || typeof cur !== "object") return undefined;
    cur = (cur as Record<string, unknown>)[seg];
  }
  return cur;
}

function num(v: unknown): number {
  if (typeof v === "number") return v;
  if (typeof v === "string") return parseFloat(v);
  return NaN;
}

export function evalRule(rule: Rule, values: Values): boolean {
  const v = resolveField(values, rule.field);
  switch (rule.op) {
    case Operator.Eq:
      return v === rule.value;
    case Operator.Neq:
      return v !== rule.value;
    case Operator.In:
      return Array.isArray(rule.value) && rule.value.includes(v);
    case Operator.Nin:
      return Array.isArray(rule.value) && !rule.value.includes(v);
    case Operator.Empty:
      return isEmpty(v);
    case Operator.NotEmpty:
      return !isEmpty(v);
    case Operator.Gt:
      return num(v) > num(rule.value);
    case Operator.Lt:
      return num(v) < num(rule.value);
    case Operator.Gte:
      return num(v) >= num(rule.value);
    case Operator.Lte:
      return num(v) <= num(rule.value);
    case Operator.Contains:
      return Array.isArray(v)
        ? v.map(String).includes(String(rule.value))
        : String(v ?? "").includes(String(rule.value));
    default:
      return true;
  }
}

export function isVisible(
  cond: Condition | undefined,
  values: Values,
): boolean {
  if (!cond) return true;
  if (cond.allOf && !cond.allOf.every((r) => evalRule(r, values))) return false;
  if (
    cond.anyOf &&
    cond.anyOf.length > 0 &&
    !cond.anyOf.some((r) => evalRule(r, values))
  ) {
    return false;
  }
  if (cond.all && !cond.all.every((c) => isVisible(c, values))) return false;
  if (
    cond.any &&
    cond.any.length > 0 &&
    !cond.any.some((c) => isVisible(c, values))
  ) {
    return false;
  }
  if (cond.not && isVisible(cond.not, values)) return false;
  return true;
}

function runValidator(v: Validator, value: unknown): string | null {
  const n = Number(v.value);
  switch (v.type) {
    case ValidatorType.Min:
      if (typeof value === "number" && value < n)
        return `Must be at least ${v.value}.`;
      if (typeof value === "string" && value.length < n)
        return `Must be at least ${v.value} characters.`;
      return null;
    case ValidatorType.Max:
      if (typeof value === "number" && value > n)
        return `Must be at most ${v.value}.`;
      if (typeof value === "string" && value.length > n)
        return `Must be at most ${v.value} characters.`;
      return null;
    case ValidatorType.Regex:
      return typeof value === "string" &&
        !new RegExp(String(v.value)).test(value)
        ? "Invalid format."
        : null;
    case ValidatorType.OneOf:
      return Array.isArray(v.value) && !v.value.includes(value)
        ? "Invalid choice."
        : null;
    default:
      return null;
  }
}

export function validateField(field: Field, value: unknown): string | null {
  if (field.required && isEmpty(value)) return `${field.label} is required.`;
  if (isEmpty(value)) return null;
  for (const validator of field.validators ?? []) {
    const msg = runValidator(validator, value);
    if (msg) return validator.message ?? msg;
  }
  return null;
}
