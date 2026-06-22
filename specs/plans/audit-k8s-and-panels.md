# Audit & Fix Checklist — K8s plugin + all generic panels

Status legend: `[ ]` todo · `[~]` in progress · `[x]` done. Each item: `[severity] area — file:line — problem → fix`.

Greenfield: contracts may change freely. Every fix is manifest-driven and plugin-agnostic. Tests gate each phase. Run `make fmt && make lint && make test` (Go + Vitest) before marking a phase done.

---

## Phase 1 — Save/apply contract + editor UX (user-reported) ✅ DONE

- [x] P1 contract — sdk/plugin/ui.go — added `SaveToast{Summary,Detail,Severity}` + `SaveDismiss` enum on CodeEditorConfig + FormPanelConfig; validate.go `checkSaveFeedback`; panel_schema.go props; panels.ts mirror.
- [x] P1 editor copy — CodeEditorPanel.vue — success toast from `saveToast` via useNotify; pill shows configured summary, not hardcoded "Saved".
- [x] P1 editor close — CodeEditorPanel emits `close` on saveDismiss==close; PanelHost forwards; ConnectionWorkspace dialog host → dock.closeDialog.
- [x] P1 dry-run errors — CodeEditorPanel toasts on preview/save failure (notify.error).
- [x] P1 k8s manifest — header Apply + Create dialogs set SaveToast/SaveDismiss; header Apply now has RefreshField+DryRunKey (Review enabled).
- [x] P2 form parity — FormPanel adopts saveToast + close on saveDismiss.
- Tests: streaming.test.ts close-emit; golden regenerated; sdk+kubernetes+form+type-check green.

## Phase 2 — K8s apply/dry-run backend (user-reported duplicate-port + preview 500) ✅ DONE

- [x] P0 apply verb — yaml.go `replaceOrCreate` — GET-then-Update(PUT, fresh RV, 409 retry) for existing objects, Create for new; replaces SSA → fixes duplicate port.
- [x] P1 error mapping — errors.go apiErr — added IsMethodNotSupported→ErrNotSupported and a generic 4xx StatusError → ErrInvalidInput fallthrough so dry-run errors surface their message (no hidden 500).
- [x] tests — TestYAMLApplyReplacesPorts (rename-port → http,https no duplicate, PUT), TestYAMLDryRunThreadsFlag (dryRun=All + content), TestYAMLEditRoundTrip rewritten for PUT+fresh-RV. lint clean.
- [x] P2 multi-doc — ApplyYAML now returns a top-level `content` joining the applied docs (`---`), so multi-doc RefreshField/preview work. Test: TestApplyYAMLMultiDocReturnsCombinedContent.

## Phase 3 — K8s routes security/correctness ✅ DONE (P1/P2 + secret)

- [x] P1 risk — routes.go — pod file write/upload/mkdir/delete now RiskPrivileged.
- [x] P1 injection — ops.go validateName/validateNamespace now RFC1123 (rejects `= , whitespace`); applied in DrainNode (already), ResourceEvents, WatchEvents.
- [x] P2 events ns — ResourceEvents + WatchEvents validate name and namespace before building field selectors.
- [x] P2 rbac gate — cronjob.trigger now EnabledWhen can.patch.
- [x] P3 secret — GetYAML + WatchObjectYAML strip `stringData` as well as `data`.
- [x] P3 scale guard — ScaleResource/RestartResource now reject kinds that aren't scalable/restartable (clear error instead of a raw apiserver rejection). Kept the existing replicas merge-patch (works for the wired kinds; the scale-subresource switch was marginal, skipped).

## Phase 4 — K8s watch / live-refresh backend — REVIEWED; deferred with reason

