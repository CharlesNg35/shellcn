import "monaco-editor/min/vs/editor/editor.main.css";
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

function currentTheme(): "vs" | "vs-dark" {
  return document.documentElement.classList.contains("dark") ? "vs-dark" : "vs";
}

export function syncMonacoTheme(monaco: MonacoModule): void {
  monaco.editor.setTheme(currentTheme());
}

export async function loadMonaco(): Promise<MonacoModule> {
  configureWorkers();
  const monaco = await import("monaco-editor");
  syncMonacoTheme(monaco);
  return monaco;
}
