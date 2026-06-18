import type {
  Column as ColumnSpec,
  ColumnEditor as ColumnEditorValue,
} from "@/types/projection";
import { ColumnEditor, ColumnType } from "@/types/projection";

export function fullCellText(value: unknown): string {
  if (value === undefined || value === null || value === "") return "—";
  if (typeof value === "object") {
    try {
      return JSON.stringify(value, null, 2);
    } catch {
      return String(value);
    }
  }
  return String(value);
}

export function structuredSummary(value: unknown): string {
  if (Array.isArray(value)) return `[${value.length} items]`;
  if (value && typeof value === "object")
    return `{${Object.keys(value as Record<string, unknown>).length} keys}`;
  return fullCellText(value);
}

export function isStructuredValue(value: unknown): boolean {
  return value !== null && typeof value === "object";
}

export function isCellEditable(col: ColumnSpec): boolean {
  return col.editable === true && col.readOnly !== true && Boolean(col.editor);
}

export function isInlineEditor(col: ColumnSpec): boolean {
  return isCellEditable(col) && col.editor !== ColumnEditor.Json;
}

export function isJsonEditor(col: ColumnSpec): boolean {
  return isCellEditable(col) && col.editor === ColumnEditor.Json;
}

export function writableColumns(columns: ColumnSpec[]): ColumnSpec[] {
  return columns.filter(isCellEditable);
}

export function coerceCellValue(
  col: ColumnSpec,
  prev: unknown,
  next: unknown,
): unknown {
  if (next === "" || next === undefined) return col.nullable ? null : "";
  switch (col.editor) {
    case ColumnEditor.Number: {
      if (next === null) return col.nullable ? null : prev;
      const n = Number(next);
      return Number.isNaN(n) ? next : n;
    }
    case ColumnEditor.Toggle:
      return next === true || next === "true";
    default:
      return next;
  }
}

export function defaultColumnEditor(rawType: unknown): ColumnEditorValue {
  const t = String(rawType ?? "").toLowerCase();
  if (/bool/.test(t)) return ColumnEditor.Toggle;
  if (/json/.test(t)) return ColumnEditor.Json;
  if (/(int|serial|numeric|decimal|real|double|float|money|number)/.test(t))
    return ColumnEditor.Number;
  return ColumnEditor.Text;
}

export function defaultColumnType(
  rawType: unknown,
): ColumnSpec["type"] | undefined {
  const t = String(rawType ?? "").toLowerCase();
  if (!t) return undefined;
  if (/bool/.test(t)) return ColumnType.Bool;
  if (/json/.test(t)) return ColumnType.Json;
  if (/(int|serial|numeric|decimal|real|double|float|money|number)/.test(t))
    return ColumnType.Number;
  if (/(date|time|timestamp)/.test(t)) return ColumnType.DateTime;
  return undefined;
}

export function cellValueEquals(a: unknown, b: unknown): boolean {
  if (Object.is(a, b)) return true;
  if (!isStructuredValue(a) || !isStructuredValue(b)) return false;
  return fullCellText(a) === fullCellText(b);
}
