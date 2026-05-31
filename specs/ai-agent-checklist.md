# AI Agent — implementation checklist

Companion task list for [`ai-agent.md`](./ai-agent.md). Check items off as they
land. Each phase is done only when its tests pass and `make fmt && make lint &&
make test` are green. Section refs (§) point at the spec.

---

## P0 — `RouteInvoker` refactor (prereq, no AI yet) — §5.2

- [ ] Extract the secure pipeline from `internal/server/dispatch.go` `serveHTTP`
      (resolve → `authorize()` → `acquireSession()` → `ValidateSchema` →
      `route.Handle` → audit) into `Server.InvokeRoute(ctx, user, connID, routeID,
      params, body) (any, error)`.
- [ ] Rewrite HTTP dispatch to call `InvokeRoute` (no behavior change).
- [ ] Add an audit annotation/source field (`source = "http" | "ai"`,
      conversation/turn id) threaded into the audit `Event`.
- [ ] Unit tests: authz allow/deny, validation failure, handler error, audit
      recorded — parity with prior dispatch behavior.
- [ ] Full gate green; existing e2e unaffected.

## P1 — Config + secrets + user UI — §3, §5 (`config`)

### Backend
- [ ] `internal/config/ai.go`: `AIConfig` struct (`kind`, `name`, `baseURL`,
      `apiKey` from env, `models`, `defaultModel`) added to `Config`; defaults in
      `setDefaults`; `SHELLCN_AI_*` env binding. **Global = env/config, no DB, no UI.**
- [ ] `models.AIProviderConfig` (**user-scoped**: `ownerId`, `kind`, `name`,
      `baseURL`, `models []string`, `defaultModel`, `apiKeyCiphertext []byte`,
      timestamps); add to `allModels()` in `internal/store/db.go`.
- [ ] `internal/ai/config`: user-config CRUD; encrypt/decrypt keys via
      `secrets.Vault` (DB stores ciphertext only). Global keys never touch the DB.
- [ ] Built-in `kind`s: `openai`, `anthropic`, `google`, `openai_compatible`;
      custom providers = named `openai_compatible` (config or user row) (§3.1).
- [ ] Endpoints: `GET /api/ai/global` (read-only: present? + provider/model, **no
      key**); `GET/PUT/DELETE /api/me/ai/config` (user); `GET …/{id}/models`.
- [ ] Authz: users manage **only their own** config; there is **no global write path**.

### Frontend
- [ ] New nested settings page `AiSettingsView.vue` at route `settings/ai`
      (name `ai-settings`), linked from `SettingsView.vue` with a `RouterLink` row
      (same pattern as *My activity* / *Users & access*; not admin-gated).
- [ ] In it: list/add/edit/delete own providers; key field write-only/secret; model
      allow-list; default model; **Add custom provider** (name + base URL + key + models).
- [ ] Read-only **global** status row on `SettingsView.vue` mirroring the existing
      **Email** row ("Configured / Not configured" + model), from `GET /api/ai/global`.
- [ ] Reuse `SchemaForm` where it fits; PrimeVue components verified via context7.

### Tests
- [ ] User API key never returned/logged; stored encrypted; round-trips.
- [ ] Global config loads from env/YAML; key never exposed via API (only
      provider/model); no global write endpoint exists.
- [ ] User config is owner-scoped; model listing works per provider kind.

## P2 — Engine + read-only agent + chat MVP — §4, §5.1, §5.3, §5.5, §5.6

### Backend
- [ ] `internal/ai/engine`: define `Provider`, `Model`, `ChatRequest`,
      `StreamEvent`, `ToolSpec`, `ToolExecutor` interfaces (framework-agnostic).
- [ ] `internal/ai/engine/eino`: eino + eino-ext adapter for **one** provider
      end-to-end (verify APIs/adapters via context7). eino imported **only** here.
- [ ] `internal/ai/tools`: enumerate connection routes; filter to `RiskSafe`;
      build `ToolSpec` from route + `Input` schema (reuse schema→JSON-schema);
      `Execute` → `Server.InvokeRoute` (read-only) → cleaned result.
