<script setup lang="ts">
import { onMounted, ref } from "vue";

const out = ref<HTMLElement | null>(null);
const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

async function typeInto(node: HTMLElement, text: string, cps: number) {
  for (const ch of text) {
    node.textContent += ch;
    await sleep(1000 / cps + Math.random() * 16);
  }
}

onMounted(async () => {
  const host = out.value;
  if (!host) return;
  if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
    host.innerHTML = STATIC;
    return;
  }
  const span = (cls: string) => {
    const s = document.createElement("span");
    if (cls) s.className = cls;
    host.appendChild(s);
    return s;
  };
  const raw = (html: string) => host.insertAdjacentHTML("beforeend", html);

  await typeInto(span("text-ok"), "$ ", 55);
  await typeInto(span("text-accent"), "docker run -d -p 8081:8081 ", 40);
  await typeInto(span("text-[#a78bfa]"), "ghcr.io/charlesng35/shellcn", 40);
  raw("\n\n");
  await sleep(420);
  raw('<span class="text-muted">▸ shellcn — starting…</span>\n');
  await sleep(480);
  raw('<span class="text-muted">▸ store: sqlite (embedded) · master key loaded</span>\n');
  await sleep(420);
  raw('<span class="text-ok">✓ 20 protocols registered</span>\n');
  await sleep(360);
  raw('<span class="text-ok">✓ reverse-agent tunnel hub ready</span>\n');
  await sleep(360);
  raw('<span class="text-ok">✓ listening on <span class="text-accent underline">http://localhost:8081</span></span>\n\n');
  await sleep(520);
  await typeInto(span("text-muted"), "# open a browser and sign in — you're in.", 42);
});

const STATIC = `<span class="text-ok">$ </span><span class="text-accent">docker run -d -p 8081:8081 </span><span class="text-[#a78bfa]">ghcr.io/charlesng35/shellcn</span>

<span class="text-muted">▸ shellcn — starting…</span>
<span class="text-ok">✓ 20 protocols registered</span>
<span class="text-ok">✓ listening on <span class="text-accent underline">http://localhost:8081</span></span>`;
</script>

<template>
  <div
    class="overflow-hidden rounded-2xl border border-line bg-[linear-gradient(180deg,#0c1626,#0a1220)] shadow-[0_30px_80px_rgba(0,0,0,0.55)]"
    aria-hidden="true"
  >
    <div class="flex items-center gap-2 border-b border-line bg-white/[0.02] px-3.5 py-3">
      <span class="h-3 w-3 rounded-full bg-[#ff5f57]"></span>
      <span class="h-3 w-3 rounded-full bg-[#febc2e]"></span>
      <span class="h-3 w-3 rounded-full bg-[#28c840]"></span>
      <span class="ml-2.5 font-mono text-xs text-dim">bash — shellcn</span>
    </div>
    <pre class="m-0 min-h-[300px] whitespace-pre-wrap break-words px-[18px] py-5 font-mono text-[13.5px] leading-[1.7] text-[#cfe0f5]"><code ref="out"></code><span class="blink text-accent">▊</span></pre>
  </div>
</template>
