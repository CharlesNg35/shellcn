const $ = (s, r = document) => r.querySelector(s);
const $$ = (s, r = document) => [...r.querySelectorAll(s)];
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

/* ---------- protocols ---------- */
const CAT = {
  Shell: "#38bdf8",
  Files: "#22d3ee",
  Containers: "#60a5fa",
  Virtualization: "#a78bfa",
  Desktop: "#f472b6",
  Databases: "#34d399",
  Observability: "#fbbf24",
  Directory: "#94a3b8",
};
const PROTOCOLS = [
  ["SSH", "Shell"], ["SFTP", "Files"], ["FTP", "Files"], ["FTPS", "Files"],
  ["SMB", "Files"], ["WebDAV", "Files"], ["S3", "Files"], ["Docker", "Containers"],
  ["Swarm", "Containers"], ["Podman", "Containers"], ["Kubernetes", "Containers"],
  ["Proxmox", "Virtualization"], ["VNC", "Desktop"], ["RDP", "Desktop"],
  ["PostgreSQL", "Databases"], ["MySQL", "Databases"], ["MongoDB", "Databases"],
  ["Redis", "Databases"], ["Monitoring", "Observability"], ["LDAP", "Directory"],
];

function renderProtocols() {
  const grid = $("#proto-grid");
  if (!grid) return;
  PROTOCOLS.forEach(([n, c]) => {
    const el = document.createElement("div");
    el.className = "proto";
    el.innerHTML = `<span class="pd" style="background:${CAT[c]}"></span><span><b>${n}</b><small>${c}</small></span>`;
    grid.appendChild(el);
  });
  const more = document.createElement("div");
  more.className = "proto more";
  more.textContent = "+ external plugins";
  grid.appendChild(more);
}

function renderMarquee() {
  const items = PROTOCOLS.map(([n]) => n).concat(["External plugins", "Marketplace"]);
  const html = items.map((n) => `<span class="chip">${n}</span>`).join("");
  const a = $("#marquee-a"), b = $("#marquee-b");
  if (a) a.innerHTML = html;
  if (b) b.innerHTML = html;
}

/* ---------- terminal typing ---------- */
async function typeInto(node, text, cps) {
  for (const ch of text) {
    node.textContent += ch;
    await sleep(1000 / cps + Math.random() * 18);
  }
}
async function runTerminal() {
  const out = $("#term-out");
  if (!out) return;
  const span = (cls) => {
    const s = document.createElement("span");
    if (cls) s.className = cls;
    out.appendChild(s);
    return s;
  };
  const raw = (html) => out.insertAdjacentHTML("beforeend", html);

  await typeInto(span("p"), "$ ", 55);
  await typeInto(span("c"), "docker run -d -p 8081:8081 ", 40);
  await typeInto(span("k"), "ghcr.io/charlesng35/shellcn", 40);
  raw("\n\n");
  await sleep(450);
  raw('<span class="info">▸ shellcn — starting…</span>\n');
  await sleep(500);
  raw('<span class="info">▸ store: sqlite (embedded) · master key loaded</span>\n');
  await sleep(450);
  raw('<span class="ok">✓ 20 protocols registered</span>\n');
  await sleep(380);
  raw('<span class="ok">✓ reverse-agent tunnel hub ready</span>\n');
  await sleep(380);
  raw('<span class="ok">✓ listening on <span class="url">http://localhost:8081</span></span>\n\n');
  await sleep(550);
  await typeInto(span("info"), "# open a browser and sign in — you're in.", 42);
}

/* ---------- tabs + copy ---------- */
function initStart() {
  $$(".tab").forEach((tab) => {
    tab.addEventListener("click", () => {
      const id = tab.dataset.tab;
      $$(".tab").forEach((t) => t.classList.toggle("active", t === tab));
      $$(".panel").forEach((p) => p.classList.toggle("active", p.dataset.panel === id));
    });
  });
  const copyBtn = $("[data-copy]");
  if (copyBtn) {
    copyBtn.addEventListener("click", async () => {
      const code = $(".panel.active code")?.textContent || "";
      try {
        await navigator.clipboard.writeText(code);
      } catch {
        const ta = document.createElement("textarea");
        ta.value = code;
        document.body.appendChild(ta);
        ta.select();
        document.execCommand("copy");
        ta.remove();
      }
      copyBtn.classList.add("done");
      const label = copyBtn.querySelector("span");
      const prev = label.textContent;
      label.textContent = "Copied";
      copyBtn.querySelector("use").setAttribute("href", "#i-check");
      setTimeout(() => {
        copyBtn.classList.remove("done");
        label.textContent = prev;
        copyBtn.querySelector("use").setAttribute("href", "#i-copy");
      }, 1600);
    });
  }
}

/* ---------- reveal on scroll ---------- */
function initReveal() {
  const io = new IntersectionObserver(
    (entries) => {
      entries.forEach((e) => {
        if (e.isIntersecting) {
          e.target.classList.add("in");
          io.unobserve(e.target);
        }
      });
    },
    { threshold: 0.12, rootMargin: "0px 0px -8% 0px" },
  );
  $$(".reveal").forEach((el) => io.observe(el));
}

renderProtocols();
renderMarquee();
initStart();
initReveal();
runTerminal();
