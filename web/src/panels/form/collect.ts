import { FieldType, type Field } from "@/types/projection";
import { isVisible, validateField } from "./condition";

export interface Collected {
  value?: unknown;
  error?: string;
}

// collectField validates a field value and returns the value to submit,
// recursing into object/array fields and parsing json. The first nested error
// is surfaced prefixed with the row/sub-field that produced it.
export function collectField(field: Field, value: unknown): Collected {
  switch (field.type) {
    case FieldType.Json:
      return collectJson(field, value);
    case FieldType.Object:
      return collectObject(field, value);
    case FieldType.Array:
      return collectArray(field, value);
    case FieldType.Map:
      return collectMap(field, value);
    default: {
      const msg = validateField(field, value);
      return msg ? { error: msg } : { value };
    }
  }
}

function collectJson(field: Field, value: unknown): Collected {
  let parsed = value;
  if (typeof value === "string") {
    const trimmed = value.trim();
    if (trimmed === "") return { value: undefined };
    try {
      parsed = JSON.parse(trimmed);
    } catch {
      return { error: "Enter valid JSON." };
    }
  }
  const msg = validateField(field, parsed);
  return msg ? { error: msg } : { value: parsed };
}

function collectObject(field: Field, value: unknown): Collected {
  const record = (value ?? {}) as Record<string, unknown>;
  const out: Record<string, unknown> = {};
  for (const sub of field.fields ?? []) {
    if (!isVisible(sub.visibleWhen, record)) continue;
    const r = collectField(sub, record[sub.key]);
    if (r.error) return { error: `${sub.label}: ${r.error}` };
    if (r.value !== undefined) out[sub.key] = r.value;
  }
  if (Object.keys(out).length === 0) {
    return field.required
      ? { error: `${field.label} is required.` }
      : { value: undefined };
  }
  return { value: out };
}

function collectMap(field: Field, value: unknown): Collected {
  const record = (value ?? {}) as Record<string, unknown>;
  const keys = Object.keys(record);
  if (field.required && keys.length === 0)
    return { error: `${field.label} is required.` };
  const out: Record<string, unknown> = {};
  for (const key of keys) {
    if (key.trim() === "") return { error: `${field.label}: empty key.` };
    const r = field.item
      ? collectField(field.item, record[key])
      : { value: record[key] };
    if (r.error) return { error: `${field.label} "${key}": ${r.error}` };
    out[key] = r.value;
  }
  if (keys.length === 0 && !field.required) return { value: undefined };
  return { value: out };
}

function collectArray(field: Field, value: unknown): Collected {
  const list = Array.isArray(value) ? value : [];
  if (field.required && list.length === 0)
    return { error: `${field.label} is required.` };
  if (field.minItems && list.length < field.minItems)
    return { error: `Add at least ${field.minItems}.` };
  if (field.maxItems && list.length > field.maxItems)
    return { error: `At most ${field.maxItems} allowed.` };
  if (!field.item) return { value: list };
  const label = field.itemLabel ?? "Item";
  const out: unknown[] = [];
  for (let i = 0; i < list.length; i++) {
    const r = collectField(field.item, list[i]);
    if (r.error) return { error: `${label} ${i + 1}: ${r.error}` };
    out.push(r.value);
  }
  return { value: out };
}
