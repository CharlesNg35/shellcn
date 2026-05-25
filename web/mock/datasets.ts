// Canned datasets for the fixture-backed mock API. Items are deliberately
// shaped to satisfy both a table Row (column fields + ref) and a TreeNode
// (key/label/leaf/childrenSource), so one dataset feeds both renderers.

type Item = Record<string, unknown>;

function range(n: number): number[] {
  return Array.from({ length: n }, (_, i) => i);
}

const containerStates = ["running", "running", "running", "exited", "paused"];

const containers: Item[] = range(25).map((i) => {
  const name =
    [
      "nginx",
      "api",
      "worker",
      "redis",
      "postgres",
      "grafana",
      "minio",
      "traefik",
    ][i % 8] +
    "-" +
    (i + 1);
  const state = containerStates[i % containerStates.length];
  return {
    key: `c-${i + 1}`,
    label: name,
    leaf: true,
    ref: { kind: "container", name, uid: `c-${i + 1}` },
    name,
    image: [
      "nginx:1.27",
      "node:22-alpine",
      "redis:7",
      "postgres:16",
      "grafana/grafana:11",
    ][i % 5],
    state,
    status: state === "running" ? `Up ${i + 1} hours` : "Exited (0) 2 days ago",
    ports: i % 3 === 0 ? "0.0.0.0:8080->80/tcp" : "",
  };
});

const images: Item[] = range(12).map((i) => ({
  key: `img-${i + 1}`,
  label:
    ["nginx", "node", "redis", "postgres"][i % 4] + `:${[1, 16, 22, 7][i % 4]}`,
  leaf: true,
  ref: {
    kind: "image",
    name: `img-${i + 1}`,
    uid: `sha256:${(i + 1).toString(16).padStart(12, "0")}`,
  },
  repository: ["nginx", "node", "redis", "postgres"][i % 4],
  tag: `${[1, 16, 22, 7][i % 4]}`,
  size: (40 + i * 17) * 1024 * 1024,
}));

const volumes: Item[] = range(6).map((i) => ({
  key: `vol-${i + 1}`,
  label: `vol_${i + 1}`,
  leaf: true,
  ref: { kind: "volume", name: `vol_${i + 1}`, uid: `vol-${i + 1}` },
  name: `vol_${i + 1}`,
  driver: "local",
  mountpoint: `/var/lib/docker/volumes/vol_${i + 1}/_data`,
}));

const networks: Item[] = range(5).map((i) => ({
  key: `net-${i + 1}`,
  label: ["bridge", "host", "none", "app-net", "db-net"][i],
  leaf: true,
  ref: {
    kind: "network",
    name: ["bridge", "host", "none", "app-net", "db-net"][i],
    uid: `net-${i + 1}`,
  },
  name: ["bridge", "host", "none", "app-net", "db-net"][i],
  driver: ["bridge", "host", "null", "bridge", "bridge"][i],
  scope: "local",
}));

const pveNodes: Item[] = ["pve1", "pve2"].map((node, i) => ({
  key: node,
  label: node,
  leaf: false,
  childrenSource: { routeId: "proxmox.vm.list", params: { node } },
  ref: { kind: "node", name: node, uid: node },
  status: i === 0 ? "online" : "online",
}));

function vmsForNode(node: string): Item[] {
  return range(node === "pve1" ? 4 : 3).map((i) => {
    const vmid = (node === "pve1" ? 100 : 200) + i + 1;
    const name = `${node}-vm-${i + 1}`;
    const status = i % 3 === 0 ? "stopped" : "running";
    return {
      key: `${node}-${vmid}`,
      label: `${vmid} (${name})`,
      leaf: true,
      ref: { kind: "vm", namespace: node, name, uid: String(vmid) },
      vmid,
      name,
      status,
      cpu: status === "running" ? (i + 1) * 12 : 0,
      mem: (1 + i) * 1024 * 1024 * 1024,
    };
  });
}

