# ShellCN Plugin Roadmap

All plugins are planned as first-party, compiled-in Go plugins. This list is a
product and architecture backlog: adding a plugin should mean adding one Go
package that declares a manifest, routes, resources, actions, streams, and
session behavior without requiring frontend-specific code.

Credential kinds follow the same ownership rule. Core keeps only broad reusable
shapes such as database passwords, API tokens, TLS client certs, cloud keys,
service account JSON, basic auth, and bearer tokens. Protocol-specific kinds
such as SSH private keys/passwords, kubeconfig, VNC/RDP/SMB passwords, and SNMP
material are declared by the plugin that owns them. Protocol compatibility is
derived from registered plugin `credential_ref` selectors; it is not maintained
as a hardcoded list on the kind itself.

Plugin categories are also core-owned. Each manifest declares one builtin
category key so management UI can group protocols without hardcoded frontend
protocol lists. Current groups are shell, files/storage, containers,
virtualization, remote desktop, databases, orchestration, cloud, network,
security, DevOps/CI, observability, messaging, and other.
Search engines are a separate category from databases because products such as
Elasticsearch/OpenSearch, Meilisearch, Typesense, and Solr expose different
operational and query models even when they all manage searchable documents.

## Priority Legend

- `P0`: MVP foundation.
- `P1`: High-value infrastructure targets after MVP.
- `P2`: Important expansion plugins.
- `P3`: Later/niche integrations.

## P0: Reference Plugins

These plugins should prove the core architecture first.

| Plugin       | Purpose                | Main Capabilities                                         |
| ------------ | ---------------------- | --------------------------------------------------------- |
| `ssh`        | Remote shell access    | terminal, **SFTP (Files tab)**, command snippets          |
| `sftp`       | File-only access       | filesystem, upload/download, editor, permissions          |
| `docker`     | Docker host management | containers, images, volumes, networks, logs, exec, stats  |
| `postgresql` | PostgreSQL access      | schema browser, query editor, table data, snippets, audit |

> **`ssh` vs `sftp`:** an `ssh` connection exposes SFTP as its **Files** tab over
> the _same_ `ssh.Client` (no second connection / re-auth). The standalone `sftp`
> plugin is for users who want **file access only** (no shell). Both
> render the same `file_browser` panel and share the SFTP route handlers — the
> only difference is the manifest each declares. The frontend special-cases
> neither (v2 §12, §13).

> **SQL plugins:** PostgreSQL, MySQL/MariaDB, MSSQL, Oracle, CockroachDB,
> ClickHouse, Cassandra, SQLite, and later SQL/CQL engines share only driver-neutral helpers from `plugins/shared/sqldb`
> (query editor envelopes, identifier/DDL helpers, statement safety checks,
> audit metadata/result redaction, and TLS/config parsing). Dialect catalog
> queries, driver connection code, actions, and manifests remain inside each
> plugin. PostgreSQL, MySQL/MariaDB, MSSQL, Oracle, CockroachDB, ClickHouse,
> Redis, MongoDB, and Cassandra are implemented as direct-only database/data-store plugins;
> agent transport is reserved for private control-plane targets such as Docker
> and Kubernetes.

## P1: Core Infrastructure

| Plugin        | Purpose                       | Main Capabilities                                                                   |
| ------------- | ----------------------------- | ----------------------------------------------------------------------------------- |
| `proxmox`     | Proxmox VE management         | nodes, VMs, LXC, storage, network, snapshots, VNC console, tasks                    |
| `kubernetes`  | Kubernetes cluster management | workloads, pods, services, ingress, storage, config, RBAC, logs, exec, port-forward |
| `mysql`       | MySQL/MariaDB access          | schema browser, query editor, table data, users, DDL helpers, audit                 |
| `mongodb`     | MongoDB access                | databases, collections, document editor, command console, indexes                   |
| `redis`       | Redis access                  | key browser, strings, hashes, lists, sets, sorted sets, command console, pub/sub    |
| `mssql`       | Microsoft SQL Server          | schema browser, T-SQL editor, table data, jobs, users, DDL helpers, audit           |
| `oracle`      | Oracle Database               | schemas, SQL editor, PL/SQL objects, sessions, tablespaces, DDL helpers, audit      |
| `cockroachdb` | CockroachDB access            | schemas, SQL editor, table data, ranges, jobs, sessions, DDL helpers, audit         |
| `clickhouse`  | ClickHouse analytics DB       | databases, tables, views, dictionaries, mutations, merges, processes, SQL editor    |
| `cassandra`   | Cassandra access              | keyspaces, tables, materialized views, types, functions, CQL query                  |
| `vnc`         | Remote desktop via VNC/RFB    | `remote_desktop` over an RFB stream, clipboard, keyboard/mouse                      |