- [ ] `internal/ai/agent`: system prompt builder; turn loop via `engine.Stream`
      with `MaxSteps` + output cap; relay `StreamEvent`s (buffered ~40ms/160ch).
- [ ] `internal/ai/service.go`: resolve provider/model/key, build tool set, run a
      turn, stream out.
- [ ] Transport: `WS /api/connections/:id/ai/chat` (ticket-auth, same infra as
      other streams); authorize connection + `aiMode != disabled`.

### Frontend
- [ ] `AiChatLauncher.vue` — **tiny, main-bundle** header icon + Drawer shell in
      `ConnectionWorkspace.vue` (shown when AI configured + `aiMode != disabled` +
      connected); **lazy-loads** `AiChatPanel` via `defineAsyncComponent`
      (loadingComponent skeleton + errorComponent retry) — no AI deps in `main.js`.
- [ ] `web/src/panels/ai/AiChatPanel.vue` in a right **Drawer** (overlay, no
      remount); `AiMessageList.vue` + `AiMessage.vue`; `AiComposer.vue`. All chat
      deps (store, markstream-vue/markdown, highlight) imported **only** here so
      they ride the async chunk.
- [ ] Verify with the Vite build report: separate AI chunk; main chunk unchanged
      when AI absent.
- [ ] `AiToolBadges.vue` (grouped, collapsible, status).
- [ ] `stores/aiChat.ts` (Pinia): run state, streaming buffers.
- [ ] Consume chat WS via `useStream`; apply `text_delta`/`tool_call`/
      `tool_result`/`step`/`error`/`done`. Streaming markdown via **`markstream-vue`**
      (verify) or `markdown-it`+DOMPurify+highlight.js fallback. Custom widget on
      PrimeVue — **no** `@ai-sdk/vue` `useChat`, **no** external chat-UI framework.

### Tests
- [ ] Integration: against a real connection the agent lists resources via tools.
- [ ] Authz enforced on every tool call (as the user); only `RiskSafe` exposed.
- [ ] Streaming events render; stop aborts cleanly.

## P3 — Memory + conversations — §3.3, §5.4

### Backend
- [ ] `models.AIConversation` + `models.AIMessage` (role, content, toolCalls,
      toolResults, reasoning?, scope/model used) → `allModels()`.
- [ ] `internal/ai/memory`: persist turns; append stream deltas; finalize.
- [ ] Token budgeting + compaction: keep recent N turns full, roll older into a
      summary; truncate tool results (full recent / compact older). Token
      estimator chosen (`tiktoken-go` vs heuristic — verify).
- [ ] Per-model context window lookup (provider metadata or static registry +
      safe default).
- [ ] Auto-title conversations after first exchange.
- [ ] Conversation CRUD endpoints (create/list/get/rename/delete).

### Frontend
- [ ] `AiConversationList.vue` (history sidebar; new/rename/delete; streaming dot).
- [ ] Wire conversation switching, persistence, scrollback.

### Tests
- [ ] Compaction keeps recent turns; summary produced; budget math bounded.
- [ ] Conversation CRUD + auto-title.

## P4 — Subagents + recent-ops context — §5.5

### Backend
- [ ] Subagent tool(s) (e.g. `investigate` / `bulk_read`): nested **read-only**
      agent run with its own context budget, returns a **summary string**.
- [ ] Stream nested subagent tool-progress to the UI (prefixed).
- [ ] Recent-operations context: fetch user's recent audit entries for the
      connection (success + error); inject compact block or expose
      `recent_operations` read tool.

### Frontend
- [ ] `AiToolBadges.vue`: subagent calls shown with `▸`/`↳` prefix + distinct
      color; nested progress visible.

### Tests
- [ ] Subagent returns a summary; parent context stays bounded.
- [ ] "What just errored?" surfaces the recent failed operation.

## P5 — Per-connection write/destructive opt-in + HITL — §2, §3.2, §6

### Backend
- [ ] `models.Connection`: add `aiMode {disabled, read_only, read_write}` +
      `aiAllowDestructive bool` columns.
