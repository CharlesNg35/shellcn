import type { PanelType } from "../../types/projection";

const WRITE_METHODS = new Set(["POST", "PUT", "PATCH", "DELETE"]);
const METHODS = new Set(["GET", ...WRITE_METHODS, "WS"]);

type Path = string;

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function optionalRecord(value: unknown, path: Path): string | null {
  if (value == null) return null;
  return isRecord(value) ? null : `${path} must be an object.`;
}

function optionalString(
  config: Record<string, unknown>,
  key: string,
  path: Path,
): string | null {
  return config[key] == null || typeof config[key] === "string"
    ? null
    : `${path}.${key} must be a string.`;
}

function optionalStringArray(
  config: Record<string, unknown>,
  key: string,
  path: Path,
): string | null {
  const value = config[key];
  if (value == null) return null;
  return Array.isArray(value) && value.every((item) => typeof item === "string")
    ? null
    : `${path}.${key} must be an array of strings.`;
}

function optionalStringMap(
  config: Record<string, unknown>,
  key: string,
  path: Path,
): string | null {
  const value = config[key];
  if (value == null) return null;
  if (!isRecord(value)) return `${path}.${key} must be an object.`;
  return Object.values(value).every((item) => typeof item === "string")
    ? null
    : `${path}.${key} values must be strings.`;
}

function optionalMethod(
  config: Record<string, unknown>,
  key: string,
  path: Path,
  allowed = METHODS,
): string | null {
  const value = config[key];
  if (value == null) return null;
  if (typeof value !== "string") return `${path}.${key} must be a string.`;
  return allowed.has(value)
    ? null
    : `${path}.${key} has unsupported method ${JSON.stringify(value)}.`;
}

function validateDataSource(value: unknown, path: Path): string | null {
  if (!isRecord(value)) return `${path} must be an object.`;
  if (typeof value.routeId !== "string" || value.routeId === "") {
    return `${path}.routeId must be a non-empty string.`;
  }
  return (
    optionalMethod(value, "method", path) ??
    optionalStringMap(value, "params", path)
  );
}

function optionalDataSource(
  config: Record<string, unknown>,
  key: string,
  path: Path,
): string | null {
  const value = config[key];
  return value == null ? null : validateDataSource(value, `${path}.${key}`);
}

function validateRouteStrings(
  config: Record<string, unknown>,
  keys: string[],
  path: Path,
): string | null {
  for (const key of keys) {
    const err = optionalString(config, key, path);
    if (err) return err;
  }
  return null;
}

function validateColumns(value: unknown, path: Path): string | null {
  if (value == null) return null;
  if (!Array.isArray(value)) return `${path} must be an array.`;
  for (let i = 0; i < value.length; i += 1) {
    const column = value[i];
    if (!isRecord(column)) return `${path}[${i}] must be an object.`;
    if (typeof column.key !== "string" || column.key === "") {
      return `${path}[${i}].key must be a non-empty string.`;
    }
  }
  return null;
}

function validateKeyedItems(
  config: Record<string, unknown>,
  key: string,
  path: Path,
): string | null {
  const value = config[key];
  if (value == null) return null;
  if (!Array.isArray(value)) return `${path}.${key} must be an array.`;
  for (let i = 0; i < value.length; i += 1) {
    const item = value[i];
    if (!isRecord(item)) return `${path}.${key}[${i}] must be an object.`;
    if (typeof item.key !== "string" || item.key === "") {
      return `${path}.${key}[${i}].key must be a non-empty string.`;
    }
  }
  return null;
}

function validateHeaders(
  config: Record<string, unknown>,
  key: string,
  path: Path,
): string | null {
  const value = config[key];
  if (value == null) return null;
  if (!Array.isArray(value)) return `${path}.${key} must be an array.`;
  for (let i = 0; i < value.length; i += 1) {
    const header = value[i];
    if (!isRecord(header)) return `${path}.${key}[${i}] must be an object.`;
    if (typeof header.key !== "string" || typeof header.value !== "string") {
      return `${path}.${key}[${i}] key and value must be strings.`;
    }
  }
  return null;
}

function validateTableConfig(
  config: Record<string, unknown>,
  path: Path,
): string | null {
  return (
    validateColumns(config.columns, `${path}.columns`) ??
    optionalDataSource(config, "columnsSource", path) ??
    optionalDataSource(config, "watch", path) ??
    optionalDataSource(config, "insert", path) ??
    optionalDataSource(config, "update", path) ??
    optionalDataSource(config, "delete", path) ??
    optionalStringArray(config, "actionIds", path) ??
    optionalStringArray(config, "rowActionIds", path) ??
    optionalStringArray(config, "rowKey", path) ??
    optionalStringArray(config, "hiddenColumns", path)
  );
}

