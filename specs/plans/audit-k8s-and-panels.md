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

- [ ] P2 watch-vs-page — rows.go:19 + TablePanel.vue:1088 — page>0/sort/filter triggers full refetch per event → suppress watch-refresh off page 0 or paginate-watch.
- [ ] P2 prepend — TablePanel.vue:1137 — live adds always prepend, reorder sorted lists; events grow unbounded → insert per sort / honor max cap.
- [x] P2 visibility — TablePanel pauses the WS watch when the tab is hidden; on return it re-lists (catch up) and resubscribes.
- [ ] P2 selectors — resources.go:65, watch.go — no label/field selector; client-side substring only → plumb selectors into list+watch.
- [x] P2 ns param — CHECKED, non-issue: the core merges `p.`-prefixed query params into rc.params, so `rc.Param("namespace")` is correct for namespaced watches; `param()`'s fallback is only for plain query (container/follow). No change needed.
- [ ] P2 forbidden state — TablePanel.vue:1620, errors.go:27 — forbidden/empty/no-ns collapse to one cryptic state → distinct friendly states.
- [ ] P2 rbac rows — resources.go:138, permissions.go:49 — delete shown regardless of perms → can map on list rows / clear message.
- [x] P2 tree badges — ResourceTree now reloads category badges on refresh (extracted loadBadges, called onMounted + on refreshKey), so counts don't go stale.
- [ ] P2 a11y rows — TablePanel.vue:966 — no keyboard row activation → Enter opens detail/navigate.
- [ ] P3 misc — modified re-sort (TablePanel.vue:1128); CRD list unbounded (resources.go:76); reloadExpanded serial (ResourceTree.vue:131); CRD/Helm tree no watch; aria-live count; SSAR per-open (object_overview.go:24).

## Phase 6 — K8s pod operations (logs + debug DONE; rest pending)

- [x] P0 logs container — NEW generic `StreamControl`/`LogStreamConfig` contract (ui.go/validate.go/panel_schema.go/panels.ts). LogsStream now streams ALL containers by default (per-container `[name]` prefixes), single when filtered; PodContainers route offers "All containers" + each (only when >1). LogStreamPanel renders a manifest-driven container Select + reconnects (useStream closes old channel on param change). Plugin-agnostic. Tests: TestLogsStreamPrefixesAllContainers, TestPodContainersOffersAllAndEach.
- [x] P0 debug select — debug route gets Input schema (image text default busybox + target container); Confirm dropped (form is the gate).
- [x] P1 logs previous — LogStreamConfig.AllowPrevious → "Previous (crashed)" toggle re-streams with previous=true.
- [x] P3 logs a11y — viewport now role="log" aria-live; filter/follow/previous aria-labels/aria-pressed.
- [ ] P1 logs frames — LogStreamPanel JSON {ts,line} branch is harmless for k8s plain text; revisit if a plugin sends structured frames.
- [ ] P1 files container — podfs.go — container picker (reuse StreamControl pattern); symlink target + dir-ness; MIME inference.
- [x] P1 metrics absent — onFrame now keeps numeric context (requests/limits) when metricsAvailable=false; panel shows a non-blocking PrimeVue Message + the request/limit stat cards instead of a blank error; backend sends an actionable `message` per source (metrics-server/Prometheus/none).
- [ ] P2 exec — container/shell picker; exit code surfacing; keepalive.
- [ ] P2 logs UX — wrap toggle; jump-to-latest; pause-on-scroll-up; bound workload fan-out.

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

Still TODO (agents for shared/specialized panels not run):
- [ ] P1 menu — ActionBar.vue:457-512 — hand-rolled `<a>` items in PrimeVue Menu → item template/Button with roles.
- [x] P1 cred error — CredentialSelect load error now has a compact Retry (calls load) + role=alert.
- [x] P1 http aria — HTTPClient header key/value inputs now have indexed aria-labels (the send result is already the feedback, so no toast).
- [x] P1 task aria — TaskProgress ProgressBar aria-label now includes the percent (or "in progress" when indeterminate).
- [ ] CodeDiffView fallback — SKIPPED: the `<pre>` fallback already shows the content (graceful degradation), so a notice is marginal.
- [ ] files/exec container pickers — DEFERRED: valuable, but threading a container selector through every file route / the xterm panel is feature-scope, not a simple fix. Needs a dedicated change like the logs `StreamControl`.
- [ ] P1 table — TablePanel.vue — badge `<span>`→Tag; delete error no retry; inline editors no validation/aria-invalid.
- [ ] P1 kv — KVPanel.vue:430-476,391,401 — labels unassociated; hardcoded copy; no detail skeleton.
- [ ] P1 diff — CodeDiffView.vue:53-57 — silent fallback to `<pre>`, no message → PanelError/message.
- [ ] P1 logclear — LogStreamPanel — destructive Clear has no confirm → add confirm + aria-labels (NO success toast).
- [ ] P1 http — HTTPClientPanel.vue:209,249 — send feedback (single channel); inputs no aria-label.
- [ ] P1 task — TaskProgressPanel.vue:145 — ProgressBar aria-label lacks percent.
- [ ] copy (ObjectDetail/Document) — already have inline "copied" badges; only handle clipboard-unavailable. Low priority.
- [ ] P2 remainder — DockPanel native `<button>` tabs + resize keyboard; EnrollPanel invalid `variant="text"`; JsonNode treeitem roles; ScopeBar double-label; ConnectPanel pulse motion-safe; ArrayField/MapField hardcoded copy.
- [ ] P3 — hardcoded user-facing strings → config (debatable; many empty-states are fine inline); export Menu icon class; terminal search debounce.

---

## Cross-cutting principles (apply, don't over-apply)

- [ ] Standardize error states on `PanelError` (role=alert + retry) and loading on `SkeletonList` (motion-safe) — replace ad-hoc red `<p>` and bare spinners. (Skeletons only for things that actually load — never for genuinely-absent values.)
- One feedback channel per outcome: a contextual inline error OR a toast, never both. Stream/connection state stays in `StreamStatusBar`. Toasts only for actions whose result isn't otherwise visible (e.g. a dialog that closes on success). Do NOT toast routine/idempotent actions.
- [ ] `prefers-reduced-motion` honored via `motion-safe:` (CSS) or a media-query check (JS charts) wherever motion is used.
