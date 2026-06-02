<script setup lang="ts">
import { computed } from "vue";
import MarkdownIt from "markdown-it";
import DOMPurify from "dompurify";
import hljs from "highlight.js/lib/common";

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
.ai-markdown :deep(pre) {
  overflow-x: auto;
  border-radius: 0.5rem;
  padding: 0.75rem;
  margin: 0.5rem 0;
  background: var(--p-surface-100, #f1f5f9);
}
.ai-markdown :deep(code) {
  font-size: 0.85em;
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
}
.ai-markdown :deep(table) {
  display: block;
  overflow-x: auto;
  border-collapse: collapse;
}
.ai-markdown :deep(th),
.ai-markdown :deep(td) {
  border: 1px solid var(--p-surface-300, #cbd5e1);
  padding: 0.25rem 0.5rem;
}
</style>