- [ ] Tool gating: `RiskWrite` only when `read_write`; `RiskDestructive` only when
      `aiAllowDestructive`; `RiskPrivileged` never; streaming routes never.
- [ ] Write/destructive tools pause (eino `interrupt/resume`); resume on confirm,
      decline on reject; destructive = mandatory confirm.
- [ ] Audit each mutation with real `Risk`/`AuditEvent` + `ai` source + turn id.

### Frontend
- [ ] `ConnectionFormDialog.vue`: `aiMode` control + **"Allow destructive
      operations"** checkbox (only in Read & write, off by default, warned).
- [ ] `AiActionConfirm.vue`: inline approve/reject card (route + resolved params);
      destructive rendered in danger style, no "always allow".

### Tests
- [ ] Write blocked in `read_only`; destructive blocked unless `aiAllowDestructive`.
- [ ] Privileged never exposed.
- [ ] RBAC still blocks a user lacking the underlying permission via the agent.
- [ ] Confirmation required before execution; audited as `ai`.

## P6 — Multi-provider + model switcher + polish — §3.1, §6

### Backend
- [ ] Remaining engine provider adapters (anthropic, google, openai_compatible).
- [ ] Global-vs-user scope selection stored per conversation.

### Frontend
- [ ] `AiModelSwitcher.vue`: provider+model switch across the user's providers
      (built-in **and** custom); read-only "Provider · Model" indicator for global.
- [ ] `AiReasoning.vue` (collapsible reasoning, last-N lines).
- [ ] Composer **message queue** (mid-stream input), send/stop, Enter/Shift+Enter.
- [ ] Retry-on-hover, scroll-to-bottom, empty state (quick-starts), error/retry,
      AI-not-configured + disabled-for-connection states.
- [ ] Conversation export (optional), copy.
- [ ] a11y (ARIA/keyboard/focus), dark/light, `prefers-reduced-motion` pass.

### Tests
- [ ] Each provider adapter streams + tool-calls.
- [ ] Model switch (user) vs locked indicator (global).
- [ ] Vitest for chat components (badges, confirm, switcher, queue, reasoning).

## Cross-cutting (apply across phases) — §9

- [ ] **Lazy-load** the whole chat widget (defineAsyncComponent + loading/error
      wrapper); zero AI deps in `main.js` — first paint constant when AI is absent
      (ShellCN invariant).
- [ ] Per-turn `MaxSteps` + output-token caps; optional per-user/day request cap.
- [ ] Surface token usage on messages; explicit "capped" notice (no silent trunc).
- [ ] Clean cancellation: stop aborts via context; partial message kept.
- [ ] Provider errors surface in chat with retry.
- [ ] Prompt-injection containment: tool output = data; writes/destructive always
      confirmed; tool results cleaned/truncated before context.
- [ ] i18n for all new UI strings.
- [ ] **Code rules (AGENTS.md):** verify every lib/API via context7 + websearch;
      PrimeVue-only UI via the preset; VueUse; **pnpm** (never npm); small focused
      units (no god-components); minimal comments (non-obvious *why* only), **no**
      spec/PR refs in source; eino confined to `engine/eino`; plugin-agnostic core.
- [ ] **Gate each step**: `make fmt && make lint && make test` green; from P2,
      **write *and execute*** env-gated integration tests (Docker self-provision).
- [ ] Update `specs/project.md` with the AI agent architecture once stable.

## Pre-build confirmations — §10

- [x] Global config = env/`internal/config/ai.go` only (no admin/user UI, key in
      env); model locked. User config = DB + UI, switchable.
- [x] Destructive behind `aiAllowDestructive` + mandatory confirm; privileged excluded v1.
- [x] Chat UI: custom widget on PrimeVue (no full chat-UI framework, no
      `@ai-sdk/vue` `useChat`); streaming markdown via `markstream-vue` (verify) /
      `markdown-it` fallback.
- [ ] Engine: eino (recommended) vs official-SDK fallback.
- [ ] Token estimator: `tiktoken-go` vs heuristic (verify maintenance).
- [ ] Chat placement: right Drawer (recommended).