- [ ] P1 errors (frozen socket) — CONFIRMED REAL: on a persistent watch failure (e.g. RBAC revoked mid-session) `feed.run` retries forever and never closes the subscriber channel, so the WS handler blocks on `<-events`. But a correct fix needs careful feed-lifecycle work on the shared, ref-counted feed (avoid double-close races vs `remove()`/`Subscribe`) — not a simple change. Deferred per "don't over-complicate"; revisit as a dedicated, well-tested change.
- [ ] P2 resync — same: emit a re-list signal on 410/Error. Pairs with the P1 lifecycle work.
- [x] P3 coalesce — CHECKED, non-issue: `broadcast` holds the feed lock (single writer per channel); after draining one slot the resend always has room (buffer ≥1). No real drop-newest bug.
- [x] P3 query keepalive — CHECKED, would BREAK: `StreamQuery` handlers read browser request frames themselves; a `controlReader` (`discardWebSocketReads`) would steal those frames. Current `enabled:true` is correct.
- [ ] P3 caps / encode-logging — marginal; deferred.

## Phase 5 — K8s resource browsing (table/tree)

- [x] P2 visibility — TablePanel pauses the WS watch when the tab is hidden; on return it re-lists + resubscribes.
- [x] P2 tree badges — ResourceTree reloads category badges on refresh (loadBadges on mount + refreshKey).
- [x] P2 ns param — verified non-issue (core merges `p.`-prefixed query into rc.params; `rc.Param` is correct).
- [ ] P2 watch-vs-page + prepend — TablePanel — on page>0/sort the live merge refetches per event, and live adds prepend (row-jump). REAL but intricate (touches the watch/merge core); deferred to a dedicated change.
- [ ] P2 a11y rows — TablePanel — no keyboard activation to open a row's detail. REAL WCAG gap; deferred (needs care in the large DataTable wiring).

Removed as not worth doing (verified marginal / intentional / feature-scope):
- forbidden/empty state — the apiserver error already reads "forbidden: User cannot list…"; a custom message is cosmetic.
- rbac row gating — intentional fail-open (server enforces RBAC; per-row SSAR is expensive); clear post-hoc denial is fine.
- label/field-selector lists — a feature, not a bug; client-side filter covers the common case.
- P3 micro-items (modified re-sort, CRD list cap, serial reloadExpanded, CRD/Helm tree watch, SSAR-per-open) — marginal.

## Phase 6 — K8s pod operations (logs + debug DONE; rest pending)

- [x] P0 logs container — NEW generic `StreamControl`/`LogStreamConfig` contract (ui.go/validate.go/panel_schema.go/panels.ts). LogsStream now streams ALL containers by default (per-container `[name]` prefixes), single when filtered; PodContainers route offers "All containers" + each (only when >1). LogStreamPanel renders a manifest-driven container Select + reconnects (useStream closes old channel on param change). Plugin-agnostic. Tests: TestLogsStreamPrefixesAllContainers, TestPodContainersOffersAllAndEach.
- [x] P0 debug select — debug route gets Input schema (image text default busybox + target container); Confirm dropped (form is the gate).
- [x] P1 logs previous — LogStreamConfig.AllowPrevious → "Previous (crashed)" toggle re-streams with previous=true.
- [x] P3 logs a11y — viewport now role="log" aria-live; filter/follow/previous aria-labels/aria-pressed.
- [x] P1 metrics absent — onFrame keeps numeric context (requests/limits) when metricsAvailable=false; non-blocking PrimeVue Message + stat cards instead of a blank error; backend sends a source-specific message.
- [x] P2 logs UX — wrap/no-wrap toggle added.
- [x] files container picker — `FileBrowserConfig.Controls` (generic `StreamControl`) threads a container selector through every file operation via `operationParams`; `kubernetes.pod.containers` (app containers only, since init containers are terminated and can't be exec'd) feeds it. Picker hidden for single-container pods; switching containers resets to the start path.
- [x] pod files preview/download — podfs ignored the file's real size and MIME: `FileContent.Size` reported the capped read length (looked like the file "didn't fully arrive") and downloads were always `application/octet-stream`, so `nosniff` blocked inline image/PDF/audio/video previews (only text rendered). Now mirrors the shared filesystem contract: real size via a `stat`/`wc` probe in the same exec, extension MIME (`filesystem.MimeFor`/`IsText` exported for reuse), correct truncation (read cap+1), and a trailing partial-rune trim so a large UTF-8 file split at the cap stays text. Tests: TestPodFileContent.
- [ ] exec (terminal) container picker — DEFERRED (feature-scope): the xterm/Terminal panel has no generic `StreamControl` host yet; the file/log pattern doesn't transfer directly. Valuable; needs a dedicated change.

