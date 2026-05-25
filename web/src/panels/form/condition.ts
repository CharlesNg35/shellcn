import type { Condition, Field, Rule, Validator } from "../../types/projection";

type Values = Record<string, unknown>;

function isEmpty(v: unknown): boolean {
  return (
    v === undefined ||
    v === null ||
    v === "" ||
    (Array.isArray(v) && v.length === 0)
  );
}

export function evalRule(rule: Rule, values: Values): boolean {
  const v = values[rule.field];
  switch (rule.op) {
    case "eq":
      return v === rule.value;
    case "neq":
      return v !== rule.value;
    case "in":
      return Array.isArray(rule.value) && rule.value.includes(v);
    case "nin":
      return Array.isArray(rule.value) && !rule.value.includes(v);
    case "empty":
      return isEmpty(v);
    case "notEmpty":
      return !isEmpty(v);
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
  return true;
}

function runValidator(v: Validator, value: unknown): string | null {
  const n = Number(v.value);
  switch (v.type) {
    case "min":
      if (typeof value === "number" && value < n)
        return `Must be at least ${v.value}.`;
      if (typeof value === "string" && value.length < n)
        return `Must be at least ${v.value} characters.`;
      return null;
    case "max":
      if (typeof value === "number" && value > n)
        return `Must be at most ${v.value}.`;
      if (typeof value === "string" && value.length > n)
        return `Must be at most ${v.value} characters.`;
      return null;
    case "regex":
      return typeof value === "string" &&
        !new RegExp(String(v.value)).test(value)
        ? "Invalid format."
        : null;
    case "oneOf":
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