function validateFileBrowserConfig(
  config: Record<string, unknown>,
  path: Path,
): string | null {
  return validateRouteStrings(
    config,
    [
      "pathParam",
      "readRouteId",
      "downloadRouteId",
      "writeRouteId",
      "uploadRouteId",
      "mkdirRouteId",
      "renameRouteId",
      "deleteRouteId",
      "uploadFieldName",
    ],
    path,
  );
}

function validateFormConfig(
  config: Record<string, unknown>,
  path: Path,
): string | null {
  return (
    optionalString(config, "submitRouteId", path) ??
    optionalMethod(config, "submitMethod", path, WRITE_METHODS) ??
    optionalStringMap(config, "params", path)
  );
}

function validateCodeEditorConfig(
  config: Record<string, unknown>,
  path: Path,
): string | null {
  return (
    optionalString(config, "saveRouteId", path) ??
    optionalMethod(config, "saveMethod", path, WRITE_METHODS) ??
    optionalStringMap(config, "saveParams", path) ??
    optionalString(config, "saveBodyKey", path) ??
    optionalRecord(config.saveExtra, `${path}.saveExtra`)
  );
}

function validateQueryEditorConfig(
  config: Record<string, unknown>,
  path: Path,
): string | null {
  return (
    validateRouteStrings(
      config,
      ["cancelRouteId", "completionRouteId"],
      path,
    ) ??
    optionalStringMap(config, "cancelParams", path) ??
    optionalStringMap(config, "completionParams", path)
  );
}

function validateDashboardConfig(
  config: Record<string, unknown>,
  path: Path,
): string | null {
  if (config.cells == null) return null;
  if (!Array.isArray(config.cells)) return `${path}.cells must be an array.`;
  for (let i = 0; i < config.cells.length; i += 1) {
    const cell = config.cells[i];
    const cellPath = `${path}.cells[${i}]`;
    if (!isRecord(cell)) return `${cellPath} must be an object.`;
    if (typeof cell.key !== "string" || cell.key === "") {
      return `${cellPath}.key must be a non-empty string.`;
    }
    if (typeof cell.panel !== "string" || cell.panel === "") {
      return `${cellPath}.panel must be a non-empty string.`;
    }
    const sourceErr =
      cell.source == null
        ? null
        : validateDataSource(cell.source, `${cellPath}.source`);
    if (sourceErr) return sourceErr;
    const configErr = panelConfigError(
      cell.panel as PanelType,
      cell.config as Record<string, unknown> | undefined,
      `${cellPath}.config`,
    );
    if (configErr) return configErr;
  }
  return null;
}

function validateMetricsConfig(
  config: Record<string, unknown>,
  path: Path,
): string | null {
  return (
    validateKeyedItems(config, "stats", path) ??
    validateKeyedItems(config, "gauges", path) ??
    validateKeyedItems(config, "series", path)
  );
}

function validateKVConfig(
  config: Record<string, unknown>,
  path: Path,
): string | null {
  return (
    validateRouteStrings(
      config,
      [
        "createRouteId",
        "readRouteId",
        "writeRouteId",
        "deleteRouteId",
        "keyParam",
      ],
      path,
    ) ?? optionalStringArray(config, "valueTypes", path)
  );
}

function validateHTTPClientConfig(
  config: Record<string, unknown>,
  path: Path,
): string | null {
  return (
    optionalString(config, "executeRouteId", path) ??
    optionalStringArray(config, "methods", path) ??
    optionalMethod(config, "defaultMethod", path) ??
    validateHeaders(config, "defaultHeaders", path)
  );
}

function validateGraphConfig(
  config: Record<string, unknown>,
  path: Path,
): string | null {
  if (
    config.layout != null &&
    config.layout !== "grid" &&
    config.layout !== "manual"
  ) {
    return `${path}.layout must be "grid" or "manual".`;
  }
  return null;
}

export function panelConfigError(
  panel: PanelType,
  config: Record<string, unknown> | undefined,
  path = "config",
): string | null {
  const recordErr = optionalRecord(config, path);
  if (recordErr) return recordErr;
  const cfg = config ?? {};
  switch (panel) {
    case "table":
      return validateTableConfig(cfg, path);
    case "file_browser":
      return validateFileBrowserConfig(cfg, path);
    case "form":
      return validateFormConfig(cfg, path);
    case "code_editor":
      return validateCodeEditorConfig(cfg, path);
    case "query_editor":
      return validateQueryEditorConfig(cfg, path);
    case "dashboard":
      return validateDashboardConfig(cfg, path);
    case "metrics":
      return validateMetricsConfig(cfg, path);
    case "kv":
      return validateKVConfig(cfg, path);
    case "http_client":
      return validateHTTPClientConfig(cfg, path);
    case "graph":
      return validateGraphConfig(cfg, path);
    default:
      return null;
  }
}
