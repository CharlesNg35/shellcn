import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import { MergeView, unifiedMergeView } from "@codemirror/merge";
import {
  autocompletion,
  type Completion,
  type CompletionContext,
  type CompletionResult,
  type CompletionSource,
} from "@codemirror/autocomplete";
import { json } from "@codemirror/lang-json";
import {
  schemaCompletionSource,
  sql,
  type SQLNamespace,
} from "@codemirror/lang-sql";
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
  completions: Compartment;
}

export interface CodeMirrorCompletion {
  label: string;
  type?: string;
  detail?: string;
  apply?: string;
}

export interface CodeMirrorOptions {
  value: string;
  language?: string;
  readOnly?: boolean;
  ariaLabel?: string;
  completions?: CodeMirrorCompletion[];
  onChange?: (value: string) => void;
}

export type CodeMirrorDiffMode = "side_by_side" | "unified";

export interface CodeMirrorDiffView {
  destroy: () => void;
  syncTheme: () => void;
}

export interface CodeMirrorDiffOptions {
  original: string;
  modified: string;
  language?: string;
  mode?: CodeMirrorDiffMode;
  collapseUnchanged?: boolean;
}

interface HighlightPalette {
  keyword: string; // keywords, modifiers
  func: string; // function names, labels
  property: string; // YAML keys, object props, tags, attribute names
  string: string; // strings + YAML plain scalar values
  number: string; // numbers, booleans, null, constants
  type: string; // type names, classes, namespaces
  operator: string; // operators, regexp, escapes, URLs
  comment: string; // comments, doc markers
  variable: string; // identifiers, punctuation (≈ foreground)
  invalid: string;
}

// Includes `content`, lang-yaml's tag for plain scalar values.
function buildHighlight(c: HighlightPalette): Extension {
  return syntaxHighlighting(
    HighlightStyle.define([
      {
        tag: [
          tags.keyword,
          tags.controlKeyword,
          tags.operatorKeyword,
          tags.moduleKeyword,
          tags.definitionKeyword,
          tags.modifier,
          tags.self,
        ],
        color: c.keyword,
      },
      {
        tag: [tags.function(tags.variableName), tags.labelName],
        color: c.func,
      },
      {
        tag: [
          tags.propertyName,
          tags.attributeName,
          tags.tagName,
          tags.macroName,
          tags.character,
          tags.deleted,
        ],
        color: c.property,
      },
      {
        tag: [
          tags.string,
          tags.content,
          tags.attributeValue,
          tags.inserted,
          tags.special(tags.string),
        ],
        color: c.string,
      },
      { tag: [tags.number, tags.bool, tags.null, tags.atom], color: c.number },
      {
        tag: [
          tags.typeName,
          tags.className,
          tags.namespace,
          tags.annotation,
          tags.changed,
        ],
        color: c.type,
      },
      {
        tag: [
          tags.operator,
          tags.derefOperator,
          tags.regexp,
          tags.escape,
          tags.url,
          tags.link,
        ],
        color: c.operator,
      },
      { tag: tags.comment, color: c.comment, fontStyle: "italic" },
      { tag: tags.meta, color: c.comment },
      {
        tag: [
          tags.variableName,
          tags.punctuation,
          tags.separator,
          tags.brace,
          tags.squareBracket,
          tags.angleBracket,
          tags.paren,
        ],
        color: c.variable,
      },
      { tag: tags.invalid, color: c.invalid },
    ]),
  );
}

const lightHighlight = buildHighlight({
  keyword: "#a626a4",
  func: "#4078f2",
  property: "#e45649",
  string: "#50a14f",
  number: "#986801",
  type: "#c18401",
  operator: "#0184bc",
  comment: "#a0a1a7",
  variable: "#383a42",
  invalid: "#e45649",
});

const darkHighlight = buildHighlight({
  keyword: "#c678dd",
  func: "#61afef",
  property: "#e06c75",
  string: "#98c379",
  number: "#d19a66",
  type: "#e5c07b",
  operator: "#56b6c2",
  comment: "#7d8799",
  variable: "#abb2bf",
  invalid: "#e06c75",
});

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

interface EditorPalette {
  background: string;
  foreground: string;
  caret: string;
  selection: string;
  selectionFocused: string;
  gutterForeground: string;
  gutterBorder: string;
  activeLine: string;
  activeLineGutterForeground: string;
  activeLineGutterBackground: string;
  matchingBracket: string;
  scrollbarThumb: string;
  scrollbarThumbHover: string;
  tooltipBackground: string;
  tooltipForeground: string;
  tooltipBorder: string;
  tooltipShadow: string;
  completionSelected: string;
  completionSelectedForeground: string;
  completionDetail: string;
  completionMatched: string;
}

