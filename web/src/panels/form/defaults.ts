import type { Schema } from "../../types/projection";

export function schemaDefaults(
  schema?: Schema | null,
): Record<string, unknown> {
  const defaults: Record<string, unknown> = {};
  for (const group of schema?.groups ?? []) {
    for (const field of group.fields ?? []) {
      if (field.default !== undefined) defaults[field.key] = field.default;
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
