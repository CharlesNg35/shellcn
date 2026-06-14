import { FieldType, type Field, type Schema } from "@/types/projection";
import { interpolate, lookupRaw, type ResolveContext } from "@/api/dataSource";

// defaultForField seeds an initial value, recursing object sub-fields and the
// minItems rows of an array so a composite field renders ready to edit.
export function defaultForField(
  field: Field,
  ctx: ResolveContext = {},
): unknown {
  if (field.type === FieldType.Object) {
    const out: Record<string, unknown> = {};
    for (const sub of field.fields ?? []) {
      const d = defaultForField(sub, ctx);
      if (d !== undefined) out[sub.key] = d;
    }
    return out;
  }
  if (field.type === FieldType.Array) {
    const n = field.minItems ?? 0;
    return field.item
      ? Array.from({ length: n }, () =>
          defaultForField(field.item as Field, ctx),
        )
      : [];
  }
  if (field.type === FieldType.Map) {
    return resolveDefault(field.default ?? {}, ctx);
  }
  return resolveDefault(field.default, ctx);
}

function resolveDefault(value: unknown, ctx: ResolveContext): unknown {
  if (Array.isArray(value))
    return value.map((item) => resolveDefault(item, ctx));
  if (value && typeof value === "object") {
    return Object.fromEntries(
      Object.entries(value as Record<string, unknown>).map(([key, item]) => [
        key,
        resolveDefault(item, ctx),
      ]),
    );
  }
  if (typeof value !== "string" || !value.includes("${")) return value;
  const lone = value.match(/^\$\{([^}]+)\}$/);
  if (lone) {
    const raw = lookupRaw(lone[1].trim(), ctx);
    if (raw !== undefined && raw !== "") return raw;
  }
  try {
    return interpolate(value, ctx);
  } catch {
    return value;
  }
}

export function schemaDefaults(
  schema?: Schema | null,
  ctx: ResolveContext = {},
): Record<string, unknown> {
  const defaults: Record<string, unknown> = {};
  for (const group of schema?.groups ?? []) {
    for (const field of group.fields ?? []) {
      const d = defaultForField(field, ctx);
      if (d !== undefined) defaults[field.key] = d;
    }
  }
  return defaults;
}

export function mergeSchemaDefaults(
  schema: Schema | null | undefined,
  values?: Record<string, unknown> | null,
): Record<string, unknown> {
  return { ...schemaDefaults(schema), ...(values ?? {}) };
}
