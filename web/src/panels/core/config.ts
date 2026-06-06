import { computed, inject, provide, type ComputedRef } from "vue";
import type { PanelType } from "../../types/projection";

export interface PanelConfigProperty {
  type: "string" | "number" | "boolean" | "object" | "array";
  items?: PanelConfigProperty;
  properties?: Record<string, PanelConfigProperty>;
  enum?: string[];
  required?: string[];
}

export interface PanelConfigSchema {
  type: "object";
  properties?: Record<string, PanelConfigProperty>;
  required?: string[];
}

export type PanelConfigSchemas = Record<string, PanelConfigSchema>;

const PANEL_CONFIG_SCHEMAS = Symbol("panel-config-schemas");

export function providePanelConfigSchemas(
  schemas: ComputedRef<PanelConfigSchemas>,
): void {
  provide(PANEL_CONFIG_SCHEMAS, schemas);
}

export function usePanelConfigSchemas(): ComputedRef<PanelConfigSchemas> {
  return (
    inject<ComputedRef<PanelConfigSchemas>>(PANEL_CONFIG_SCHEMAS) ??
    computed(() => ({}))
  );
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function validateProp(
  value: unknown,
  schema: PanelConfigProperty,
  path: string,
): string | null {
  if (value == null) return null;
  if (schema.enum && typeof value === "string" && !schema.enum.includes(value))
    return `${path} must be one of ${schema.enum.join(", ")}.`;
  switch (schema.type) {
    case "string":
      return typeof value === "string" ? null : `${path} must be a string.`;
    case "number":
      return typeof value === "number" ? null : `${path} must be a number.`;
    case "boolean":
      return typeof value === "boolean" ? null : `${path} must be a boolean.`;
    case "array":
      if (!Array.isArray(value)) return `${path} must be an array.`;
      if (!schema.items) return null;
      for (let i = 0; i < value.length; i += 1) {
        const err = validateProp(value[i], schema.items, `${path}[${i}]`);
        if (err) return err;
      }
      return null;
    case "object":
      if (!isRecord(value)) return `${path} must be an object.`;
      return validateObject(value, schema, path);
  }
}

function validateObject(
  value: Record<string, unknown>,
  schema: Pick<PanelConfigSchema, "properties" | "required">,
  path: string,
): string | null {
  const hasDeclaredProperties = schema.properties !== undefined;
  const properties = schema.properties ?? {};
  if (!hasDeclaredProperties && !(schema.required ?? []).length) {
    return null;
  }
  for (const key of schema.required ?? []) {
    if (value[key] == null) return `${path}.${key} is required.`;
  }
  for (const [key, item] of Object.entries(value)) {
    const prop = properties[key] ?? properties["*"];
    if (!prop) return `${path}.${key} is not supported.`;
    const err = validateProp(item, prop, `${path}.${key}`);
    if (err) return err;
  }
  return null;
}

function validateChildPanels(
  panel: PanelType,
  config: Record<string, unknown>,
  schemas: PanelConfigSchemas,
  path: string,
): string | null {
  const key =
    panel === "dashboard" ? "cells" : panel === "split" ? "panels" : "";
  if (!key) return null;
  const children = config[key];
  if (children == null) return null;
  if (!Array.isArray(children)) return `${path}.${key} must be an array.`;
  for (let i = 0; i < children.length; i += 1) {
    const child = children[i];
    const childPath = `${path}.${key}[${i}]`;
    if (!isRecord(child)) return `${childPath} must be an object.`;
    if (typeof child.panel !== "string" || child.panel === "") {
      return `${childPath}.panel must be a non-empty string.`;
    }
    const err = panelConfigError(
      child.panel as PanelType,
      child.config as Record<string, unknown> | undefined,
      schemas,
      `${childPath}.config`,
    );
    if (err) return err;
  }
  return null;
}

export function panelConfigError(
  panel: PanelType,
  config: Record<string, unknown> | undefined,
  schemas: PanelConfigSchemas,
  path = "config",
): string | null {
  if (config == null) return null;
  if (!isRecord(config)) return `${path} must be an object.`;
  const schema = schemas[panel];
  if (!schema) return null;
  return (
    validateObject(config, schema, path) ??
    validateChildPanels(panel, config, schemas, path)
  );
}
