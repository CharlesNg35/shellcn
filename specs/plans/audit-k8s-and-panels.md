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
- [ ] P2 multi-doc — yaml.go:150-153 — multi-doc apply returns no top-level `content`, breaks RefreshField → deferred (single-doc tab/dialog unaffected; revisit for multi-doc preview).

## Phase 3 — K8s routes security/correctness ✅ DONE (P1/P2 + secret)

- [x] P1 risk — routes.go — pod file write/upload/mkdir/delete now RiskPrivileged.
- [x] P1 injection — ops.go validateName/validateNamespace now RFC1123 (rejects `= , whitespace`); applied in DrainNode (already), ResourceEvents, WatchEvents.
- [x] P2 events ns — ResourceEvents + WatchEvents validate name and namespace before building field selectors.
- [x] P2 rbac gate — cronjob.trigger now EnabledWhen can.patch.
- [x] P3 secret — GetYAML + WatchObjectYAML strip `stringData` as well as `data`.
- [ ] P3 scale guard — generic.go:156-173 — guard scalable kinds / scale subresource → deferred (apiErr maps failures; no privilege gain).

## Phase 4 — K8s watch / live-refresh backend

- [ ] P1 errors — watch.go:50,140, events.go:99, watchhub.go:189 — watch errors swallowed; client sees frozen socket → surface terminal error frame / close so frontend shows error+reconnect.
- [ ] P2 resync — watchhub.go:164-166,189-190 — 410/Error restart from "current" with no re-LIST → emit resync signal so client re-lists (no phantom rows).
- [ ] P3 coalesce — watchhub.go:128-145 — broadcast can drop newest under contention → guarantee resend after drain / latest-wins slot.
- [ ] P3 caps — watchhub.go:84, session.go — no per-session feed/subscriber cap → bound.
- [ ] P3 encode — watch.go/events.go — encode error returns nil (looks clean) → distinguish client-gone from real error, log.
- [ ] P3 query keepalive — dispatch.go:716 — StreamQuery keepalive without controlReader → pongs unprocessed; add controlReader.

## Phase 5 — K8s resource browsing (table/tree)

- [ ] P2 watch-vs-page — rows.go:19 + TablePanel.vue:1088 — page>0/sort/filter triggers full refetch per event → suppress watch-refresh off page 0 or paginate-watch.
- [ ] P2 prepend — TablePanel.vue:1137 — live adds always prepend, reorder sorted lists; events grow unbounded → insert per sort / honor max cap.
- [ ] P2 visibility — TablePanel.vue:1146 — WS watch not paused on document.hidden → gate startWatch on visibility.
- [ ] P2 selectors — resources.go:65, watch.go — no label/field selector; client-side substring only → plumb selectors into list+watch.
- [ ] P2 ns param — watch.go:108, events.go — object/event watch read rc.Param raw vs list param() query-fallback → use param() everywhere.
- [ ] P2 forbidden state — TablePanel.vue:1620, errors.go:27 — forbidden/empty/no-ns collapse to one cryptic state → distinct friendly states.
- [ ] P2 rbac rows — resources.go:138, permissions.go:49 — delete shown regardless of perms → can map on list rows / clear message.
- [ ] P2 tree badges — ResourceTree.vue:198 — badges fetched once, never refresh → refresh on refreshKey/scope/watch.
- [ ] P2 a11y rows — TablePanel.vue:966 — no keyboard row activation → Enter opens detail/navigate.
- [ ] P3 misc — modified re-sort (TablePanel.vue:1128); CRD list unbounded (resources.go:76); reloadExpanded serial (ResourceTree.vue:131); CRD/Helm tree no watch; aria-live count; SSAR per-open (object_overview.go:24).

## Phase 6 — K8s pod operations