const pveStorage: Item[] = ["local", "local-lvm", "ceph-pool"].map(
  (name, i) => ({
    key: name,
    label: name,
    leaf: true,
    ref: { kind: "storage", name, uid: name },
    name,
    type: ["dir", "lvmthin", "rbd"][i],
    used: (i + 1) * 50 * 1024 * 1024 * 1024,
    total: 500 * 1024 * 1024 * 1024,
  }),
);

const pgDatabases: Item[] = ["app", "analytics"].map((db) => ({
  key: `db-${db}`,
  label: db,
  leaf: false,
  ref: { kind: "database", name: db, uid: db },
  childrenSource: { routeId: "postgres.folder.list", params: { db } },
}));

function pgFolders(db: string): Item[] {
  return [
    {
      key: `${db}-tables`,
      label: "Tables",
      leaf: false,
      childrenSource: {
        routeId: "postgres.table.list",
        params: { db, schema: "public" },
      },
    },
    {
      key: `${db}-views`,
      label: "Views",
      leaf: false,
      childrenSource: {
        routeId: "postgres.view.list",
        params: { db, schema: "public" },
      },
    },
    {
      key: `${db}-functions`,
      label: "Functions",
      leaf: true,
      childrenSource: {
        routeId: "postgres.function.list",
        params: { db, schema: "public" },
      },
    },
  ];
}

function pgTables(schema: string): Item[] {
  return ["users", "orders", "products", "sessions", "audit_log"].map(
    (name, i) => ({
      key: `${schema}.${name}`,
      label: name,
      leaf: true,
      ref: { kind: "table", namespace: schema, name, uid: `${schema}.${name}` },
      name,
      schema,
      rows: (i + 1) * 1423,
      size: (i + 1) * 32 * 1024,
    }),
  );
}

const sshTunnels: Item[] = [
  {
    key: "t1",
    ref: { kind: "tunnel", name: "pg", uid: "t1" },
    name: "pg",
    type: "local",
    listen: "127.0.0.1:5432",
    target: "db.internal:5432",
    status: "active",
  },
  {
    key: "t2",
    ref: { kind: "tunnel", name: "redis", uid: "t2" },
    name: "redis",
    type: "local",
    listen: "127.0.0.1:6379",
    target: "cache.internal:6379",
    status: "active",
  },
];

const sshSnippets: Item[] = [
  {
    key: "s1",
    ref: { kind: "snippet", name: "disk usage", uid: "s1" },
    name: "disk usage",
    command: "df -h",
  },
  {
    key: "s2",
    ref: { kind: "snippet", name: "tail syslog", uid: "s2" },
    name: "tail syslog",
    command: "tail -f /var/log/syslog",
  },
  {
    key: "s3",
    ref: { kind: "snippet", name: "top procs", uid: "s3" },
    name: "top procs",
    command: "ps aux --sort=-%cpu | head",
  },
];

function envRows(id: string): Item[] {
  return [
    {
      key: `${id}-1`,
      name: "PATH",
      value: "/usr/local/sbin:/usr/local/bin:/usr/bin",
    },
    { key: `${id}-2`, name: "NODE_ENV", value: "production" },
    { key: `${id}-3`, name: "PORT", value: "8080" },
  ];
}

function tableColumns(table: string): Item[] {
  return [
    {
      key: `${table}-id`,
      name: "id",
      dataType: "bigint",
      nullable: false,
      default: "nextval(...)",
    },
    { key: `${table}-name`, name: "name", dataType: "text", nullable: false },
    {
      key: `${table}-created_at`,
      name: "created_at",
      dataType: "timestamptz",
      nullable: false,
      default: "now()",
    },
  ];
}

function tableIndexes(table: string): Item[] {
  return [
    {
      key: `${table}-pk`,
      name: `${table}_pkey`,
      definition: `PRIMARY KEY (id)`,
      unique: true,
    },
    {
      key: `${table}-name-idx`,
      name: `${table}_name_idx`,
      definition: `btree (name)`,
      unique: false,
    },
  ];
}

interface FsNode {
  entries: { name: string; isDir: boolean; size?: number; mime?: string }[];
}

