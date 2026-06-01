# AI Agent — implementation checklist

Companion task list for [`ai-agent.md`](./ai-agent.md). Check items off as they
land. Each phase is done only when its tests pass and `make fmt && make lint &&
make test` are green. Section refs (§) point at the spec.

---

## P0 — `RouteInvoker` refactor (prereq, no AI yet) — §5.2

- [x] Extract the secure pipeline from `internal/server/dispatch.go` `serveHTTP`
      (resolve → `authorize()` → `acquireSession()` → `ValidateSchema` →
      `route.Handle` → audit) into shared invocation helpers plus
      `Server.InvokeRoute(ctx, user, connID, routeID, params, body) (any, error)`.
- [x] Rewrite HTTP dispatch to share the same invocation core as `InvokeRoute`
      (no behavior change).
- [x] Add an audit annotation/source field (`source = "http" | "ai"`,
      conversation/turn id) threaded into the audit `Event`.
- [x] Unit tests: authz allow/deny, validation failure, handler error, audit
      recorded — parity with prior dispatch behavior.
- [x] Full gate green; existing e2e unaffected.

## P1 — Config + secrets + user UI — §3, §5 (`config`)

### Backend

- [x] `internal/config/ai.go`: `AIConfig` struct (`kind`, `name`, `baseURL`,
      `apiKey` from env, pinned `model`) added to `Config`; defaults in
      `setDefaults`; `SHELLCN_AI_*` env binding. **Global = env/config, no DB, no UI.**
- [x] `models.AIProviderConfig` (**user-scoped**: `ownerId`, `kind`, `name`,
      `baseURL`, `models []string`, `defaultModel`, `apiKeyCiphertext []byte`,
      timestamps); add to `allModels()` in `internal/store/db.go`.
- [x] `internal/ai/config`: user-config CRUD; encrypt/decrypt keys via
      `secrets.Vault` (DB stores ciphertext only). Global keys never touch the DB.
- [x] Built-in `kind`s: `openai`, `openrouter`, `anthropic`, `google`, `openai_compatible`;
      custom providers = named `openai_compatible` (config or user row) (§3.1).
- [x] Endpoints: `GET /api/ai/global` (read-only: present? + provider/model, **no
      key**); `GET/PUT/DELETE /api/me/ai/config` (user); `GET …/{id}/models`.
- [x] Authz: users manage **only their own** config; there is **no global write path**.

### Frontend

- [x] New nested settings page `AiSettingsView.vue` at route `settings/ai`
      (name `ai-settings`), linked from `SettingsView.vue` with a `RouterLink` row
      (same pattern as _My activity_ / _Users & access_; not admin-gated).
- [x] In it: list/add/edit/delete own providers; key field write-only/secret; model
      allow-list; default model; **Add custom provider** (name + base URL + key + models).
- [x] Read-only **global** status row on `SettingsView.vue` mirroring the existing
      **Email** row ("Configured / Not configured" + model), from `GET /api/ai/global`.
- [x] PrimeVue components used for settings controls; `SchemaForm` was not a good
      fit for the provider CRUD form.

### Tests

- [x] User API key never returned/logged; stored encrypted; round-trips.
- [x] Global config loads from env/YAML; key never exposed via API (only
      provider/model); no global write endpoint exists.
- [x] User config is owner-scoped; model listing works per provider kind.

## P2 — Engine + read-only agent + chat MVP — §4, §5.1, §5.3, §5.5, §5.6

### Backend

- [x] `internal/ai/engine`: define `Provider`, `Model`, `ChatRequest`,
      `StreamEvent`, `ToolSpec`, `ToolExecutor` interfaces (framework-agnostic).
- [x] `internal/ai/engine/eino`: eino + eino-ext adapter for OpenAI-compatible,
      Anthropic, and Google. eino is imported **only** here.
- [x] `internal/ai/tools`: enumerate connection routes; filter to `RiskSafe`;
      build `ToolSpec` from route + `Input` schema (reuse schema→JSON-schema);
      `Execute` → `Server.InvokeRoute` (read-only) → cleaned result.
