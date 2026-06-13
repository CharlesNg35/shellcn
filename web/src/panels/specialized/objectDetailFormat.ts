import type { ColumnType, ObjectDetailField, Row } from "@/types/projection";

export function humanize(key: string): string {
  const spaced = key
    .replace(/[_-]+/g, " ")
    .replace(/([a-z\d])([A-Z])/g, "$1 $2");
  return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}

export function formatBytes(value: number): string {
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let n = value;
  let i = 0;
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024;
    i += 1;
  }
  return `${n.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

export function formatValue(value: unknown, type?: ColumnType): string {
  if (value === undefined || value === null || value === "") return "—";
  const numeric = numberValue(value);
  if (type === "bytes" && numeric != null) return formatBytes(numeric);
  if (type === "percent" && numeric != null) return `${numeric}%`;
  if (type === "datetime" && typeof value === "string") {
    return new Date(value).toLocaleString();
  }
  if (type === "json" || typeof value === "object") {
    return JSON.stringify(value, null, 2);
  }
  return String(value);
}

function numberValue(value: unknown): number | null {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value === "string" && value.trim() !== "") {
    const n = Number(value);
    if (Number.isFinite(n)) return n;
  }
  return null;
}

function clampPercent(value: number | null): number | null {
  if (value == null) return null;
  return Math.max(0, Math.min(100, value));
}

export function valueFor(record: Row, field: ObjectDetailField): unknown {
  return record[field.key];
}

function withUnit(value: string, unit?: string): string {
  return unit && value !== "—" ? `${value} ${unit}` : value;
}

export function usagePercent(
  record: Row,
  field: ObjectDetailField,
): number | null {
  const usage = field.usage;
  if (!usage) return null;
  const direct = numberValue(record[usage.percentKey || field.key]);
  if (direct != null) return clampPercent(direct);
  const used = numberValue(record[usage.usedKey || field.key]);
  const total = numberValue(record[usage.totalKey ?? ""]);
  if (used == null || total == null || total <= 0) return null;
  return clampPercent((used / total) * 100);
}

export function usageMainText(record: Row, field: ObjectDetailField): string {
  const usage = field.usage;
  if (!usage) return formatValue(valueFor(record, field), field.type);

  const percent = usagePercent(record, field);
  const usedKey = usage.usedKey || field.key;
  const totalKey = usage.totalKey;
  const used = formatValue(record[usedKey], usage.usedType ?? field.type);
  if (!totalKey) {
    return percent == null
      ? withUnit(used, usage.unit)
      : `${percent.toFixed(1)}%`;
  }

  const total = withUnit(
    formatValue(record[totalKey], usage.totalType ?? field.type),
    usage.unit,
  );
  const label = usage.totalLabel || "of";
  if (!usage.usedKey) {
    if (percent == null) return `${used} ${label} ${total}`;
    return `${percent.toFixed(1)}% ${label} ${total}`;
  }
  if (percent == null) return `${used} ${label} ${total}`;
  return `${percent.toFixed(1)}% (${used} ${label} ${total})`;
}

export function usageCaption(record: Row, field: ObjectDetailField): string {
  const usage = field.usage;
  if (!usage) return "";
  const usedKey = usage.usedKey || field.key;
  const totalKey = usage.totalKey;
  if (!totalKey || !usage.usedKey) return "";
  const used = formatValue(record[usedKey], usage.usedType ?? field.type);
  const total = withUnit(
    formatValue(record[totalKey], usage.totalType ?? field.type),
    usage.unit,
  );
  return `${used} ${usage.totalLabel || "of"} ${total}`;
}

export function usageToneClass(record: Row, field: ObjectDetailField): string {
  const percent = usagePercent(record, field);
  const usage = field.usage;
  const base = "h-full rounded-full transition-[width] duration-150";
  if (percent == null || !usage) return `${base} bg-primary-500`;
  if (usage.criticalAt && percent >= usage.criticalAt) {
    return `${base} bg-rose-500`;
  }
  if (usage.warnAt && percent >= usage.warnAt) {
    return `${base} bg-amber-500`;
  }
  return `${base} bg-primary-500`;
}
