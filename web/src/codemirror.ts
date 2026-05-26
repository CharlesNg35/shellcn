import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import { json } from "@codemirror/lang-json";
import { sql } from "@codemirror/lang-sql";
import { yaml } from "@codemirror/lang-yaml";
import {
  bracketMatching,
  defaultHighlightStyle,
  HighlightStyle,
  indentOnInput,
  StreamLanguage,
  syntaxHighlighting,
} from "@codemirror/language";
import { Compartment, EditorState, type Extension } from "@codemirror/state";
import {
  drawSelection,
  dropCursor,
  highlightActiveLine,
  highlightActiveLineGutter,
  highlightSpecialChars,
  keymap,
  lineNumbers,
  EditorView,
} from "@codemirror/view";
import { tags } from "@lezer/highlight";
import { dockerFile } from "@codemirror/legacy-modes/mode/dockerfile";
import { http } from "@codemirror/legacy-modes/mode/http";
import { nginx } from "@codemirror/legacy-modes/mode/nginx";
import { powerShell } from "@codemirror/legacy-modes/mode/powershell";
import { properties } from "@codemirror/legacy-modes/mode/properties";
import { shell } from "@codemirror/legacy-modes/mode/shell";
import { toml } from "@codemirror/legacy-modes/mode/toml";
import { xml } from "@codemirror/legacy-modes/mode/xml";

export interface CodeMirrorEditor {
  view: EditorView;
  language: Compartment;
  readOnly: Compartment;
  theme: Compartment;
}

export interface CodeMirrorOptions {
  value: string;
  language?: string;
  readOnly?: boolean;
  ariaLabel?: string;
  onChange?: (value: string) => void;
}

const lightHighlight = syntaxHighlighting(
  HighlightStyle.define([
    { tag: tags.keyword, color: "#1d4ed8" },
    { tag: tags.atom, color: "#7c3aed" },
    { tag: tags.number, color: "#b45309" },
    { tag: tags.string, color: "#047857" },
    { tag: tags.comment, color: "#64748b", fontStyle: "italic" },
    { tag: tags.variableName, color: "#0f172a" },
    { tag: tags.propertyName, color: "#0f766e" },
    { tag: tags.operator, color: "#475569" },
    { tag: tags.punctuation, color: "#475569" },
    { tag: tags.invalid, color: "#dc2626" },
  ]),
  { fallback: true },
);

const darkHighlight = syntaxHighlighting(
  HighlightStyle.define([
    { tag: tags.keyword, color: "#93c5fd" },
    { tag: tags.atom, color: "#c4b5fd" },
    { tag: tags.number, color: "#fbbf24" },
    { tag: tags.string, color: "#86efac" },
    { tag: tags.comment, color: "#94a3b8", fontStyle: "italic" },
    { tag: tags.variableName, color: "#e2e8f0" },
    { tag: tags.propertyName, color: "#5eead4" },
    { tag: tags.operator, color: "#cbd5e1" },
    { tag: tags.punctuation, color: "#cbd5e1" },
    { tag: tags.invalid, color: "#f87171" },
  ]),
  { fallback: true },
);

const editorSetup: Extension = [
  lineNumbers(),
  highlightActiveLineGutter(),
  highlightSpecialChars(),
  history(),
  drawSelection(),
  dropCursor(),
  indentOnInput(),
  syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
  bracketMatching(),
  highlightActiveLine(),
  keymap.of([...defaultKeymap, ...historyKeymap]),
];

const lightTheme = EditorView.theme({
  "&": {
    height: "100%",
    color: "#0f172a",
    backgroundColor: "#ffffff",
  },
  ".cm-scroller": {
    fontFamily: "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace",
    fontSize: "12px",
    lineHeight: "1.65",
  },
  ".cm-content": {
    minHeight: "100%",
    padding: "12px 0",
  },
  ".cm-line": {
    padding: "0 16px",
  },
  ".cm-gutters": {
    color: "#64748b",
    backgroundColor: "#ffffff",
    borderRight: "1px solid #e2e8f0",
  },
  ".cm-activeLine": {
    backgroundColor: "#f8fafc",
  },
  ".cm-activeLineGutter": {
    color: "#1d4ed8",
    backgroundColor: "#eff6ff",
  },
  "&.cm-focused": {
    outline: "none",
  },
  "&.cm-focused .cm-cursor": {
    borderLeftColor: "#2563eb",
  },
  "&.cm-focused .cm-selectionBackground, .cm-selectionBackground, ::selection":
    {
      backgroundColor: "#bfdbfe",
    },
});

