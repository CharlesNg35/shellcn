# ShellCN — Progress Checklist

**Living progress tracker — update after every completed step.** This is the
single source of truth for "where are we." Detailed steps (sub-task checklists +
Definitions of Done) live in [`specs/plans/`](specs/plans/); architecture in
[`specs/v2.md`](specs/v2.md); test standard in
[`specs/plans/TESTING.md`](specs/plans/TESTING.md).

_Last updated: 2026-05-27 — Phase 6 (M5 PostgreSQL) is complete, and the MySQL/MariaDB, Redis, MongoDB, MSSQL, Oracle, CockroachDB, ClickHouse, Cassandra, DynamoDB, and Neo4j first-party plugins are now implemented as direct-only database/data-store plugins. PostgreSQL and CockroachDB use pgxpool over `cfg.Net.DialContext`; MySQL/MariaDB uses `go-sql-driver/mysql`; Redis uses `go-redis`; MongoDB uses the official MongoDB Go driver; MSSQL uses `go-mssqldb`; Oracle uses pure-Go `go-ora/v2`; ClickHouse uses the official `clickhouse-go/v2` native driver; Cassandra uses `gocql`; DynamoDB uses the AWS SDK for Go v2; Neo4j uses the official Neo4j Go driver v6; all use the same direct net transport. RabbitMQ, Kafka, and NATS are implemented as direct-only messaging plugins under the core messaging category: RabbitMQ uses the management API for queues, exchanges, bindings, consumers, message peek, and publish; Kafka uses Sarama for brokers/topics/partitions, consumer groups, offsets, recent message reads, and production; NATS uses the official client for server info, JetStream streams, consumers, stored messages, and publish. Elasticsearch, OpenSearch, Meilisearch, Typesense, and Solr are implemented as direct-only search plugins under the core search category. Elasticsearch/OpenSearch use a shared Elasticsearch-compatible REST helper for indexes, mappings, settings, aliases, shards, JSON DSL search, and document CRUD. Meilisearch adds index/document management, JSON search, async task tracking/cancel, API keys, dumps, and snapshots. Typesense adds collection/schema management, document search/import/export, aliases, global synonym sets, global curation sets, API keys, stats, and metrics. Solr adds user-managed core and SolrCloud collection management, document CRUD, select-query execution, managed schema fields, config, ping, commit, optimize, and delete-by-query. All five search plugins have live container integration coverage. Prometheus is implemented as a direct-only observability plugin with PromQL instant/range query, targets, active alerts, rules, labels, metric metadata, series, status documents, live overview metrics, and gated admin/lifecycle operations. PostgreSQL, MySQL/MariaDB, MSSQL, Oracle, CockroachDB, ClickHouse, and Cassandra use reusable `db_password` credentials, manifest-driven schema/table browsing, table data panels, completion, audited query execution, read-only/destructive-statement controls, and result redaction. DynamoDB uses cloud access key credentials or AWS default provider chain auth and exposes tables, indexes, items, TTL, tags, backups, item editing, guarded actions, and PartiQL. Neo4j uses Neo4j-specific password/stored-password/bearer/stored-bearer/none auth, scheme-driven TLS, database/label/relationship-type/schema browsing, graph visualization, guarded node/relationship mutations, and Cypher execution. MySQL/MariaDB adds database/table/view/routine/user resources, DDL helpers for table/column creation, and user browsing where the database grants access to `mysql.user`. MSSQL adds database/schema/table/view/procedure/user/job resources, T-SQL query execution, table/column DDL helpers, SQL Server catalog panels, and live SQL Server integration coverage. Oracle adds schema/table/view/procedure/package/sequence/user/tablespace/session resources, Oracle SQL and PL/SQL catalog panels, DBA-to-ALL/USER catalog fallbacks, table/column DDL helpers, and live Oracle Database Free integration coverage. CockroachDB adds database/schema/table/view/function resources plus nodes, ranges, jobs, sessions, active queries, CockroachDB SQL safety rules, SHOW/information_schema catalog routes, and live CockroachDB integration coverage. ClickHouse adds database/table/view/dictionary/mutation/merge/process/user resources, MergeTree-oriented DDL helpers, ClickHouse SYSTEM/KILL/OPTIMIZE safety rules, system-table catalog panels, and live ClickHouse integration coverage. Cassandra adds keyspace/table/materialized-view/type/function/node resources, CQL query execution, token-aware/DC-aware cluster settings, Cassandra-specific paging, table/keyspace/column DDL helpers, and CQL safety rules. Redis adds a generic-panel cockpit: overview/info documents, SCAN-backed typed key browser for string/hash/list/set/zset plus stream preview, generic KV create/save/delete, Redis command console with completion and safety gates, client list, and pub/sub channel list. MongoDB adds database/collection resources, collection stats, document table browsing, Extended JSON document create/edit/delete through generic actions/code editor, index browsing, and a MongoDB command console with read-only/write-confirmation safety gates. SQL/CQL plugins share driver-neutral helpers in `plugins/shared/sqldb` for query envelopes, identifier/DDL validation, statement safety checks, TLS config, common config parsing, audit metadata, completion items, and result redaction. The renderer contract includes `treeGroup.resourceKind` so lazy tree sources and group-click resource lists stay explicit instead of inferred from route IDs. Numeric config defaults are covered by registry tests so defaults cannot exceed their validators. Database/data-store, messaging, search, and observability plugins do not use agent transport; agent remains for Docker and later Kubernetes-style private control planes. Telnet is now a direct-only first-party terminal plugin with manifest-declared asciicast recording. Earlier phases through M5 are complete; live validation against a real Proxmox cluster is still pending. **Phase 7 (M6 Kubernetes) is now implemented**: a Lens/OpenLens-grade cockpit built with client-go over **both** direct (kubeconfig) and **L7-agent** transport. The agent gained a generic, plugin-agnostic `http_proxy` reverse-proxy mode (credential injection via declared token/CA file paths — zero Kubernetes vocabulary in the agent/transport core), and install artifacts can now be **URL-delivered** via a single-use signed-ticket fetch (generic `InstallArtifact` capability; token minted into the body, never a path). The plugin is fully manifest-driven off a single kind catalog (Lens-style categorized nav, ~25 built-in kinds + runtime CRDs, generic `{kind}`-parameterized list/get/watch/delete/scale/restart/cordon, live `ResourceEvent` watch, Secrets redacted), with pod logs/exec/port-forward over both transports (agent upgrades via a per-session loopback bridge), editable YAML + dynamically-generated per-kind Create + server-side apply with dry-run + events, and cluster/node/workload overviews with metrics-server gauges/series that degrade gracefully. The only frontend change was a generic `Action.Config` primitive (completing step-0 create-from-content); the k8s package added zero frontend code. Live validation against a real cluster is still pending._

