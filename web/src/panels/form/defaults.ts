import { FieldType, type Field, type Schema } from "@/types/projection";

// defaultForField seeds an initial value, recursing object sub-fields and the
// minItems rows of an array so a composite field renders ready to edit.
export function defaultForField(field: Field): unknown {
  if (field.type === FieldType.Object) {
    const out: Record<string, unknown> = {};
    for (const sub of field.fields ?? []) {
      const d = defaultForField(sub);
      if (d !== undefined) out[sub.key] = d;
    }
    return out;
  }
  if (field.type === FieldType.Array) {
    const n = field.minItems ?? 0;
    return field.item
      ? Array.from({ length: n }, () => defaultForField(field.item as Field))
      : [];
  }
  if (field.type === FieldType.Map) {
    return field.default ?? {};
  }
  return field.default;
}

export function schemaDefaults(
  schema?: Schema | null,
): Record<string, unknown> {
  const defaults: Record<string, unknown> = {};
  for (const group of schema?.groups ?? []) {
    for (const field of group.fields ?? []) {
      const d = defaultForField(field);
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