## P1: Filesystem And Storage Protocols

| Plugin   | Purpose                      | Main Capabilities                                            |
| -------- | ---------------------------- | ------------------------------------------------------------ |
| `ftp`    | FTP access                   | file browser, upload/download, rename/delete                 |
| `ftps`   | FTP over TLS                 | file browser, upload/download, TLS config                    |
| `webdav` | WebDAV storage               | file browser, upload/download, locks where supported         |
| `smb`    | SMB/CIFS shares              | file browser, share browsing, upload/download                |
| `nfs`    | NFS shares                   | file browser, mounts/exports where applicable                |
| `s3`     | S3-compatible object storage | buckets, objects, upload/download, metadata, presigned links |
| `minio`  | MinIO object storage         | buckets, objects, policies, users, service accounts          |

## P2: Databases And Data Stores

| Plugin       | Purpose               | Main Capabilities                                       |
| ------------ | --------------------- | ------------------------------------------------------- |
| `sqlite`     | SQLite database files | schema browser, query editor, table data                |
| `neo4j`      | Graph database        | Cypher query, graph/table results                       |
| `influxdb`   | Time-series database  | buckets/databases, query, measurements                  |
| `prometheus` | Metrics query target  | PromQL query, targets, alerts, rules                    |

## P2: Container And Orchestration Platforms

| Plugin       | Purpose                          | Main Capabilities                                       |
| ------------ | -------------------------------- | ------------------------------------------------------- |
| `podman`     | Podman host management           | containers, pods, images, volumes, networks, logs, exec |
| `containerd` | containerd runtime               | namespaces, containers, images, tasks, logs             |
| `nomad`      | HashiCorp Nomad                  | jobs, allocations, nodes, logs, exec                    |
| `swarm`      | Docker Swarm                     | services, stacks, nodes, tasks, logs                    |
| `helm`       | Helm releases through Kubernetes | releases, values, history, rollback                     |
| `argocd`     | Argo CD applications             | apps, sync, health, manifests, events                   |
| `flux`       | Flux CD resources                | kustomizations, helm releases, sources, reconciliation  |

## P2: Virtualization And Remote Desktop

| Plugin           | Purpose                     | Main Capabilities                                               |
| ---------------- | --------------------------- | --------------------------------------------------------------- |
| `rdp`            | Windows/Linux RDP access    | `remote_desktop`; server-side RDP decoding bridged to noVNC/RFB |
| `xenserver`      | XenServer/XCP-ng management | hosts, VMs, storage, networks, console                          |
| `vmware-vsphere` | VMware vSphere              | datacenters, clusters, hosts, VMs, datastores, console          |
| `libvirt`        | libvirt/KVM management      | domains, networks, storage pools, console                       |
| `incus`          | Incus/LXD management        | instances, images, profiles, networks, storage, console         |
| `lxd`            | LXD management              | containers, VMs, images, profiles, networks, storage            |

## P2: Cloud Providers

| Plugin         | Purpose            | Main Capabilities                                     |
| -------------- | ------------------ | ----------------------------------------------------- |
| `aws`          | AWS infrastructure | EC2, ECS, EKS, RDS, S3, IAM read views, CloudWatch    |
| `gcp`          | Google Cloud       | Compute Engine, GKE, Cloud SQL, GCS, IAM read views   |
| `azure`        | Microsoft Azure    | VMs, AKS, SQL, Blob Storage, resource groups          |
| `cloudflare`   | Cloudflare         | DNS, tunnels, access apps, workers, logs              |
| `digitalocean` | DigitalOcean       | droplets, Kubernetes, databases, volumes, firewalls   |
| `hetzner`      | Hetzner Cloud      | servers, volumes, networks, firewalls, load balancers |
| `linode`       | Akamai/Linode      | instances, volumes, Kubernetes, object storage        |
| `ovh`          | OVHcloud           | VPS/dedicated/cloud resources, storage                |
| `vultr`        | Vultr              | instances, Kubernetes, block storage, firewalls       |