const fs: Record<string, FsNode> = {
  "/": {
    entries: [
      { name: "etc", isDir: true },
      { name: "home", isDir: true },
      { name: "var", isDir: true },
      { name: "README.md", isDir: false, size: 412, mime: "text/markdown" },
      { name: "diagram.svg", isDir: false, size: 980, mime: "image/svg+xml" },
      { name: "app.log", isDir: false, size: 20480, mime: "text/plain" },
      {
        name: "backup.tar.gz",
        isDir: false,
        size: 10485760,
        mime: "application/gzip",
      },
    ],
  },
  "/etc": {
    entries: [
      { name: "nginx.conf", isDir: false, size: 1024, mime: "text/plain" },
      { name: "hosts", isDir: false, size: 220, mime: "text/plain" },
    ],
  },
  "/home": { entries: [{ name: "deploy", isDir: true }] },
  "/home/deploy": {
    entries: [
      { name: "app.json", isDir: false, size: 156, mime: "application/json" },
      { name: ".bashrc", isDir: false, size: 312, mime: "text/plain" },
    ],
  },
  "/var": { entries: [{ name: "log", isDir: true }] },
  "/var/log": {
    entries: [
      { name: "syslog", isDir: false, size: 81920, mime: "text/plain" },
    ],
  },
};

function fsList(path: string): Item[] {
  const node = fs[path] ?? { entries: [] };
  return node.entries.map((e) => ({
    name: e.name,
    path: path === "/" ? `/${e.name}` : `${path}/${e.name}`,
    isDir: e.isDir,
    size: e.size,
    mime: e.mime,
    modTime: "2026-05-20T08:00:00Z",
    mode: e.isDir ? "drwxr-xr-x" : "rw-r--r--",
  }));
}

const fileContents: Record<string, string> = {
  "/README.md":
    "# Project\n\nA sample file served by the mock SFTP backend.\n\n- item one\n- item two\n",
  "/app.log": Array.from(
    { length: 40 },
    (_, i) =>
      `2026-05-20T08:0${i % 10}:00Z app: request ${i + 1} served in ${10 + i}ms`,
  ).join("\n"),
  "/etc/nginx.conf":
    "server {\n  listen 80;\n  server_name _;\n  location / { proxy_pass http://127.0.0.1:8080; }\n}\n",
  "/etc/hosts": "127.0.0.1 localhost\n10.0.0.1 app.internal\n",
  "/home/deploy/app.json":
    '{\n  "name": "app",\n  "port": 8080,\n  "env": "production"\n}\n',
  "/home/deploy/.bashrc":
    "export PATH=$PATH:/usr/local/bin\nalias ll='ls -la'\n",
  "/var/log/syslog":
    "May 20 08:00:00 host systemd[1]: Started Daily apt activities.\n",
};

const svgBase64 = Buffer.from(
  '<svg xmlns="http://www.w3.org/2000/svg" width="240" height="120"><rect width="240" height="120" fill="#1e293b"/><text x="20" y="65" fill="#38bdf8" font-family="sans-serif" font-size="20">ShellCN mock</text></svg>',
).toString("base64");

function fsRead(path: string): unknown {
  if (path === "/diagram.svg") {
    return {
      path,
      mime: "image/svg+xml",
      encoding: "base64",
      content: svgBase64,
      size: 980,
    };
  }
  if (path === "/backup.tar.gz") {
    return { path, mime: "application/gzip", encoding: "url", size: 10485760 };
  }
  const content = fileContents[path];
  if (content !== undefined) {
    return {
      path,
      mime: "text/plain",
      encoding: "utf8",
      content,
      size: content.length,
    };
  }
  return { path, encoding: "utf8", content: "" };
}