- [ ] P0 logs container — LogStreamPanel.vue + pod.go:31-36, logs.go:104 — no container picker; empty container 400s on multi-container pods → manifest-driven container select threading container param.
- [ ] P0 debug select — debug.go:19-48, routes.go:50 — image/target hardcoded busybox → Input schema (image text + target select).
- [ ] P1 logs controls — pod.go:34, LogStreamPanel.vue:16 — follow/tail/previous/timestamps fixed; "follow" only scrolls → controls that reconnect stream; previous/crashed logs.
- [ ] P1 logs frames — LogStreamPanel.vue:28-32 — JSON {ts,line} parse never matches raw byte stream → drop/branch per stream type.
- [ ] P1 files container — podfs.go:25-43, pod.go:46 — files hit default container; symlink target lost; no MIME → container picker; parse symlink+dir-ness; infer MIME.
- [ ] P1 metrics absent — MetricsPanel.vue:68-73 + metrics.go:153-165 — metrics-server absent hides known request/limit; generic message; pod limit gauges missing → render requests/limits + specific message + pod Usage gauges.
- [ ] P2 exec — exec.go:34-41, pod.go:38 — no container/shell picker; exit code hidden under TTY; keepalive → add pickers, surface exit, ping.
- [ ] P2 logs UX — LogStreamPanel.vue — wrap toggle; jump-to-latest; pause-on-scroll-up; bound workload fan-out (logs.go:81).
- [ ] P3 motion/a11y — gauge/pulse/cursor reduced-motion; log role=log aria-live; download as Blob; file save byte size.

## Phase 7 — Generic panels UX / a11y / consistency (ALL panels)

- [ ] P0 form a11y — FormField.vue:~371 — controls lack aria-invalid/aria-describedby → link errors to controls.
- [ ] P0 grid keyboard — FileEntryGrid.vue:54-109 — no keyboard nav; checkbox in span not operable → roving focus + operable checkbox.
- [ ] P1 menu — ActionBar.vue:457-512 — hand-rolled `<a>` items in PrimeVue Menu → item template/Button with roles.
- [ ] P1 cred error — CredentialSelect.vue:120 — load error bare text, no retry → PanelError-style retry.
- [ ] P1 table — TablePanel.vue:~1491,~1685,~1530 — badge `<span>`→Tag; delete error no retry; inline editors no validation/aria-invalid.
- [ ] P1 form groups — FormField.vue:295-312,278,211 — radio no fieldset/legend; slider no live value; FileUpload no loading/error/toast.
- [ ] P1 kv — KVPanel.vue:430-476,391,401 — labels unassociated; hardcoded copy; no detail skeleton.
- [ ] P1 copy — ObjectDetailPanel.vue:90, DocumentPanel.vue:51 — silent clipboard copy → toast.
- [ ] P1 diff — CodeDiffView.vue:53-57 — silent fallback to `<pre>`, no message → PanelError/message.
- [ ] P1 query — QueryEditorPanel.vue:306 — error not role=alert; headers no scope/caption.
- [ ] P1 streams — TerminalPanel.vue:241,387 / RemoteDesktopPanel.vue:173,297 / CanvasPanel.vue:304 — silent reconnect; implicit alert roles → toast + role=alert.
- [ ] P1 logclear — LogStreamPanel.vue:138 — destructive Clear silent, no confirm → confirm + toast + aria-labels.
- [ ] P1 termgrid — TerminalGridPanel.vue:280/349/367/385 — silent destructive split/close/reset → feedback/confirm.
- [ ] P1 file select — FileEntryGrid.vue:63/FileEntryList.vue:93 — aria-current misused → aria-selected.
- [ ] P1 file dnd — FileBrowserPanel.vue:240-246,760-766,1013 — drag-drop no keyboard alt/announce; chmod Select no aria-label.
- [ ] P1 http — HTTPClientPanel.vue:209,249 — silent send; inputs no aria-label.
- [ ] P1 task — TaskProgressPanel.vue:138,145 — silent cancel/retry; ProgressBar aria-label no percent.
- [ ] P2 batch — reduced-motion gating (GaugeChart/SeriesChart/StreamStatusBar/ConnectPanel/Terminal/Table draggable); aria-labels (object-detail/document/log/term buttons); skeletons (StatCard/KV detail/RemoteDesktop); DockPanel native `<button>` tabs + resize keyboard; EnrollPanel invalid `variant="text"` + silent download; FieldGroup fieldset; JsonNode treeitem roles; ScopeBar double-label; ArrayField/MapField hardcoded copy.
- [ ] P3 batch — hardcoded user-facing strings → config (Fallback/Wasm/Dashboard/Trace/HTTPClient/QueryEditor export); export Menu icon class; submit aria-busy; terminal search debounce; severity contrast check.

---

## Cross-cutting refactors (do early, reused by many fixes)

- [ ] Standardize error states on `PanelError` (role=alert + retry) and loading on `SkeletonList` (motion-safe) — replace ad-hoc red `<p>` and bare spinners.
- [ ] `useNotify` success/error helper used by every side-effecting action.
- [ ] `prefers-reduced-motion` honored via `motion-safe:`/media query everywhere motion is used.