- [x] `internal/ai/agent`: system prompt builder; turn loop via `engine.Stream`
      with `MaxSteps` + output cap; relay `StreamEvent`s (buffered ~40ms/160ch).
- [x] `internal/ai/service.go`: resolve provider/model/key, build tool set, run a
      turn, stream out.
- [x] Transport: `WS /api/connections/:id/ai/chat` (ticket-auth, same infra as
      other streams); authorize connection + `aiMode != disabled`.

### Frontend

- [x] `AiChatLauncher.vue` — **tiny, main-bundle** header icon + Drawer shell in
      `ConnectionWorkspace.vue` (shown when AI configured + `aiMode != disabled` +
      connected); **lazy-loads** `AiChatPanel` via `defineAsyncComponent`
      (loadingComponent skeleton + errorComponent retry) — no AI deps in `main.js`.
- [x] `web/src/panels/ai/AiChatPanel.vue` in a right **Drawer** (overlay, no
      remount); `AiMessageList.vue` + `AiMessage.vue`; `AiComposer.vue`. All chat
      deps (store, markdown, highlight) imported **only** here so
      they ride the async chunk.
- [ ] Verify with the Vite build report: separate AI chunk; main chunk unchanged
      when AI absent.
- [x] `AiToolBadges.vue` (grouped, collapsible, status).
- [x] `stores/aiChat.ts` (Pinia): run state, streaming buffers.
- [x] Consume chat WS; apply `text_delta`/`tool_call`/
      `tool_result`/`step`/`error`/`done`. Streaming markdown via **`markstream-vue`**
      (verify) or `markdown-it`+DOMPurify+highlight.js fallback. Custom widget on
      PrimeVue — **no** `@ai-sdk/vue` `useChat`, **no** external chat-UI framework.

### Tests

- [ ] Integration: against a real connection the agent lists resources via tools.
- [x] Authz enforced on every tool call (as the user); only `RiskSafe` exposed.
- [x] Streaming events render; stop aborts cleanly.

## P3 — Memory + conversations — §3.3, §5.4

### Backend

- [x] `models.AIConversation` + `models.AIMessage` (role, content, toolCalls,
      toolResults, reasoning?, scope/model used) → `allModels()`.
- [x] `internal/ai/memory`: persist turns; append stream deltas; finalize.
- [x] Token budgeting + compaction: keep recent N turns full, roll older into a
      summary; truncate tool results (full recent / compact older). Token
      estimator chosen: heuristic.
- [x] Per-model context window lookup (provider metadata or static registry +
      safe default).
- [x] Auto-title conversations after first exchange.
- [x] Conversation CRUD endpoints (create/list/get/rename/delete).

### Frontend

- [x] `AiConversationList.vue` (history sidebar; new/rename/delete; streaming dot).
- [x] Wire conversation switching, persistence, scrollback.

### Tests

- [x] Compaction keeps recent turns; summary produced; budget math bounded.
- [x] Conversation CRUD + auto-title.

## P4 — Subagents + recent-ops context — §5.5

### Backend

- [x] Subagent tool(s) (e.g. `investigate` / `bulk_read`): nested **read-only**
      agent run with its own context budget, returns a **summary string**.
- [x] Stream nested subagent tool-progress to the UI (prefixed).
- [x] Recent-operations context: fetch user's recent audit entries for the
      connection (success + error); inject compact block or expose
      `recent_operations` read tool.

### Frontend

- [x] `AiToolBadges.vue`: subagent calls shown with `▸`/`↳` prefix + distinct
      color; nested progress visible.

### Tests

- [x] Subagent returns a summary; parent context stays bounded.
- [x] "What just errored?" prompt context includes the recent failed operation.

## P5 — Per-connection write/destructive opt-in + HITL — §2, §3.2, §6

### Backend

- [x] `models.Connection`: add `aiMode {disabled, read_only, read_write}` +
      `aiAllowDestructive bool` columns.