const monoFont = "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace";

// Match CodeMirror's selector specificity so these theme colors win cleanly.
function editorTheme(c: EditorPalette, dark: boolean): Extension {
  return EditorView.theme(
    {
      "&": {
        height: "100%",
        color: c.foreground,
        backgroundColor: c.background,
      },
      ".cm-scroller": {
        fontFamily: monoFont,
        fontSize: "12px",
        lineHeight: "1.65",
        scrollbarWidth: "thin",
        scrollbarColor: `${c.scrollbarThumb} transparent`,
      },
      ".cm-scroller::-webkit-scrollbar": { width: "8px", height: "8px" },
      ".cm-scroller::-webkit-scrollbar-thumb": {
        backgroundColor: c.scrollbarThumb,
        borderRadius: "999px",
        border: "2px solid transparent",
        backgroundClip: "padding-box",
      },
      ".cm-scroller::-webkit-scrollbar-track": { background: "transparent" },
      ".cm-scroller::-webkit-scrollbar-corner": { background: "transparent" },
      ".cm-scroller::-webkit-scrollbar-thumb:hover": {
        backgroundColor: c.scrollbarThumbHover,
      },
      ".cm-content": {
        minHeight: "100%",
        padding: "12px 0",
        caretColor: c.caret,
      },
      ".cm-line": { padding: "0 16px" },
      ".cm-gutters": {
        color: c.gutterForeground,
        backgroundColor: c.background,
        borderRight: `1px solid ${c.gutterBorder}`,
      },
      ".cm-activeLine": { backgroundColor: c.activeLine },
      ".cm-activeLineGutter": {
        color: c.activeLineGutterForeground,
        backgroundColor: c.activeLineGutterBackground,
      },
      "&.cm-focused": { outline: "none" },
      ".cm-cursor, .cm-dropCursor": {
        borderLeftColor: c.caret,
        borderLeftWidth: "2px",
      },
      ".cm-selectionBackground, .cm-content ::selection": {
        backgroundColor: c.selection,
      },
      "&.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground":
        {
          backgroundColor: c.selectionFocused,
        },
      "&.cm-focused .cm-matchingBracket": {
        backgroundColor: c.matchingBracket,
        outline: `1px solid ${c.caret}`,
        borderRadius: "2px",
      },
      "&.cm-focused .cm-nonmatchingBracket": {
        backgroundColor: "transparent",
        color: "#ef4444",
      },
      ".cm-tooltip": {
        border: `1px solid ${c.tooltipBorder}`,
        borderRadius: "8px",
        backgroundColor: c.tooltipBackground,
        color: c.tooltipForeground,
        boxShadow: c.tooltipShadow,
        overflow: "hidden",
      },
      ".cm-tooltip .cm-tooltip-arrow:before": {
        borderTopColor: c.tooltipBorder,
        borderBottomColor: c.tooltipBorder,
      },
      ".cm-tooltip .cm-tooltip-arrow:after": {
        borderTopColor: c.tooltipBackground,
        borderBottomColor: c.tooltipBackground,
      },
      ".cm-tooltip.cm-tooltip-autocomplete > ul": {
        fontFamily: monoFont,
        fontSize: "12px",
        maxHeight: "14em",
        minWidth: "14em",
        padding: "4px",
      },
      ".cm-tooltip.cm-tooltip-autocomplete > ul > li": {
        padding: "2px 8px",
        lineHeight: "1.5",
        borderRadius: "4px",
      },
      ".cm-tooltip-autocomplete ul li[aria-selected]": {
        backgroundColor: c.completionSelected,
        color: c.completionSelectedForeground,
      },
      ".cm-completionIcon": { color: c.completionDetail, opacity: "0.8" },
      ".cm-completionDetail": {
        color: c.completionDetail,
        fontStyle: "italic",
      },
      ".cm-completionMatchedText": {
        color: c.completionMatched,
        textDecoration: "none",
        fontWeight: "600",
      },
      ".cm-tooltip-autocomplete ul li[aria-selected] .cm-completionIcon, .cm-tooltip-autocomplete ul li[aria-selected] .cm-completionDetail, .cm-tooltip-autocomplete ul li[aria-selected] .cm-completionMatchedText":
        {
          color: "inherit",
          opacity: "1",
        },
    },
    { dark },
  );
}