## P2: Network And Security Devices

| Plugin      | Purpose                | Main Capabilities                        |
| ----------- | ---------------------- | ---------------------------------------- |
| `snmp`      | SNMP device monitoring | interfaces, metrics, system info         |
| `mikrotik`  | MikroTik RouterOS      | interfaces, routes, firewall, DHCP, logs |
| `pfsense`   | pfSense firewall       | interfaces, rules, gateways, VPN status  |
| `opnsense`  | OPNsense firewall      | interfaces, rules, gateways, VPN status  |
| `wireguard` | WireGuard management   | peers, status, config, traffic           |
| `openvpn`   | OpenVPN management     | clients, sessions, status, logs          |
| `tailscale` | Tailscale tailnet      | devices, users, ACL visibility           |
| `zerotier`  | ZeroTier networks      | members, networks, routes                |
| `ipmi`      | Server BMC/IPMI        | power control, sensors, event log        |
| `redfish`   | Server Redfish API     | power, sensors, inventory, event log     |

## P2: DevOps And CI/CD

| Plugin            | Purpose                    | Main Capabilities                                           |
| ----------------- | -------------------------- | ----------------------------------------------------------- |
| `github`          | GitHub operations          | repos, actions, environments, deployments, secrets metadata |
| `gitlab`          | GitLab operations          | projects, pipelines, runners, environments                  |
| `gitea`           | Gitea/Forgejo              | repos, actions, releases, users                             |
| `jenkins`         | Jenkins                    | jobs, builds, logs, nodes                                   |
| `buildkite`       | Buildkite                  | pipelines, builds, agents                                   |
| `terraform-cloud` | Terraform Cloud/Enterprise | workspaces, runs, state versions                            |
| `vault`           | HashiCorp Vault            | mounts, policies, secret metadata, leases                   |
| `openbao`         | OpenBao                    | mounts, policies, secret metadata, leases                   |

## P2: Observability And Logging

| Plugin            | Purpose                   | Main Capabilities                   |
| ----------------- | ------------------------- | ----------------------------------- |
| `grafana`         | Grafana                   | dashboards, datasources, alerts     |
| `loki`            | Loki logs                 | log query, labels, streams          |
| `tempo`           | Tempo traces              | trace search and detail             |
| `jaeger`          | Jaeger traces             | trace search and detail             |
| `victoriametrics` | VictoriaMetrics           | MetricsQL query, targets            |
| `zabbix`          | Zabbix                    | hosts, items, triggers, events      |
| `graylog`         | Graylog                   | streams, searches, alerts           |
| `kibana`          | Kibana/Elastic dashboards | saved objects, dashboards, searches |

## P2: Search Engines

Elasticsearch, OpenSearch, Meilisearch, Typesense, and Solr are implemented as
direct-only search plugins in the core `search` category. Elasticsearch and
OpenSearch share `plugins/shared/escompat`, which is
intentionally scoped to Elasticsearch-compatible REST APIs: indexes, mappings,
settings, aliases, shards, JSON DSL search, and document CRUD. Meilisearch and
Typesense use plugin-specific REST handlers because their APIs model tasks,
keys, settings, collection schemas, synonym sets, and curation sets differently.
Solr uses plugin-specific CoreAdmin, Collections API, Schema API, update, and
select handlers so standalone cores, SolrCloud collections, managed schema
fields, commits, optimizes, and query parameters keep Solr semantics.
Future engines should use their own plugin-specific clients or a separate helper
only where their APIs actually overlap.