- [x] Tool gating: `RiskWrite` only when `read_write`; `RiskDestructive` only when
      `aiAllowDestructive`; `RiskPrivileged` never; streaming routes never.
- [x] Write/destructive tools pause via the chat confirmer; resume on confirm,
      decline on reject; destructive = mandatory confirm.
- [x] Audit each mutation with real `Risk`/`AuditEvent` + `ai` source + turn id.

### Frontend

- [x] `ConnectionFormDialog.vue`: `aiMode` control + **"Allow destructive
      operations"** checkbox (only in Read & write, off by default, warned).
- [x] `AiActionConfirm.vue`: inline approve/reject card (route + resolved params);
      destructive rendered in danger style, no "always allow".

### Tests

- [x] Write blocked in `read_only`; destructive blocked unless `aiAllowDestructive`.
- [x] Privileged never exposed.
- [x] RBAC still blocks a user lacking the underlying permission via the agent.
- [x] Confirmation required before execution; audited as `ai`.

## P6 — Multi-provider + model switcher + polish — §3.1, §6

### Backend

- [x] Remaining engine provider adapters (openrouter, anthropic, google, openai_compatible).
- [x] Global-vs-user scope selection stored per conversation.

### Frontend

- [x] `AiModelSwitcher.vue`: provider+model switch across the user's providers
      (built-in **and** custom); read-only "Provider · Model" indicator for global.
- [x] `AiReasoning.vue` (collapsible reasoning, last-N lines).
- [x] Composer **message queue** (mid-stream input), send/stop, Enter/Shift+Enter.
- [x] Retry-on-hover, scroll-to-bottom, empty state (quick-starts), error/retry,
      AI-not-configured + disabled-for-connection states.
- [ ] Conversation export (optional), copy.
- [ ] a11y (ARIA/keyboard/focus), dark/light, `prefers-reduced-motion` pass.

### Tests

- [ ] Each provider adapter streams + tool-calls.
- [x] Model switch (user) vs locked indicator (global).
- [x] Vitest for chat components (badges, confirm, switcher, queue, reasoning).

## Cross-cutting (apply across phases) — §9

- [x] **Lazy-load** the whole chat widget (defineAsyncComponent + loading/error
      wrapper); zero AI deps in `main.js` — first paint constant when AI is absent
      (ShellCN invariant).
- [x] Per-turn `MaxSteps` + output-token caps; optional per-user/day request cap.
- [x] Clean cancellation: stop aborts via context; partial message kept.
- [x] Provider errors surface in chat with retry.
- [x] Prompt-injection containment: tool output = data; writes/destructive always
      confirmed; tool results cleaned/truncated before context.
- [ ] i18n for all new UI strings.
- [ ] **Code rules (AGENTS.md):** verify every lib/API via context7 + websearch;
      PrimeVue-only UI via the preset; VueUse; **pnpm** (never npm); small focused
      units (no god-components); minimal comments (non-obvious _why_ only), **no**
      spec/PR refs in source; eino confined to `engine/eino`; plugin-agnostic core.
- [x] **Gate green**: `make fmt && make lint && make test`.
- [ ] From P2,
      **write _and execute_** env-gated integration tests (Docker self-provision).
- [ ] Update `specs/project.md` with the AI agent architecture once stable.

## Pre-build confirmations — §10

- [x] Global config = env/`internal/config/ai.go` only (no admin/user UI, key in
      env); model locked. User config = DB + UI, switchable.
- [x] Destructive behind `aiAllowDestructive` + mandatory confirm; privileged excluded v1.
- [x] Chat UI: custom widget on PrimeVue (no full chat-UI framework, no
      `@ai-sdk/vue` `useChat`); streaming markdown via `markstream-vue` (verify) /
      `markdown-it` fallback.
- [x] Engine: eino (recommended) vs official-SDK fallback.
- [x] Token estimator: `tiktoken-go` vs heuristic (verify maintenance).
- [x] Chat placement: right Drawer (recommended).