Legend: `[ ]` todo · `[~]` in progress · `[x]` done.
A step is `[x]` only when its **tests pass**; a phase is done when all its steps are `[x]`.

## Phase 0 — Bootstrap

- [x] 0.1 Initialize Go module and repo skeleton
- [x] 0.2 Scaffold the Vue + Vite frontend
- [x] 0.3 Makefile and developer tooling

## Phase 1 — M0 · Declarative UI on fixtures (priority)

- [x] 1.1 Define the projection contract (TypeScript)
- [x] 1.2 Author fixture manifests and mock dev server
- [x] 1.3 App shell, stores, and routing
- [x] 1.4 Manifest renderer and panel dispatch
- [x] 1.5 DataSource resolver
- [x] 1.6 Declarative panels, including specialized graph/trace/kv/http-client renderers and grouped panel source layout
- [x] 1.7 Stub streaming panels

## Phase 2 — M1 · Core runtime

- [x] 2.1 Package skeleton and plugin contract types
- [x] 2.2 Manifest validator and browser projection
- [x] 2.3 GORM models and store repositories
- [x] 2.4 Secret vault
- [x] 2.5 Authentication and sessions
- [x] 2.6 Authorization with Casbin
- [x] 2.7 Session, channel, and transport runtime
- [x] 2.8 chi server and route wrapper
- [x] 2.9 Audit and telemetry
- [x] 2.10 Test plugin and end-to-end validation

