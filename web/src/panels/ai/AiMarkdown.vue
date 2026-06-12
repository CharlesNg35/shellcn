<script setup lang="ts">
import { computed } from "vue";
import MarkdownIt from "markdown-it";
import DOMPurify from "dompurify";
import hljs from "highlight.js/lib/common";
import type { RenderRule } from "markdown-it/lib/renderer.mjs";

// Keep markdown dependencies in the lazy AI chunk.
const props = defineProps<{ source: string }>();

function escapeHtml(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

const md: MarkdownIt = new MarkdownIt({
  html: false,
  linkify: true,
  breaks: true,
  highlight(code: string, lang: string): string {
    if (lang && hljs.getLanguage(lang)) {
      try {
        return hljs.highlight(code, { language: lang }).value;
      } catch {
        /* fall through */
      }
    }
    return escapeHtml(code);
  },
});

const renderToken: RenderRule = (tokens, idx, options, _, self) =>
  self.renderToken(tokens, idx, options);

const renderTableOpen = md.renderer.rules.table_open || renderToken;
const renderTableClose = md.renderer.rules.table_close || renderToken;

md.renderer.rules.table_open = (tokens, idx, options, env, self) =>
  `<div class="ai-markdown-table">${renderTableOpen(tokens, idx, options, env, self)}`;

md.renderer.rules.table_close = (tokens, idx, options, env, self) =>
  `${renderTableClose(tokens, idx, options, env, self)}</div>`;

const html = computed(() =>
  DOMPurify.sanitize(md.render(props.source || ""), {
    ADD_ATTR: ["class"],
  }),
);
</script>

<template>
  <!-- Sanitized: markdown-it html:false + DOMPurify before render. -->
  <!-- eslint-disable-next-line vue/no-v-html -->
  <div class="ai-markdown text-sm leading-relaxed" v-html="html" />
</template>

<!-- Global (not scoped): markdown HTML is injected via v-html, so scoped
     `:deep()` rules combined with `:global(.dark)` don't reliably compile to a
     working dark selector. Everything is namespaced under `.ai-markdown`, and
     dark mode uses the same plain `.dark .ai-markdown …` pattern as style.css.
     Surface tokens are one non-flipping scale (0 = light … 950 = dark). -->
<style>
.ai-markdown {
  max-width: 100%;
  min-width: 0;
  overflow-wrap: break-word;
  color: var(--p-surface-800);
}
.dark .ai-markdown {
  color: var(--p-surface-100);
}
.ai-markdown > :first-child {
  margin-top: 0;
}
.ai-markdown > :last-child {
  margin-bottom: 0;
}
.ai-markdown p {
  margin: 0.35rem 0;
}
.ai-markdown a {
  color: var(--p-primary-600);
  text-decoration: underline;
  overflow-wrap: anywhere;
}
.dark .ai-markdown a {
  color: var(--p-primary-400);
}

/* Tailwind preflight strips heading sizes/margins; restore a compact scale so
   markdown structure stays legible inside the bubble. */
.ai-markdown h1,
.ai-markdown h2,
.ai-markdown h3,
.ai-markdown h4,
.ai-markdown h5,
.ai-markdown h6 {
  margin: 0.75rem 0 0.35rem;
  font-weight: 600;
  line-height: 1.3;
}
.ai-markdown h1 {
  font-size: 1.3em;
}
.ai-markdown h2 {
  font-size: 1.2em;
}
.ai-markdown h3 {
  font-size: 1.1em;
}
.ai-markdown h4,
.ai-markdown h5,
.ai-markdown h6 {
  font-size: 1em;
}
.ai-markdown ul,
.ai-markdown ol {
  margin: 0.35rem 0;
  padding-left: 1.25rem;
  list-style: revert;
}
.ai-markdown blockquote {
  margin: 0.5rem 0;
  padding: 0.1rem 0.75rem;
  border-left: 3px solid var(--p-surface-300);
  color: var(--p-surface-500);
}
.dark .ai-markdown blockquote {
  border-left-color: var(--p-surface-600);
  color: var(--p-surface-400);
}
.ai-markdown hr {
  margin: 0.75rem 0;
  border: 0;
  border-top: 1px solid var(--p-surface-200);
}
.dark .ai-markdown hr {
  border-top-color: var(--p-surface-700);
}

.ai-markdown code {
  font-size: 0.85em;
  overflow-wrap: break-word;
  white-space: break-spaces;
}
.ai-markdown :not(pre) > code {
  border-radius: 0.25rem;
  padding: 0.1em 0.35em;
  background: var(--p-surface-100);
}
.dark .ai-markdown :not(pre) > code {
  background: var(--p-surface-800);
}
.ai-markdown pre {
  max-width: 100%;
  overflow-x: auto;
  border-radius: 0.5rem;
  padding: 0.75rem;
  margin: 0.5rem 0;
  background: var(--p-surface-100);
}
.dark .ai-markdown pre {
  background: var(--p-surface-800);
}
.ai-markdown pre code {
  overflow-wrap: normal;
  white-space: pre;
  word-break: normal;
  padding: 0;
  background: transparent;
}

/* Wrapper scrolls only when a wide table can't fit; no per-column sizing — works
   for any shape. */
.ai-markdown .ai-markdown-table {
  width: 100%;
  max-width: 100%;
  margin: 0.5rem 0;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
  scrollbar-color: var(--shell-scrollbar-thumb) transparent;
  scrollbar-width: thin;
}
.ai-markdown .ai-markdown-table:hover {
  scrollbar-color: var(--shell-scrollbar-thumb-hover) transparent;
}
.ai-markdown .ai-markdown-table::-webkit-scrollbar {
  width: 0.5rem;
  height: 0.5rem;
}
.ai-markdown .ai-markdown-table::-webkit-scrollbar-track {
  background: transparent;
}
.ai-markdown .ai-markdown-table::-webkit-scrollbar-thumb {
  border-radius: 9999px;
  background-color: var(--shell-scrollbar-thumb);
}
.ai-markdown .ai-markdown-table:hover::-webkit-scrollbar-thumb {
  background-color: var(--shell-scrollbar-thumb-hover);
}
.ai-markdown table {
  border-collapse: collapse;
  width: 100%;
  font-size: 0.8125rem;
}
/* overflow-wrap: break-word breaks an over-long token only when it can't fit,
   and (unlike the deprecated word-break: break-word / overflow-wrap: anywhere)
   keeps the column's min-content at the longest word — so identifier columns
   stay wide enough to read, and the wrapper scrolls only when truly needed. */
.ai-markdown th,
.ai-markdown td {
  border: 1px solid var(--p-surface-200);
  padding: 0.375rem 0.625rem;
  overflow-wrap: break-word;
  vertical-align: top;
  color: var(--p-surface-800);
}
.dark .ai-markdown th,
.dark .ai-markdown td {
  border-color: var(--p-surface-700);
  color: var(--p-surface-100);
}
.ai-markdown th {
  background: var(--p-surface-100);
  font-weight: 600;
  text-align: left;
}
.dark .ai-markdown th {
  background: var(--p-surface-800);
}
</style>