const lightPalette: EditorPalette = {
  background: "#ffffff",
  foreground: "#0f172a",
  caret: "#2563eb",
  selection: "rgba(37, 99, 235, 0.14)",
  selectionFocused: "rgba(37, 99, 235, 0.24)",
  gutterForeground: "#94a3b8",
  gutterBorder: "#e2e8f0",
  activeLine: "rgba(148, 163, 184, 0.1)",
  activeLineGutterForeground: "#1d4ed8",
  activeLineGutterBackground: "#eff6ff",
  matchingBracket: "rgba(37, 99, 235, 0.16)",
  scrollbarThumb: "#cbd5e1",
  scrollbarThumbHover: "#94a3b8",
  tooltipBackground: "#ffffff",
  tooltipForeground: "#334155",
  tooltipBorder: "#e2e8f0",
  tooltipShadow:
    "0 0 0 1px rgba(2, 6, 23, 0.05), 0 10px 15px -3px rgba(0, 0, 0, 0.1), 0 4px 6px -4px rgba(0, 0, 0, 0.1)",
  completionSelected: "#eff6ff",
  completionSelectedForeground: "#1d4ed8",
  completionDetail: "#64748b",
  completionMatched: "#1d4ed8",
};

const darkPalette: EditorPalette = {
  background: "#020617",
  foreground: "#e2e8f0",
  caret: "#60a5fa",
  selection: "rgba(59, 130, 246, 0.24)",
  selectionFocused: "rgba(59, 130, 246, 0.4)",
  gutterForeground: "#64748b",
  gutterBorder: "#1e293b",
  activeLine: "rgba(148, 163, 184, 0.08)",
  activeLineGutterForeground: "#93c5fd",
  activeLineGutterBackground: "rgba(59, 130, 246, 0.15)",
  matchingBracket: "rgba(96, 165, 250, 0.28)",
  scrollbarThumb: "#334155",
  scrollbarThumbHover: "#475569",
  tooltipBackground: "#0f172a",
  tooltipForeground: "#e2e8f0",
  tooltipBorder: "#334155",
  tooltipShadow:
    "0 0 0 1px rgba(255, 255, 255, 0.05), 0 10px 15px -3px rgba(0, 0, 0, 0.4), 0 4px 6px -4px rgba(0, 0, 0, 0.4)",
  completionSelected: "rgba(59, 130, 246, 0.15)",
  completionSelectedForeground: "#93c5fd",
  completionDetail: "#94a3b8",
  completionMatched: "#93c5fd",
};

const lightTheme = editorTheme(lightPalette, false);
const darkTheme = editorTheme(darkPalette, true);

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

const SQL_LANGUAGES = new Set([
  "sql",
  "postgres",
  "postgresql",
  "mysql",
  "mariadb",
  "sqlite",
  "mssql",
  "oracle",
  "cql",
]);

function toCompletion(item: CodeMirrorCompletion): Completion {
  return {
    label: item.label,
    type: item.type,
    detail: item.detail,
    apply: item.apply,
  };
}

// buildSqlSchema turns the flat completion catalog into a lang-sql namespace
// (table -> columns, plus schema -> table -> columns), enabling context-aware
// completion: tables after FROM/JOIN and columns after `table.`/`schema.table.`.
export function buildSqlSchema(
  items: CodeMirrorCompletion[],
): Record<string, SQLNamespace> {
  const tables: Record<string, Set<string>> = {};
  const schemas: Record<string, Record<string, Set<string>>> = {};
  const ensure = (rec: Record<string, Set<string>>, key: string) =>
    (rec[key] ??= new Set<string>());
  for (const item of items) {
    if ((item.type === "table" || item.type === "view") && item.detail) {
      ensure(tables, item.label);
      (schemas[item.detail] ??= {})[item.label] ??= new Set();
    } else if (item.type === "property" && item.detail) {
      const dot = item.detail.indexOf(".");
      const schema = dot >= 0 ? item.detail.slice(0, dot) : "";
      const relation = dot >= 0 ? item.detail.slice(dot + 1) : item.detail;
      ensure(tables, relation).add(item.label);
      if (schema) ensure((schemas[schema] ??= {}), relation).add(item.label);
    }
  }
  const out: Record<string, SQLNamespace> = {};
  for (const [table, cols] of Object.entries(tables)) out[table] = [...cols];
  for (const [schema, rels] of Object.entries(schemas)) {
    out[schema] = Object.fromEntries(
      Object.entries(rels).map(([rel, cols]) => [rel, [...cols]]),
    );
  }
  return out;
}

