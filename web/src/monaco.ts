import EditorWorker from "monaco-editor/esm/vs/editor/editor.worker?worker";
import JsonWorker from "monaco-editor/esm/vs/language/json/json.worker?worker";
import CssWorker from "monaco-editor/esm/vs/language/css/css.worker?worker";
import HtmlWorker from "monaco-editor/esm/vs/language/html/html.worker?worker";
import TsWorker from "monaco-editor/esm/vs/language/typescript/ts.worker?worker";

export type MonacoModule = typeof import("monaco-editor");

type MonacoWorkerEnvironment = {
  MonacoEnvironment?: {
    getWorker(workerId: string, label: string): Worker;
  };
};

let configured = false;
let themesConfigured = false;

function configureWorkers(): void {
  if (configured) return;
  configured = true;
  (self as unknown as MonacoWorkerEnvironment).MonacoEnvironment = {
    getWorker(_workerId: string, label: string): Worker {
      if (label === "json") return new JsonWorker();
      if (label === "css" || label === "scss" || label === "less") {
        return new CssWorker();
      }
      if (label === "html" || label === "handlebars" || label === "razor") {
        return new HtmlWorker();
      }
      if (label === "typescript" || label === "javascript") {
        return new TsWorker();
      }
      return new EditorWorker();
    },
  };
}

export function currentMonacoTheme(): "shellcn-light" | "shellcn-dark" {
  return document.documentElement.classList.contains("dark")
    ? "shellcn-dark"
    : "shellcn-light";
}

function configureThemes(monaco: MonacoModule): void {
  if (themesConfigured) return;
  themesConfigured = true;
  monaco.editor.defineTheme("shellcn-light", {
    base: "vs",
    inherit: true,
    rules: [],
    colors: {
      "editor.background": "#ffffff",
      "editor.foreground": "#0f172a",
      "editorLineNumber.foreground": "#64748b",
      "editorLineNumber.activeForeground": "#1d4ed8",
      "editorCursor.foreground": "#2563eb",
      "editor.selectionBackground": "#bfdbfe",
      "editor.inactiveSelectionBackground": "#e2e8f0",
      "editor.lineHighlightBackground": "#f8fafc",
      "editorGutter.background": "#ffffff",
    },
  });
  monaco.editor.defineTheme("shellcn-dark", {
    base: "vs-dark",
    inherit: true,
    rules: [],
    colors: {
      "editor.background": "#020617",
      "editor.foreground": "#e2e8f0",
      "editorLineNumber.foreground": "#64748b",
      "editorLineNumber.activeForeground": "#93c5fd",
      "editorCursor.foreground": "#60a5fa",
      "editor.selectionBackground": "#1e40af",
      "editor.inactiveSelectionBackground": "#1e293b",
      "editor.lineHighlightBackground": "#0f172a",
      "editorGutter.background": "#020617",
    },
  });
}

export function syncMonacoTheme(monaco: MonacoModule): void {
  configureThemes(monaco);
  monaco.editor.setTheme(currentMonacoTheme());
}

export async function loadMonaco(): Promise<MonacoModule> {
  configureWorkers();
  const monaco = await import("monaco-editor");
  syncMonacoTheme(monaco);
  return monaco;
}