const lists: Record<string, (params: Record<string, string>) => Item[]> = {
  "ssh.sftp.list": (p) => fsList(p.path ?? "/"),
  "docker.container.list": () => containers,
  "docker.image.list": () => images,
  "docker.volume.list": () => volumes,
  "docker.network.list": () => networks,
  "docker.container.env": (p) => envRows(p.id ?? "c-1"),
  "proxmox.node.list": () => pveNodes,
  "proxmox.vm.list": (p) =>
    p.node
      ? vmsForNode(p.node)
      : [...vmsForNode("pve1"), ...vmsForNode("pve2")],
  "proxmox.storage.list": () => pveStorage,
  "proxmox.network.list": () => [
    {
      key: "vmbr0",
      label: "vmbr0",
      leaf: true,
      ref: { kind: "network", name: "vmbr0", uid: "vmbr0" },
      name: "vmbr0",
      type: "bridge",
      active: true,
    },
  ],
  "proxmox.datacenter.list": () => [
    {
      key: "ha",
      label: "HA",
      leaf: true,
      ref: { kind: "dc", name: "ha", uid: "ha" },
      name: "HA groups",
      value: "2 groups",
    },
  ],
  "proxmox.vm.snapshots.list": (p) => [
    {
      key: `${p.vmid}-snap1`,
      ref: { kind: "snapshot", name: "pre-upgrade", uid: "snap1" },
      name: "pre-upgrade",
      time: "2026-05-20T10:00:00Z",
      description: "before kernel upgrade",
    },
  ],
  "proxmox.vm.backups.list": (p) => [
    {
      key: `${p.vmid}-bk1`,
      ref: { kind: "backup", name: "vzdump-2026-05-24", uid: "bk1" },
      name: "vzdump-2026-05-24",
      size: 4 * 1024 * 1024 * 1024,
      storage: "local",
    },
  ],
  "postgres.database.list": () => pgDatabases,
  "postgres.folder.list": (p) => pgFolders(p.db ?? "app"),
  "postgres.table.list": (p) => pgTables(p.schema ?? "public"),
  "postgres.view.list": (p) => [
    {
      key: `${p.schema ?? "public"}.active_users`,
      label: "active_users",
      leaf: true,
      ref: {
        kind: "view",
        namespace: p.schema ?? "public",
        name: "active_users",
        uid: "active_users",
      },
      name: "active_users",
      schema: p.schema ?? "public",
    },
  ],
  "postgres.function.list": (p) => [
    {
      key: `${p.schema ?? "public"}.fn_touch`,
      label: "fn_touch()",
      leaf: true,
      ref: {
        kind: "function",
        namespace: p.schema ?? "public",
        name: "fn_touch",
        uid: "fn_touch",
      },
      name: "fn_touch",
      returns: "trigger",
    },
  ],
  "postgres.table.columns": (p) => tableColumns(p.table ?? "users"),
  "postgres.table.indexes": (p) => tableIndexes(p.table ?? "users"),
  "ssh.tunnel.list": () => sshTunnels,
  "ssh.snippet.list": () => sshSnippets,
};

export function listData(
  routeId: string,
  params: Record<string, string>,
): Item[] | undefined {
  const fn = lists[routeId];
  return fn ? fn(params) : undefined;
}

export function badgeData(routeId: string): { value: number } | undefined {
  if (routeId === "docker.container.count") {
    return { value: containers.filter((c) => c.state === "running").length };
  }
  return undefined;
}

export function docData(
  routeId: string,
  params: Record<string, string>,
): unknown {
  if (routeId === "ssh.sftp.read") {
    return fsRead(params.path ?? "/");
  }
  if (routeId === "docker.container.inspect") {
    const c =
      containers.find(
        (x) => x.ref && (x.ref as { uid: string }).uid === params.id,
      ) ?? containers[0];
    return {
      Id: (c.ref as { uid: string }).uid,
      Name: c.name,
      State: { Status: c.state },
      Config: { Image: c.image, Env: ["NODE_ENV=production"] },
    };
  }
  if (routeId === "proxmox.vm.config") {
    return {
      groups: [
        {
          name: "Hardware",
          fields: [
            { key: "cores", label: "Cores", type: "number", default: 4 },
            {
              key: "memory",
              label: "Memory (MB)",
              type: "number",
              default: 8192,
            },
            {
              key: "boot",
              label: "Boot order",
              type: "text",
              default: "order=scsi0;net0",
            },
          ],
        },
      ],
    };
  }
  return undefined;
}