const darkTheme = EditorView.theme(
  {
    "&": {
      height: "100%",
      color: "#e2e8f0",
      backgroundColor: "#020617",
    },
    ".cm-scroller": {
      fontFamily: "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace",
      fontSize: "12px",
      lineHeight: "1.65",
    },
    ".cm-content": {
      minHeight: "100%",
      padding: "12px 0",
    },
    ".cm-line": {
      padding: "0 16px",
    },
    ".cm-gutters": {
      color: "#94a3b8",
      backgroundColor: "#020617",
      borderRight: "1px solid #1e293b",
    },
    ".cm-activeLine": {
      backgroundColor: "#0f172a",
    },
    ".cm-activeLineGutter": {
      color: "#93c5fd",
      backgroundColor: "#172554",
    },
    "&.cm-focused": {
      outline: "none",
    },
    "&.cm-focused .cm-cursor": {
      borderLeftColor: "#60a5fa",
    },
    "&.cm-focused .cm-selectionBackground, .cm-selectionBackground, ::selection":
      {
        backgroundColor: "#1e40af",
      },
  },
  { dark: true },
);

function isDark(): boolean {
  return document.documentElement.classList.contains("dark");
}

export function currentCodeMirrorTheme(): Extension {
  return isDark() ? [darkTheme, darkHighlight] : [lightTheme, lightHighlight];
}

export function languageExtension(language?: string): Extension {
  const normalized = (language ?? "plaintext").toLowerCase().trim();
  switch (normalized) {
    case "json":
    case "jsonc":
    case "jsonl":
      return json();
    case "yaml":
    case "yml":
      return yaml();
    case "sql":
    case "postgres":
    case "postgresql":
    case "mysql":
    case "mariadb":
    case "sqlite":
    case "mssql":
    case "oracle":
    case "cql":
    case "cypher":
    case "promql":
    case "metricsql":
      return sql();
    case "shell":
    case "sh":
    case "bash":
    case "zsh":
    case "fish":
      return StreamLanguage.define(shell);
    case "powershell":
    case "ps1":
      return StreamLanguage.define(powerShell);
    case "dockerfile":
    case "docker":
      return StreamLanguage.define(dockerFile);
    case "nginx":
      return StreamLanguage.define(nginx);
    case "toml":
      return StreamLanguage.define(toml);
    case "ini":
    case "conf":
    case "config":
    case "env":
    case "properties":
    case "editorconfig":
    case "gitignore":
    case "service":
    case "socket":
    case "timer":
      return StreamLanguage.define(properties);
    case "xml":
    case "html":
    case "svg":
      return StreamLanguage.define(xml);
    case "http":
      return StreamLanguage.define(http);
    default:
      return [];
  }
}

export function readOnlyExtension(readOnly: boolean): Extension {
  return [
    EditorState.readOnly.of(readOnly),
    EditorView.editable.of(!readOnly),
    readOnly ? EditorView.contentAttributes.of({ tabindex: "0" }) : [],
  ];
}

export function createCodeMirrorEditor(
  parent: HTMLElement,
  options: CodeMirrorOptions,
): CodeMirrorEditor {
  const language = new Compartment();
  const readOnly = new Compartment();
  const theme = new Compartment();
  const state = EditorState.create({
    doc: options.value,
    extensions: [
      editorSetup,
      EditorView.lineWrapping,
      EditorView.contentAttributes.of({
        "aria-label": options.ariaLabel ?? "Code editor",
      }),
      EditorView.updateListener.of((update) => {
        if (update.docChanged) options.onChange?.(update.state.doc.toString());
      }),
      language.of(languageExtension(options.language)),
      readOnly.of(readOnlyExtension(options.readOnly === true)),
      theme.of(currentCodeMirrorTheme()),
    ],
  });
  return {
    view: new EditorView({ state, parent }),
    language,
    readOnly,
    theme,
  };
}

export function editorValue(editor: CodeMirrorEditor | null): string {
  return editor?.view.state.doc.toString() ?? "";
}

export function setEditorValue(
  editor: CodeMirrorEditor | null,
  value: string,
): void {
  if (!editor || editor.view.state.doc.toString() === value) return;
  editor.view.dispatch({
    changes: { from: 0, to: editor.view.state.doc.length, insert: value },
  });
}

export function setEditorLanguage(
  editor: CodeMirrorEditor | null,
  language: string | undefined,
): void {
  editor?.view.dispatch({
    effects: editor.language.reconfigure(languageExtension(language)),
  });
}

export function setEditorReadOnly(
  editor: CodeMirrorEditor | null,
  readOnly: boolean,
): void {
  editor?.view.dispatch({
    effects: editor.readOnly.reconfigure(readOnlyExtension(readOnly)),
  });
}

export function syncCodeMirrorTheme(editor: CodeMirrorEditor | null): void {
  editor?.view.dispatch({
    effects: editor.theme.reconfigure(currentCodeMirrorTheme()),
  });
}