function completionExtension(
  items?: CodeMirrorCompletion[],
  language?: string,
): Extension {
  if (!items?.length) return [];
  const sources: CompletionSource[] = [];
  if (language && SQL_LANGUAGES.has(language)) {
    const schema = buildSqlSchema(items);
    if (Object.keys(schema).length) {
      sources.push(schemaCompletionSource({ schema }));
    }
    // Keep keywords/functions (and a flat fallback) from the catalog; the schema
    // source already owns tables/columns, so drop those to avoid duplicates.
    const extras = items.filter(
      (i) => i.type !== "table" && i.type !== "view" && i.type !== "property",
    );
    if (extras.length) sources.push(completionSource(extras.map(toCompletion)));
  } else {
    sources.push(completionSource(items.map(toCompletion)));
  }
  return autocompletion({
    override: sources,
    activateOnTyping: true,
    activateOnTypingDelay: 150,
    maxRenderedOptions: 80,
  });
}

function completionSource(options: Completion[]) {
  return (ctx: CompletionContext): CompletionResult | null => {
    const word = ctx.matchBefore(/[A-Za-z_][\w.$]*/);
    if (!word && !ctx.explicit) return null;
    return {
      from: word?.from ?? ctx.pos,
      options,
      validFor: /^[\w.$]*$/,
    };
  };
}

export function createCodeMirrorEditor(
  parent: HTMLElement,
  options: CodeMirrorOptions,
): CodeMirrorEditor {
  const language = new Compartment();
  const readOnly = new Compartment();
  const theme = new Compartment();
  const completions = new Compartment();
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
      completions.of(
        completionExtension(options.completions, options.language),
      ),
    ],
  });
  return {
    view: new EditorView({ state, parent }),
    language,
    readOnly,
    theme,
    completions,
  };
}

function diffCollapse(
  collapse?: boolean,
): { margin?: number; minSize?: number } | undefined {
  return collapse ? { margin: 3, minSize: 8 } : undefined;
}

function diffEditorExtensions(
  language: string | undefined,
  theme: Compartment,
  ariaLabel: string,
): Extension[] {
  return [
    editorSetup,
    EditorView.lineWrapping,
    EditorView.contentAttributes.of({ "aria-label": ariaLabel }),
    languageExtension(language),
    readOnlyExtension(true),
    theme.of(currentCodeMirrorTheme()),
  ];
}

export function createCodeMirrorDiffView(
  parent: HTMLElement,
  options: CodeMirrorDiffOptions,
): CodeMirrorDiffView {
  if (options.mode === "unified") {
    const theme = new Compartment();
    const view = new EditorView({
      parent,
      state: EditorState.create({
        doc: options.modified,
        extensions: [
          ...diffEditorExtensions(options.language, theme, "Modified content"),
          unifiedMergeView({
            original: options.original,
            gutter: true,
            collapseUnchanged: diffCollapse(options.collapseUnchanged),
          }),
        ],
      }),
    });
    return {
      destroy: () => view.destroy(),
      syncTheme: () => {
        view.dispatch({
          effects: theme.reconfigure(currentCodeMirrorTheme()),
        });
      },
    };
  }

  const originalTheme = new Compartment();
  const modifiedTheme = new Compartment();
  const view = new MergeView({
    a: {
      doc: options.original,
      extensions: diffEditorExtensions(
        options.language,
        originalTheme,
        "Original content",
      ),
    },
    b: {
      doc: options.modified,
      extensions: diffEditorExtensions(
        options.language,
        modifiedTheme,
        "Modified content",
      ),
    },
    parent,
    gutter: true,
    collapseUnchanged: diffCollapse(options.collapseUnchanged),
  });
  return {
    destroy: () => view.destroy(),
    syncTheme: () => {
      view.a.dispatch({
        effects: originalTheme.reconfigure(currentCodeMirrorTheme()),
      });
      view.b.dispatch({
        effects: modifiedTheme.reconfigure(currentCodeMirrorTheme()),
      });
    },
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

export function setEditorCompletions(
  editor: CodeMirrorEditor | null,
  completions: CodeMirrorCompletion[],
  language?: string,
): void {
  editor?.view.dispatch({
    effects: editor.completions.reconfigure(
      completionExtension(completions, language),
    ),
  });
}
