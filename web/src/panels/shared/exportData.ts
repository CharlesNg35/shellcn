// Generic, client-side result export shared by every data panel (table grids,
// query results) — plugin-agnostic, no backend involvement. Exports the rows
// currently loaded in the panel.

export type ExportFormat = "csv" | "json";

function cell(value: unknown): string {
  if (value === null || value === undefined) return "";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}

function csvField(value: unknown): string {
  const s = cell(value);
  return /[",\n\r]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
}

export function toCSV(columns: string[], rows: unknown[][]): string {
  const lines = [columns.map(csvField).join(",")];
  for (const row of rows) {
    lines.push(columns.map((_, i) => csvField(row[i])).join(","));
  }
  return lines.join("\r\n");
}

export function toJSON(columns: string[], rows: unknown[][]): string {
  const objects = rows.map((row) =>
    Object.fromEntries(columns.map((c, i) => [c, row[i] ?? null])),
  );
  return JSON.stringify(objects, null, 2);
}

function timestamp(): string {
  return new Date().toISOString().replace(/[:.]/g, "-").slice(0, 19);
}

function download(filename: string, mime: string, content: string): void {
  const blob = new Blob([content], { type: `${mime};charset=utf-8` });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

// exportMatrix downloads column-aligned rows (query results: rows are arrays).
export function exportMatrix(
  name: string,
  columns: string[],
  rows: unknown[][],
  format: ExportFormat,
): void {
  const base = `${name || "export"}-${timestamp()}`;
  if (format === "json") {
    download(`${base}.json`, "application/json", toJSON(columns, rows));
  } else {
    download(`${base}.csv`, "text/csv", toCSV(columns, rows));
  }
}

// exportRecords downloads keyed rows (table grids: rows are objects). Columns
// list the keys to include, in order.
export function exportRecords(
  name: string,
  columns: string[],
  rows: Record<string, unknown>[],
  format: ExportFormat,
): void {
  const matrix = rows.map((row) => columns.map((c) => row[c]));
  exportMatrix(name, columns, matrix, format);
}
