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

<style scoped>
.ai-markdown {
  container-type: inline-size;
  width: 100%;
  max-width: 100%;
  min-width: 0;
  overflow-wrap: break-word;
  word-break: break-word;
}

.ai-markdown :deep(pre) {
  max-width: 100%;
  overflow-x: auto;
  border-radius: 0.5rem;
  padding: 0.75rem;
  margin: 0.5rem 0;
  background: var(--p-surface-100, #f1f5f9);
}
.ai-markdown :deep(code) {
  font-size: 0.85em;
  overflow-wrap: break-word;
  white-space: break-spaces;
  word-break: break-word;
}
.ai-markdown :deep(pre code) {
  overflow-wrap: normal;
  white-space: pre;
  word-break: normal;
}
:global(.dark) .ai-markdown :deep(pre) {
  background: var(--p-surface-800, #1e293b);
}
.ai-markdown :deep(p) {
  margin: 0.35rem 0;
}
.ai-markdown :deep(ul),
.ai-markdown :deep(ol) {
  margin: 0.35rem 0;
  padding-left: 1.25rem;
  list-style: revert;
}
.ai-markdown :deep(a) {
  color: var(--p-primary-500, #6366f1);
  text-decoration: underline;
  overflow-wrap: break-word;
  word-break: break-word;
}
.ai-markdown :deep(.ai-markdown-table) {
  width: 100%;
  max-width: 100%;
  margin: 0.5rem 0;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
  scrollbar-color: var(--shell-scrollbar-thumb) transparent;
  scrollbar-width: thin;
}
.ai-markdown :deep(.ai-markdown-table:hover) {
  scrollbar-color: var(--shell-scrollbar-thumb-hover) transparent;
}
.ai-markdown :deep(.ai-markdown-table::-webkit-scrollbar) {
  width: 0.5rem;
  height: 0.5rem;
}
.ai-markdown :deep(.ai-markdown-table::-webkit-scrollbar-track) {
  background: transparent;
}
.ai-markdown :deep(.ai-markdown-table::-webkit-scrollbar-thumb) {
  border-radius: 9999px;
  background-color: var(--shell-scrollbar-thumb);
}
.ai-markdown :deep(.ai-markdown-table:hover::-webkit-scrollbar-thumb) {
  background-color: var(--shell-scrollbar-thumb-hover);
}
.ai-markdown :deep(table) {
  border-collapse: collapse;
  width: max(100%, 31.25rem);
  table-layout: fixed;
  font-size: 0.8125rem;
}
.ai-markdown :deep(th),
.ai-markdown :deep(td) {
  border: 1px solid var(--p-surface-300, #cbd5e1);
  padding: 0.375rem 0.625rem;
  overflow-wrap: break-word;
  vertical-align: top;
  word-break: break-word;
}
.ai-markdown :deep(th:first-child),
.ai-markdown :deep(td:first-child) {
  width: 20%;
  min-width: 7.5rem;
}
.ai-markdown :deep(th:nth-child(2)),
.ai-markdown :deep(td:nth-child(2)) {
  width: 35%;
  min-width: 11.25rem;
}
.ai-markdown :deep(th:nth-child(3)),
.ai-markdown :deep(td:nth-child(3)) {
  width: 45%;
  min-width: 12.5rem;
}
.ai-markdown :deep(th) {
  background: var(--p-surface-50, #f8fafc);
  font-weight: 600;
  text-align: left;
}
:global(.dark) .ai-markdown :deep(th) {
  background: var(--p-surface-800, #1e293b);
}
.ai-markdown :deep(td a) {
  display: inline-block;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  vertical-align: middle;
}

@container (max-width: 37.5rem) {
  .ai-markdown :deep(table) {
    width: max(100%, 28.125rem);
    font-size: 0.75rem;
  }

  .ai-markdown :deep(th),
  .ai-markdown :deep(td) {
    padding: 0.25rem 0.375rem;
  }

  .ai-markdown :deep(th:first-child),
  .ai-markdown :deep(td:first-child) {
    width: 25%;
    min-width: 6.25rem;
  }

  .ai-markdown :deep(th:nth-child(2)),
  .ai-markdown :deep(td:nth-child(2)) {
    width: 30%;
    min-width: 8.75rem;
  }

  .ai-markdown :deep(th:nth-child(3)),
  .ai-markdown :deep(td:nth-child(3)) {
    width: 45%;
    min-width: 11.25rem;
  }
}
</style>