| Plugin          | Purpose       | Main Capabilities                                                                   |
| --------------- | ------------- | ----------------------------------------------------------------------------------- |
| `elasticsearch` | Elasticsearch | indexes, documents, JSON DSL search, mappings, health                               |
| `opensearch`    | OpenSearch    | indexes, documents, JSON DSL search, mappings, health                               |
| `meilisearch`   | Meilisearch   | indexes, documents, search, settings, tasks, keys                                   |
| `typesense`     | Typesense     | collections, documents, search, schemas, aliases, synonym sets, curation sets, keys |
| `solr`          | Apache Solr   | cores/collections, documents, search, managed schema fields, config, ping, commit, optimize |

## P3: Messaging And Queues

RabbitMQ, Kafka, and NATS are implemented as direct-only messaging plugins in
the core `messaging` category. They share only small broker helper code for
address parsing, pagination, and config value coercion; protocol manifests,
actions, route handlers, and client/session behavior remain plugin-specific.

| Plugin     | Purpose      | Main Capabilities                                    |
| ---------- | ------------ | ---------------------------------------------------- |
| `rabbitmq` | RabbitMQ     | queues, exchanges, bindings, consumers, messages     |
| `kafka`    | Apache Kafka | clusters, topics, consumer groups, offsets, messages |
| `nats`     | NATS         | streams, consumers, messages, server info            |
| `activemq` | ActiveMQ     | queues, topics, consumers                            |
| `mqtt`     | MQTT brokers | topics, publish/subscribe, retained messages         |

## P3: Identity And Directory

| Plugin      | Purpose        | Main Capabilities                                                              |
| ----------- | -------------- | ------------------------------------------------------------------------------ |
| `ldap`      | LDAP directory | DIT tree, entry attributes (inline edit), add/rename/delete, subtree search ✅ |
| `freeipa`   | FreeIPA        | users, groups, hosts, HBAC, sudo rules                                         |
| `authentik` | Authentik      | users, groups, applications, providers, events                                 |
| `keycloak`  | Keycloak       | realms, clients, users, groups, sessions                                       |
| `zitadel`   | Zitadel        | projects, apps, users, orgs                                                    |

## P3: Backup And Storage Platforms

| Plugin     | Purpose             | Main Capabilities                       |
| ---------- | ------------------- | --------------------------------------- |
| `restic`   | Restic repositories | snapshots, restore, prune status        |
| `borg`     | Borg repositories   | archives, restore, prune status         |
| `kopia`    | Kopia repositories  | snapshots, policies, restore            |
| `velero`   | Kubernetes backup   | backups, restores, schedules            |
| `ceph`     | Ceph cluster        | pools, OSDs, monitors, RBD, CephFS      |
| `zfs`      | ZFS host/storage    | pools, datasets, snapshots, replication |
| `truenas`  | TrueNAS             | pools, datasets, shares, snapshots      |
| `synology` | Synology DSM        | shares, volumes, users, tasks           |

## P3: Generic Protocols And Utilities

| Plugin     | Purpose                 | Main Capabilities                             |
| ---------- | ----------------------- | --------------------------------------------- |
| `http-api` | Generic HTTP API target | requests, saved operations, response viewer   |
| `graphql`  | GraphQL endpoint        | schema introspection, query editor            |
| `grpc`     | gRPC endpoint           | reflection, method calls, streaming           |
| `tcp`      | Generic TCP client      | raw connection, send/receive, diagnostics     |
| `telnet`   | Legacy terminal access  | terminal, terminal recording                  |
| `serial`   | Serial console          | terminal, logs                                |
| `rsync`    | rsync targets           | sync jobs, dry-run, transfer logs             |
| `rclone`   | rclone remotes          | cloud storage browser and transfer operations |

## Suggested Build Order

1. `ssh`
2. `sftp`
3. `docker`
4. `postgresql`
5. `proxmox`
6. `vnc`
7. `mysql`
8. `redis`
9. `mongodb`
10. `mssql`
11. `oracle`
12. `kubernetes`
13. `s3`
14. `webdav`
15. `smb`
16. `nfs`
17. `rdp` as optional sidecar-based plugin

Kubernetes should remain later than SSH/SFTP, Docker, Proxmox, and PostgreSQL
because it exercises the largest surface area: resource trees, watches, logs,
exec, port-forward, YAML editing, RBAC-aware views, events, CRDs, and metrics.