## Phase 2b — M1.5 · Platform management (make it usable)

_Done — control-plane CRUD + platform UI (spec [v2 §12.2](specs/v2.md), steps [phase-2b](specs/plans/phase-2b-m1.5-platform-management/)). Connection/credential CRUD + sharing endpoints with authn→authz→audit; auth gate + global error UX; manifest-driven connection create/edit/delete with category-grouped protocol picking; credential management + sharing UI. Shared connections can use already-bound credentials without exposing those credentials; shared managers see keep-or-replace credential refs, and credential grants remain managed from the credentials surface. All secrets write-only end to end._

- [x] 2b.1 Backend — connection CRUD endpoints (schema-validated, secret-encrypted, authz'd)
- [x] 2b.2 Backend — credential CRUD + rotation (write-only secret material)
- [x] 2b.3 Backend — sharing grants endpoints (connection + credential; use/manage)
- [x] 2b.4 Frontend — auth/session gate + global error/authz UX (login, CSRF, 401→login, logout)
- [x] 2b.5 Frontend — connection management UI (manifest-driven create/edit/delete + transport selector)
- [x] 2b.6 Frontend — credential management + sharing UI (create/rotate/delete, grant use/manage)

## Phase 2c — M1.6 · Session recording foundation

_Done — recording is a generic, plugin-declared, off-by-default platform capability (spec [v2 §9.5](specs/v2.md), steps [phase-2c](specs/plans/phase-2c-m1.6-session-recording/)). Plugins declare recordable stream classes (`terminal`/`desktop`) + formats via `RecordingCapability`; connections carry a per-class policy (`disabled`/`manual`/`auto`=forced). The core stream wrapper taps recordable WS streams (forced denies the stream up front if it can't start; manual start/stops; bounded buffering never blocks the live stream). Terminal → asciicast v2; desktop → browser `webm_canvas` chunk uploads (non-authoritative). Metadata in a new `Recording` model + `RecordingStore`; bytes in a replaceable `BlobStore` (local FS default). Role-aware list/get/content/delete APIs (admin all + per-user drill-down; non-admins only their own recordings), retention OFF by default (`config.recordings`), cleanup job when enabled. Frontend: Recordings view + asciinema/WebM players, per-panel REC state + manual start/stop, connection create/edit policy options only when the plugin declares support._

- [x] 2c.1 Recording manifest contract + connection policy
- [x] 2c.2 Recording storage, metadata, retention, and authorization
- [x] 2c.3 Core stream recording wrapper and lifecycle
- [x] 2c.4 Terminal asciicast recorder and playback
- [x] 2c.5 Desktop/graphical recording framework
- [x] 2c.6 Recording APIs and frontend management UI

## Phase 2d — M-Admin · Administration foundation

_Done — user/role management + invitations + the config foundation they need
(spec [v2 §12.2](specs/v2.md), [v2 §9.1](specs/v2.md), steps
[phase-2d](specs/plans/phase-2d-m-admin/)). SMTP is bootstrap config
(`config.email.*`), not a stored table; invitations always yield a copyable
link, with email as a best-effort extra when SMTP is enabled._

- [x] 2d.1 Backend — typed bootstrap config (`internal/config`, Viper: `config.yaml` + `SHELLCN_*` env + flag overrides; master key unified)
- [x] 2d.2 Backend — admin user CRUD (`/api/admin/users`) with root-admin protection (root never deleted/locked out; only root deletes admins); audited
- [x] 2d.3 Backend — invitations create/list/revoke (`/api/admin/invitations`) + public lookup/accept (`/api/invitations/{token}`, single-use); config-driven SMTP via `internal/email`
- [x] 2d.4 Frontend — Users view (Users · Invitations tabs): create/edit/delete users, invite → copyable link, revoke, public accept page, admin-only nav + email status in Settings

> **Still M-Admin (later):** policy-rule admin (`role+permission+risk`), audit-log view + per-connection activity, light status page (health/plugin-health/session counts), agent re-enroll/rotate + history.

## Phase 3 — M2 · SSH/SFTP reference plugin

_Done — SSH and SFTP are separate compiled-in plugins with shared SSH/SFTP session and file-route code. `ssh` exposes Terminal, Files, and command Snippets; `sftp` exposes the same generic file browser only. SSH/SFTP auth supports password, private key, and stored credential without extra trust or SSH-agent configuration. SFTP opens lazily over the same SSH client, guarded by the session mutex. Terminal streaming is real xterm.js ↔ `ssh.shell` with resize control frames, theme-aware light/dark palettes, and the shared stream status/reconnect UX used by logs, metrics, query, and remote desktop panels; file browser routes implement list/read/download/upload/mkdir/rename/delete with core-streamed downloads and audit/authz wrapper coverage. SSH snippets use manifest-declared table actions for create/run/delete; the generic table renderer now understands toolbar and selected-row actions, plus validated `onSuccess.selectTab` navigation so snippet run returns to Terminal. The shipped placeholder `noop` plugin was removed; server e2e now uses an internal test-only plugin._

- [x] 3.1 SSH session and Connect
- [x] 3.2 SSH routes and manifest
- [x] 3.3 Wire the real terminal panel
- [x] 3.4 Wire the real file browser panel

## Phase 4 — M3 · Docker + agent transport

- [x] 4.1 Docker session and resource routes
- [x] 4.2 Docker manifest (tree, resources, actions)
- [x] 4.3 Real logs, exec, and watch streams
- [x] 4.4 Harden shellcn-agent L4 tcp/unix against Docker
- [x] 4.5 Harden enrollment/tunnel registry with Docker agent mode
- [x] 4.6 Wire and prove agent transport in Docker connection

## Phase 5 — M4 · Proxmox

- [x] 5.1 Proxmox session and API client
- [x] 5.2 Proxmox manifest (nodes, VMs, LXC, storage)
- [x] 5.3 Real noVNC/RFB remote-desktop panel
- [x] 5.4 Snapshots, backups, and lifecycle actions

## Phase 6 — M5 · PostgreSQL

- [x] 6.1 PostgreSQL session and schema browser
- [x] 6.2 Real query editor and results panel
- [x] 6.3 Database safety controls

## Database client UX upgrade (TablePlus/Beekeeper-class)

Generic, manifest-driven; no per-plugin frontend.

- [x] UX.1 Contract: editable `TableConfig` (insert/update/delete, rowKey, editable)
      + `ResourceRef.Scope` for database/cluster hierarchies (`internal/plugin/ui.go`,
      `web/src/types/projection.ts`, `specs/v2.md`)
- [x] UX.2 Generic editable data grid in `TablePanel.vue` (inline cell edit,
      add-row, delete-row) + driver-neutral row DML in `plugins/shared/sqldb`
- [x] UX.3 PostgreSQL reference refactor: per-database pools (fixes cross-database
      query scoping), hierarchical Databases→Schemas→Tables tree, editable Data
      grid, full CRUD (create/drop database & schema, create/drop/truncate table,
      add column, row insert/update/delete), DDL view
- [x] UX.4a Editable grid + row CRUD via `sqldb.Dialect` (+ primary-key `_key`
      tagging + tests) on **mysql** (`?`/backtick), **cockroachdb** (`$`/ANSI),
      **mssql** (`@pN`/`[ ]`, PK via `sys.indexes`), **oracle** (`:N`/`"`, PK via
      `all_constraints`; Data grid keeps real uppercase column names so quoted DML
      round-trips)
- [x] UX.4b Cassandra integration test (docker-based, skipped without
      `SHELLCN_CASSANDRA_INTEGRATION=1`)
- [x] UX.6 Navigation parity — hierarchical drill-down tree (single rooted
      group → lazy children → leaves, via `ResourceRef.Scope` + `ChildrenSource`,
      no contract change) across **all** DB plugins: mysql/clickhouse/cassandra
      (database/keyspace → tables/views), mongodb (database → collections),
      oracle/cockroachdb (schema → tables/views), mssql (database → schema →
      table/view, 3-level). Validated with docker for mysql + mssql (tree
      assertions) and regression-checked for the rest.
- [x] UX.8 Column/index management as declarative actions (drop column, create
      index, drop index — reusing the action+form renderer, **no frontend code**),
      done + docker-validated on **every** DB plugin: postgresql, mysql,
      cockroachdb, oracle, mssql (full, dialect-correct DROP INDEX), cassandra
      (CQL single-column secondary index), clickhouse (drop column + drop
      data-skipping index; ADD INDEX needs a TYPE so it stays in the SQL tab).
- [x] UX.7 SQL autocomplete is now context-aware via `@codemirror/lang-sql`
      schema completion (tables after FROM/JOIN, columns after `table.`), built
      from the catalog with a flat keyword/function fallback (unit-tested).
- [x] UX.5 Generic CSV/JSON export of loaded rows in the table grid + query
      editor, **opt-in per plugin** via manifest (`TableConfig.Exportable` /
      query-editor `exportable`; off by default). Enabled on the DB plugins'
      data grids + query/command editors (postgresql, mysql, cockroachdb, mssql,
      oracle, clickhouse, cassandra, mongodb) and Redis clients/channels tables.
- [x] UX.12 Workbench renderer primitives (Phase-7 step 0, landed early & generic —
      proven on existing plugins, no k8s yet):
      - `dashboard` **panel** (`PanelDashboard` + `DashboardConfig.Cells`) — a
        multi-panel grid usable as any detail/connection tab, sharing
        `DashboardGrid` with the dashboard layout. Showcase: Redis Overview.
      - Generic **metrics panel** (`MetricsConfig` stats/gauges/series via PrimeVue
        `Chart`/chart.js, theme-aware, lazy-loaded) — renderer hardcodes no field
        names. Showcase: Proxmox node/VM/LXC.
      - **List-opening nav** (`TreeNode.ResourceKind` + optional `ListParams` to
        scope, e.g. a namespace) — a tree node opens a kind's list view (vs. detail).
      - **Multi-open workbench tabs** — the sidebar-tree workspace keeps several
        open views (details + lists) as a closable tab strip with `KeepAlive`
        (extracted into `TreeWorkspace.vue`), instead of one selection at a time.
      - **Bottom dock** (`Action.Open` view/dock/dialog + `Action.Panel`) — a
        resizable, tabbed, `KeepAlive` dock (`DockPanel.vue` + per-connection dock
        store) hosting terminals/logs/editors from actions, or a modal. Showcase:
        Docker "Logs in dock".
      - Also fixed two manifest-driven leaks found en route: the metrics panel
        (`cpu/mem`) and KV panel value types are now config-driven.
      All Go+TS contract-mirrored, validated, unit-tested; gates green.
- [x] UX.11 Third workspace layout `dashboard` (`LayoutDashboard`) — renders every
      connection-level `Tab` panel at once in a responsive grid via the focused
      `DashboardWorkspace.vue` shell, with an optional per-`Tab` `Span` sizing hint
      (>= 2 fills the row). Contract mirrored in Go + TS; validator accepts it;
      covered by `ConnectionWorkspace.test.ts` and a Go layout-validation test.
      Intended for multi-panel overviews (e.g. upcoming Kubernetes summaries).
- [x] UX.10 Staged grid editing (commit/discard) — opt-in `TableConfig.StagedEdits`
      makes the generic grid buffer cell edits, added rows, and deletions locally
      (pending cells/rows highlighted, count + Commit/Discard bar) instead of
      applying each change immediately; commit replays the buffer through the
      existing per-row Insert/Update/Delete routes, discard reverts. Enabled on
      postgresql, mysql, cockroachdb, mssql, oracle data grids. Unit-tested in
      `TablePanel.test.ts` (buffer→commit update, discard, staged delete).
- [x] UX.9 Foreign-key navigation in the data grid — relational plugins attach a
      generic `_links` map (column → `ResourceRef`) to rows; the grid renders those
      cells as buttons that emit `select` to open the referenced table (reuses the
      existing row-select navigation, no new plumbing). Wired for postgresql,
      mysql, cockroachdb, mssql, oracle; docker integration tests assert
      `_links` points at the parent table.
- [x] UX.4d Hardening + verification (all docker integration tests run & green):
      row mutations validate the client key is exactly the primary key
      (`sqldb.ValidateRowKey`) and require one affected row; `_key` is withheld
      when a PK column is itself redaction-sensitive (`sqldb.AnyColumnRedacted`);
      Postgres closes the cached pool before `DROP DATABASE`; grid delete button
      stops click propagation. Integration tests exercise row insert/update/
      delete, non-PK-key rejection, drop-after-browse, and structure/catalog
      routes across postgresql, mysql, cockroachdb, mssql, oracle.
- [ ] UX.4c Deferred editable grids, with reasons:
      - cassandra — gocql is strongly typed; JSON-decoded values won't bind back
        into `uuid`/`int`/`timestamp` columns. Needs type-aware value coercion
        from `system_schema.columns` before a grid is safe. `_key` must be the
        full PARTITION+CLUSTERING key.
      - clickhouse — no row UPDATE/DELETE (mutations are async `ALTER`); stays
        read-only, optionally insert-only later.
      - mongodb / redis — already mutable (document code-editor / writable KV).

## Additional first-party plugins

- [x] Telnet — direct terminal plugin using the generic terminal panel and core terminal recording path
- [x] RabbitMQ — direct messaging plugin for queues, exchanges, bindings, consumers, message peek, and publish
- [x] Kafka — direct messaging plugin for topics, partitions, consumer groups, offsets, recent messages, and produce
- [x] NATS — direct messaging plugin for server info, JetStream streams, consumers, messages, and publish
- [x] Prometheus — direct observability plugin for PromQL instant/range query, targets, alerts, rules, labels, metadata, series, status, live metrics, and gated admin/lifecycle APIs
- [x] InfluxDB — direct observability/time-series plugin for v3/v2/v1 APIs, mode-specific auth fields, database/bucket browsing, measurements, schema, data preview, Flux/SQL/InfluxQL queries, and line-protocol writes
- [x] DynamoDB — direct database/data-store plugin using AWS SDK v2 for tables, indexes, items, TTL, tags, backups, item editing, guarded table/index/item/backup/TTL actions, and PartiQL
- [x] Neo4j — direct graph database plugin using the official Neo4j Go driver v6 for databases, labels, relationship types, schema, nodes, relationships, graph visualization, guarded graph mutations, and Cypher
- [x] Elasticsearch — direct search plugin for indexes, mappings, settings, shards, JSON DSL search, and document CRUD
- [x] OpenSearch — direct search plugin for indexes, mappings, settings, shards, JSON DSL search, and document CRUD
- [x] Meilisearch — direct search plugin for indexes, documents, JSON search, settings, async tasks, API keys, dumps, and snapshots
- [x] Typesense — direct search plugin for collections, schemas, documents, aliases, global synonym sets, global curation sets, API keys, stats, and metrics
- [x] Solr — direct search plugin for user-managed cores, SolrCloud collections, documents, select queries, managed schema fields, config, ping, commit, optimize, and delete-by-query
- [x] LDAP — direct directory plugin (security category) using `go-ldap/v3` over `cfg.Net.DialContext` (plain / StartTLS / LDAPS, simple + anonymous + stored bind): sidebar DIT tree with lazy one-level expansion and `hasSubordinates` leaf detection, entry attributes rendered in the generic staged editable grid (Modify add/replace/delete), entry add/rename(modifyDN)/delete actions, subtree LDAP-filter search, read-only safety gate. Searches use the Simple Paged Results control (bounded by the size limit) so they work against Active Directory's per-request `MaxPageSize`; AD object-class icons included. Live coverage against both OpenLDAP and Samba AD-DC.

## Phase 7 — M6 · Kubernetes (Lens/OpenLens-grade, manifest-driven)

- [x] 7.0 Workbench renderer extensions (**generic, cross-plugin**): bottom dock, dashboard-as-view, multiple open workbench tabs, metrics/stat panel, list-opening nav nodes (+ scoped params) — landed early & proven on Redis/Proxmox/Docker/DBs (see UX.11/UX.12). Only the optional create-resource template picker remains.
- [x] 7.1 Kubernetes session and L7 agent mode — client-go over a generic L7 (`http_proxy`) agent that injects the target's own credentials (token/CA files declared by the plugin, not the agent); `rest.Config` from kubeconfig (direct) or `cfg.Net.HTTP()` (agent); typed + dynamic + discovery + RESTMapper + metrics clients; `kubectl apply` install manifest (token in the manifest body). Lists namespaces over both transports. `make fmt/lint/test` green.
- [x] 7.2 Workloads and core resource trees — Lens-style menu (Nodes, Workloads, Config, Network, Storage, Namespaces, Events, Access Control, Custom Resources) driven by a single kind catalog; ~25 built-in kinds with curated columns + runtime-discovered CRDs; generic catalog-parameterized list/get/watch/delete/scale/restart/cordon routes (one set serves every kind); live watch → `ResourceEvent`; namespace scoping; Secrets redacted; per-resource Overview tab. `make fmt/lint/test` green.
- [x] 7.3 Pod logs, exec, and port-forward — logs (chunked GET), exec (SPDY/WebSocket fallback), and port-forward (raw pod-port tunnel) over **both** direct and agent transport. Agent upgrades ride a per-session loopback bridge (client-go upgraders ignore custom dialers, #129915). Surfaced as dock actions (Logs/Shell with `Open:dock`) + pod detail tabs; exec is recordable (asciicast). `make fmt/lint/test` green.
- [x] 7.4 YAML editor, Create Resource, and events — editable YAML detail tab + dock "Edit YAML" action; per-kind **Create** (list action) opening a starter manifest **generated dynamically from the resolved GVK**; server-side apply (create-or-update) with dry-run; resource-scoped Events tab + live cluster Events. Completed step-0's create-from-content generically (`Action.Config` → dock/dialog panel). `make fmt/lint/test` green.
- [x] 7.5 Cluster, node, and workload overviews — Cluster Overview (top tree node) is a `PanelDashboard` of live CPU/Memory gauges + pod/node stats + CPU/mem time-series + node list + recent events; Node detail adds a live Metrics panel + scheduled-pods table; workloads (Deployment/StatefulSet/DaemonSet/ReplicaSet) add an owned-pods table. Metrics come from metrics-server (`metrics.k8s.io`) and degrade gracefully when absent (`metricsAvailable:false`). Namespace overview uses the generic detail + events; the Prometheus metrics source is config-declared but the adapter is deferred. `make fmt/lint/test` green.

---

**On completing a step:** mark it `[x]` here, update the `_Last updated_` line,
set `Status: ✅ Done` in the step file (add date/PR), and confirm its tests pass.