Removed: logs JSON-frame branch (harmless for k8s plain text); jump-to-latest / pause-on-scroll (Follow covers it); exit-code banner / keepalive (transport layer handles idle).

## Phase 7 — Generic panels UX / a11y / consistency (ALL panels)

**Feedback principle (from review):** one channel per outcome. Stream/connection status lives in `StreamStatusBar`, NOT per-panel toasts. No toast + inline error for the same failure. Don't add success toasts to routine/idempotent actions.

Done (landed + kept after review prune):
- [x] P0 form a11y — FormField aria-invalid/aria-describedby link errors to every control (computed errorId/describedBy/ariaInvalid).
- [x] P0 grid keyboard — FileEntryGrid full roving-tabindex 2D keyboard nav + Enter/Space; aria-current→aria-selected (grid + list).
- [x] P1 form groups — radio group + object groups now fieldset/legend; slider value aria-live; FileUpload aria-label + invalid; SchemaForm submit aria-busy.
- [x] P1 query — error span role=alert/aria-live; result `<th>` scope="col" (heavy red-box styling reverted as noise).
- [x] P1 file select/dnd — aria-selected; drop-zone role=status + live announce; chmod Select aria-label.
- [x] P1 termgrid — destructive "reset all panes" confirm (confirmDanger); pane counter role=status/aria-live. (Per-split/close toasts deliberately NOT added — noise.)
- [x] P2 partial — StreamStatusBar status role/aria-live + motion-safe pulse + popover role; GaugeChart/SeriesChart disable animation on prefers-reduced-motion; RemoteDesktop REC pulse motion-safe; error text role=alert across stream panels.

Reverted as over-complication (intentionally NOT done):
- [x] ~~P1 streams reconnect toasts~~ — StreamStatusBar already shows reconnect/error; toasts were redundant. Kept only role=alert on error text.
- [x] ~~StatCard loading skeleton~~ — regressed genuinely-null metrics to a perpetual skeleton; reverted to "—".
- [x] CodeEditorPanel double-feedback — removed save/preview error toasts (inline saveError is the single channel); dry-run rejection no longer opens a misleading local diff.

Done (this batch):
- [x] cred error — CredentialSelect load error now has a compact Retry + role=alert.
- [x] http aria — HTTPClient header inputs have indexed aria-labels (the response is the feedback; no toast).
- [x] task aria — TaskProgress ProgressBar aria-label includes the percent (or "in progress").
- [x] kv labels — the two unlabeled `Select`s got aria-label="Type" (inputs/editors already had labels).
- [x] ConnectPanel pulse — gated with `motion-safe:`.

Removed as not worth doing (verified non-issue / marginal / stylistic):
- EnrollPanel `variant="text"` — NOT a bug: `variant` (text/outlined/link) is a valid PrimeVue Button prop (in the preset).
- ActionBar menu items, TablePanel badge→`Tag`, DockPanel tab buttons — work fine; PrimeVue-first restyle with no user-visible change.
- TablePanel delete-retry / inline-cell validation — backend validates; the dialog already shows the error.
- CodeDiffView notice — the `<pre>` fallback already shows the content.
- log Clear confirm — clearing the *view* (not data) isn't destructive; a confirm is friction.
- ObjectDetail/Document copy toast — both already show an inline "copied" badge.
- JsonNode treeitem roles, ScopeBar double-label, ArrayField/MapField copy, P3 hardcoded-strings→config, export-Menu icon, terminal-search debounce — marginal.

---

## Cross-cutting principles (apply, don't over-apply)

- [ ] Standardize error states on `PanelError` (role=alert + retry) and loading on `SkeletonList` (motion-safe) — replace ad-hoc red `<p>` and bare spinners. (Skeletons only for things that actually load — never for genuinely-absent values.)
- One feedback channel per outcome: a contextual inline error OR a toast, never both. Stream/connection state stays in `StreamStatusBar`. Toasts only for actions whose result isn't otherwise visible (e.g. a dialog that closes on success). Do NOT toast routine/idempotent actions.
- [ ] `prefers-reduced-motion` honored via `motion-safe:` (CSS) or a media-query check (JS charts) wherever motion is used.
