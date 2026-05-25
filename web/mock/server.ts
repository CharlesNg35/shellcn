import type { Plugin, ViteDevServer, Connect } from "vite";
import type { IncomingMessage, ServerResponse } from "node:http";
import type { Duplex } from "node:stream";
import { readFileSync, readdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { WebSocketServer, type WebSocket } from "ws";
import { listData, badgeData, docData, actionData } from "./datasets.ts";

const fixturesDir = join(
  dirname(fileURLToPath(import.meta.url)),
  "..",
  "fixtures",
);

function readJSON<T>(file: string): T {
  return JSON.parse(readFileSync(join(fixturesDir, file), "utf8")) as T;
}

const nonPluginFixtures = new Set(["connections.json", "credentials.json"]);

interface CredentialKindInfo {
  kind: string;
  label: string;
  secretLabel: string;
  secretMultiline?: boolean;
  identityLabel?: string;
  compatibleProtocols?: string[];
}

interface CredentialField {
  type?: string;
  credential?: {
    kinds?: string[];
    protocols?: string[];
  };
}

interface PluginFixture {
  name: string;
  title: string;
  icon: unknown;
  description: string;
  credentialKinds?: CredentialKindInfo[];
  config?: {
    groups?: Array<{
      fields?: CredentialField[];
    }>;
  };
}

const builtInCredentialKinds: CredentialKindInfo[] = [
  {
    kind: "db_password",
    label: "Database password",
    secretLabel: "Password",
    identityLabel: "Database user",
  },
  {
    kind: "api_token",
    label: "API token",
    secretLabel: "Token",
    identityLabel: "Token name / subject",
  },
  {
    kind: "tls_client_cert",
    label: "TLS client certificate",
    secretLabel: "Certificate and private key",
    secretMultiline: true,
  },
  {
    kind: "cloud_access_key",
    label: "Cloud access key",
    secretLabel: "Secret access key",
    identityLabel: "Access key ID",
  },
  {
    kind: "service_account_json",
    label: "Service account JSON",
    secretLabel: "JSON key",
    secretMultiline: true,
    identityLabel: "Service account",
  },
  {
    kind: "basic_auth",
    label: "Basic auth",
    secretLabel: "Password",
    identityLabel: "Username",
  },
  {
    kind: "bearer_token",
    label: "Bearer token",
    secretLabel: "Token",
    identityLabel: "Token name / subject",
  },
];

function credentialKinds(): CredentialKindInfo[] {
  const byKind = new Map<string, CredentialKindInfo>();
  const supports = new Map<string, Set<string>>();
  const addDefinition = (info: CredentialKindInfo): void => {
    if (!byKind.has(info.kind)) {
      const definition = { ...info };
      delete definition.compatibleProtocols;
      byKind.set(info.kind, definition);
    }
  };
  for (const info of builtInCredentialKinds) addDefinition(info);
  for (const name of pluginNames()) {
    const plugin = readJSON<PluginFixture>(`${name}.json`);
    for (const info of plugin.credentialKinds ?? []) addDefinition(info);
    for (const group of plugin.config?.groups ?? []) {
      for (const field of group.fields ?? []) {
        if (field.type !== "credential_ref" || !field.credential) continue;
        const protocols = field.credential.protocols?.length
          ? field.credential.protocols
          : [plugin.name];
        for (const kind of field.credential.kinds ?? []) {
          if (!supports.has(kind)) supports.set(kind, new Set<string>());
          for (const protocol of protocols) supports.get(kind)?.add(protocol);
        }
      }
    }
  }
  return [...byKind.values()].map((info) => ({
    ...info,
    compatibleProtocols: [...(supports.get(info.kind) ?? [])].sort(),
  }));
}

function credentialKindSupports(kind: string, protocol: string): boolean {
  const info = credentialKinds().find((k) => k.kind === kind);
  return Boolean(info && (info.compatibleProtocols ?? []).includes(protocol));
}

function pluginNames(): string[] {
  return readdirSync(fixturesDir)
    .filter((f) => f.endsWith(".json") && !nonPluginFixtures.has(f))
    .map((f) => f.replace(/\.json$/, ""));
}

function send(res: ServerResponse, status: number, body: unknown): void {
  const json = JSON.stringify(body);
  res.statusCode = status;
  res.setHeader("Content-Type", "application/json");
  res.end(json);
}

function readBody(req: IncomingMessage): Promise<unknown> {
  return new Promise((resolve) => {
    let raw = "";
    req.on("data", (c) => (raw += c));
    req.on("end", () => {
      if (String(req.headers["content-type"] ?? "").includes("multipart/")) {
        const files = [...raw.matchAll(/filename="([^"]+)"/g)].map((m) => m[1]);
        resolve({ files });
        return;
      }
      try {
        resolve(raw ? JSON.parse(raw) : {});
      } catch {
        resolve({});
      }
    });
  });
}

function readRawBody(req: IncomingMessage): Promise<Buffer> {
  return new Promise((resolve) => {
    const chunks: Buffer[] = [];
    req.on("data", (c) => chunks.push(Buffer.from(c)));
    req.on("end", () => resolve(Buffer.concat(chunks)));
  });
}

function encodeCursor(offset: number): string {
  return Buffer.from(String(offset)).toString("base64url");
}

function decodeCursor(cursor: string | null): number {
  if (!cursor) return 0;
  const n = Number(Buffer.from(cursor, "base64url").toString("utf8"));
  return Number.isFinite(n) && n >= 0 ? n : 0;
}

function paramsFrom(url: URL): Record<string, string> {
  const out: Record<string, string> = {};
  for (const [k, v] of url.searchParams) {
    if (k.startsWith("p.")) out[k.slice(2)] = v;
  }
  return out;
}

function paginate(items: Record<string, unknown>[], url: URL) {
  const limit = Math.min(
    Math.max(Number(url.searchParams.get("limit")) || 20, 1),
    100,
  );
  const offset = decodeCursor(url.searchParams.get("cursor"));
  const filter = url.searchParams.get("filter")?.toLowerCase();

  let rows = items;
  if (filter) {
    rows = rows.filter((r) =>
      JSON.stringify(Object.values(r)).toLowerCase().includes(filter),
    );
  }
  const sort = url.searchParams.get("sort");
  if (sort) {
    const desc = sort.startsWith("-");
    const field = desc ? sort.slice(1) : sort;
    rows = [...rows].sort((a, b) => {
      const av = a[field] as never;
      const bv = b[field] as never;
      const cmp = av < bv ? -1 : av > bv ? 1 : 0;
      return desc ? -cmp : cmp;
    });
  }
  const page = rows.slice(offset, offset + limit);
  const next = offset + limit < rows.length ? encodeCursor(offset + limit) : "";
  return { items: page, nextCursor: next, total: rows.length };
}

// In-memory simulation of agent enrollment progress, keyed by connection id.
const agentOnlineAt = new Map<string, number>();

type Json = Record<string, unknown>;

// Mutable in-memory control-plane state so the dev UI can create/edit/share.
let connectionsState: Json[] | null = null;
function connections(): Json[] {
  if (!connectionsState) {
    connectionsState = readJSON<Json[]>("connections.json").map((c) => ({
      ...c,
      canManage: true,
    }));
  }
  return connectionsState;
}

let credentialsState: Json[] | null = null;
function credentials(): Json[] {
  if (!credentialsState)
    credentialsState = readJSON<Json[]>("credentials.json").map((c) => ({
      ...c,
      ownerId: "u-demo",
    }));
  return credentialsState;
}

const grantsState: Record<string, Json[]> = {};
let recordingsState: Json[] | null = null;
const recordingBlobs: Record<string, Buffer[]> = {};
const mockUsers: Json[] = [
  { id: "u-demo", username: "demo", displayName: "Demo User" },
  { id: "u-alice", username: "alice", displayName: "Alice Ng" },
  { id: "u-bob", username: "bob", displayName: "Bob Reyes" },
];

let adminUsersState: Json[] | null = null;
function adminUsers(): Json[] {
  if (!adminUsersState) {
    adminUsersState = [
      {
        id: "u-demo",
        username: "demo",
        email: "demo@example.com",
        displayName: "Demo User",
        roles: ["admin"],
        disabled: false,
        protected: true,
      },
      {
        id: "u-alice",
        username: "alice",
        email: "",
        displayName: "Alice Ng",
        roles: ["operator"],
        disabled: false,
        protected: false,
      },
      {
        id: "u-bob",
        username: "bob",
        email: "",
        displayName: "Bob Reyes",
        roles: ["viewer"],
        disabled: false,
        protected: false,
      },
    ];
  }
  return adminUsersState;
}
let invitationsState: Json[] = [];

function uid(prefix: string): string {
  return `${prefix}-${Math.random().toString(36).slice(2, 8)}`;
}

function resetControlPlaneState(): void {
  connectionsState = null;
  credentialsState = null;
  recordingsState = null;
  adminUsersState = null;
  invitationsState = [];
  for (const key of Object.keys(grantsState)) delete grantsState[key];
  for (const key of Object.keys(recordingBlobs)) delete recordingBlobs[key];
  agentOnlineAt.clear();
}

function recordings(): Json[] {
  if (!recordingsState) {
    const now = Date.now();
    recordingsState = [
      {
        id: "rec-demo-terminal",
        userId: "u-demo",
        username: "demo",
        connectionId: "ssh-prod-web",
        connectionName: "prod-web-01",
        protocol: "ssh",
        class: "terminal",
        format: "asciicast_v2",
        authoritative: true,
        status: "finalized",
        title: "prod-web-01 terminal",
        startedAt: new Date(now - 120_000).toISOString(),
        endedAt: new Date(now - 80_000).toISOString(),
        durationMs: 40_000,
        size: 155,
      },
      {
        id: "rec-demo-desktop",
        userId: "u-demo",
        username: "demo",
        connectionId: "proxmox-lab",
        connectionName: "lab-pve",
        protocol: "proxmox",
        class: "desktop",
        format: "webm_canvas",
        authoritative: false,
        status: "active",
        title: "VM console",
        startedAt: new Date(now - 30_000).toISOString(),
        durationMs: 0,
        size: 0,
      },
    ];
    recordingBlobs["rec-demo-terminal"] = [
      Buffer.from(
        '{"version":2,"width":80,"height":24,"timestamp":1700000000,"title":"prod-web-01 terminal"}\n[0.1,"o","Connected to prod-web-01\\r\\n$ uptime\\r\\n"]\n[1.2,"o"," 15:58 up 12 days\\r\\n$ "]\n',
      ),
    ];
  }
  return recordingsState;
}

function filteredRecordings(url: URL, connectionId = ""): Json[] {
  return recordings().filter((r) => {
    if (connectionId && r.connectionId !== connectionId) return false;
    for (const key of ["user", "protocol", "class", "status"] as const) {
      const v = url.searchParams.get(key);
      const field = key === "user" ? "userId" : key;
      if (v && r[field] !== v) return false;
    }
    const conn = url.searchParams.get("connection");
    return !(conn && r.connectionId !== conn);
  });
}

function pluginIcon(protocol: string): unknown {
  try {
    return readJSON<{ icon: unknown }>(`${protocol}.json`).icon;
  } catch {
    return { type: "name", value: "box" };
  }
}

function agentStatus(connectionId: string): {
  status: string;
  message?: string;
} {
  const at = agentOnlineAt.get(connectionId);
  if (at === undefined)
    return {
      status: "pending",
      message: "Waiting for the agent to dial back.",
    };
  if (Date.now() >= at) return { status: "online" };
  return { status: "pending", message: "Agent enrolling…" };
}

function handleHTTP(
  req: IncomingMessage,
  res: ServerResponse,
  next: Connect.NextFunction,
): void {
  const url = new URL(req.url ?? "/", "http://localhost");
  const path = url.pathname;
  if (!path.startsWith("/api/")) return next();

  const method = req.method ?? "GET";

  if (path === "/api/__test/reset" && method === "POST") {
    resetControlPlaneState();
    return send(res, 200, { ok: true });
  }

  // Auth: the mock is always "signed in" as an admin so the dev/e2e UX stays
  // open. The real backend gates this behind a session cookie + CSRF token.
  const mockSession = () => ({
    user: {
      id: "u-demo",
      username: "demo",
      displayName: "Demo User",
      email: "demo@example.com",
      roles: ["admin"],
    },
    csrfToken: "mock-csrf-token",
  });
  if (path === "/api/auth/me" && method === "GET") {
    return send(res, 200, mockSession());
  }
  if (path === "/api/auth/login" && method === "POST") {
    return send(res, 200, mockSession());
  }
  if (path === "/api/auth/logout" && method === "POST") {
    return send(res, 200, { ok: true });
  }

  if (path === "/api/plugins" && method === "GET") {
    const summaries = pluginNames().map((name) => {
      const p = readJSON<{
        name: string;
        title: string;
        icon: unknown;
        description: string;
      }>(`${name}.json`);
      return {
        name: p.name,
        title: p.title,
        icon: p.icon,
        description: p.description,
      };
    });
    return send(res, 200, summaries);
  }

  if (path === "/api/credential-kinds" && method === "GET") {
    return send(res, 200, credentialKinds());
  }

  const pluginMatch = path.match(/^\/api\/plugins\/([^/]+)$/);
  if (pluginMatch && method === "GET") {
    if (!pluginNames().includes(pluginMatch[1]))
      return send(res, 404, { error: "unknown plugin" });
    return send(res, 200, readJSON(`${pluginMatch[1]}.json`));
  }

  if (path === "/api/users" && method === "GET") {
    const q = (url.searchParams.get("query") ?? "").toLowerCase();
    return send(
      res,
      200,
      mockUsers.filter(
        (u) =>
          !q ||
          String(u.username).toLowerCase().includes(q) ||
          String(u.displayName ?? "")
            .toLowerCase()
            .includes(q),
      ),
    );
  }

  // --- admin: users, invitations, email status ---
  if (path === "/api/admin/email" && method === "GET") {
    return send(res, 200, { enabled: false });
  }
  if (path === "/api/admin/users" && method === "GET") {
    return send(res, 200, adminUsers());
  }
  if (path === "/api/admin/users" && method === "POST") {
    return void readBody(req).then((raw) => {
      const body = raw as Json;
      const u: Json = {
        id: uid("u"),
        username: body.username,
        email: body.email ?? "",
        displayName: body.displayName ?? "",
        roles: [body.role],
        disabled: false,
        protected: false,
      };
      adminUsers().push(u);
      send(res, 201, u);
    });
  }
  const adminUserMatch = path.match(/^\/api\/admin\/users\/([^/]+)$/);
  if (adminUserMatch) {
    const id = adminUserMatch[1];
    const u = adminUsers().find((x) => x.id === id);
    if (!u) return send(res, 404, { error: "unknown user" });
    if (method === "PUT") {
      return void readBody(req).then((raw) => {
        const body = raw as Json;
        u.email = body.email ?? u.email;
        u.displayName = body.displayName ?? u.displayName;
        u.roles = [body.role];
        u.disabled = Boolean(body.disabled);
        send(res, 200, u);
      });
    }
    if (method === "DELETE") {
      adminUsersState = adminUsers().filter((x) => x.id !== id);
      return send(res, 200, { ok: true });
    }
  }

  if (path === "/api/admin/invitations" && method === "GET") {
    return send(res, 200, invitationsState);
  }
  if (path === "/api/admin/invitations" && method === "POST") {
    return void readBody(req).then((raw) => {
      const body = raw as Json;
      const token = uid("invtok");
      const inv: Json = {
        id: uid("inv"),
        email: body.email,
        role: body.role,
        status: "pending",
        createdAt: new Date().toISOString(),
        expiresAt: new Date(Date.now() + 72 * 3600_000).toISOString(),
        _token: token,
      };
      invitationsState.push(inv);
      const link = `http://${req.headers.host ?? "localhost"}/invite/${token}`;
      send(res, 201, { invitation: inv, link, emailSent: false });
    });
  }
  const inviteRevokeMatch = path.match(/^\/api\/admin\/invitations\/([^/]+)$/);
  if (inviteRevokeMatch && method === "DELETE") {
    const inv = invitationsState.find((i) => i.id === inviteRevokeMatch[1]);
    if (inv) inv.status = "revoked";
    return send(res, 200, { ok: true });
  }

  // --- public: invitation accept ---
  const acceptMatch = path.match(/^\/api\/invitations\/([^/]+)\/accept$/);
  if (acceptMatch && method === "POST") {
    const inv = invitationsState.find(
      (i) => i._token === acceptMatch[1] && i.status === "pending",
    );
    if (!inv) return send(res, 404, { error: "invalid invitation" });
    return void readBody(req).then((raw) => {
      const body = raw as Json;
      inv.status = "accepted";
      adminUsers().push({
        id: uid("u"),
        username: body.username,
        email: inv.email,
        displayName: "",
        roles: [inv.role],
        disabled: false,
        protected: false,
      });
      send(res, 201, { username: body.username });
    });
  }
  const inviteLookupMatch = path.match(/^\/api\/invitations\/([^/]+)$/);
  if (inviteLookupMatch && method === "GET") {
    const inv = invitationsState.find(
      (i) => i._token === inviteLookupMatch[1] && i.status === "pending",
    );
    if (!inv) return send(res, 404, { error: "invalid invitation" });
    return send(res, 200, { email: inv.email, role: inv.role });
  }

  if (path === "/api/connections" && method === "GET") {
    return send(res, 200, connections());
  }
  if (path === "/api/connections" && method === "POST") {
    return void readBody(req).then((raw) => {
      const body = raw as Json;
      const protocol = String(body.protocol ?? "");
      const transport = String(body.transport ?? "direct");
      const conn: Json = {
        id: uid("conn"),
        name: body.name,
        protocol,
        transport,
        icon: pluginIcon(protocol),
        online: transport !== "agent",
        status: transport === "agent" ? "pending" : undefined,
        canManage: true,
        recording: body.recording ?? {},
      };
      connections().push(conn);
      send(res, 201, conn);
    });
  }

  const connDetailMatch = path.match(/^\/api\/connections\/([^/]+)$/);
  if (connDetailMatch) {
    const id = connDetailMatch[1];
    const conn = connections().find((c) => c.id === id);
    if (!conn) return send(res, 404, { error: "unknown connection" });
    if (method === "GET") {
      return send(res, 200, {
        id,
        name: conn.name,
        protocol: conn.protocol,
        transport: conn.transport,
        ownerId: "u-demo",
        config: (conn.config as Json) ?? {},
        secrets: {},
        recording: (conn.recording as Json) ?? {},
      });
    }
    if (method === "PUT") {
      return void readBody(req).then((raw) => {
        const body = raw as Json;
        conn.name = body.name ?? conn.name;
        conn.transport = body.transport ?? conn.transport;
        conn.config = body.config ?? {};
        conn.recording = body.recording ?? conn.recording ?? {};
        send(res, 200, {
          id,
          name: conn.name,
          protocol: conn.protocol,
          transport: conn.transport,
          ownerId: "u-demo",
          config: conn.config,
          secrets: {},
          recording: conn.recording,
        });
      });
    }
    if (method === "DELETE") {
      connectionsState = connections().filter((c) => c.id !== id);
      return send(res, 200, { ok: true });
    }
  }

  const grantsMatch = path.match(
    /^\/api\/(connections|credentials)\/([^/]+)\/grants$/,
  );
  if (grantsMatch) {
    const key = `${grantsMatch[1]}:${grantsMatch[2]}`;
    grantsState[key] ??= [];
    if (method === "GET") return send(res, 200, grantsState[key]);
    if (method === "POST") {
      return void readBody(req).then((raw) => {
        const body = raw as Json;
        const user = mockUsers.find((u) => u.id === body.subjectId);
        const grant: Json = {
          id: uid("grant"),
          subjectId: body.subjectId,
          username: user?.username,
          displayName: user?.displayName,
          access: body.access ?? "use",
        };
        grantsState[key].push(grant);
        send(res, 201, grant);
      });
    }
  }
  const grantDeleteMatch = path.match(
    /^\/api\/(connections|credentials)\/([^/]+)\/grants\/([^/]+)$/,
  );
  if (grantDeleteMatch && method === "DELETE") {
    const key = `${grantDeleteMatch[1]}:${grantDeleteMatch[2]}`;
    grantsState[key] = (grantsState[key] ?? []).filter(
      (g) => g.id !== grantDeleteMatch[3],
    );
    return send(res, 200, { ok: true });
  }

  if (path === "/api/recordings" && method === "GET") {
    return send(res, 200, filteredRecordings(url));
  }
  const connectionRecordingsMatch = path.match(
    /^\/api\/connections\/([^/]+)\/recordings$/,
  );
  if (connectionRecordingsMatch && method === "GET") {
    return send(
      res,
      200,
      filteredRecordings(url, connectionRecordingsMatch[1]),
    );
  }
  const desktopRecordingMatch = path.match(
    /^\/api\/connections\/([^/]+)\/recordings\/desktop$/,
  );
  if (desktopRecordingMatch && method === "POST") {
    return void readBody(req).then((raw) => {
      const body = raw as Json;
      const conn = connections().find((c) => c.id === desktopRecordingMatch[1]);
      const rec: Json = {
        id: uid("rec"),
        userId: "u-demo",
        username: "demo",
        connectionId: desktopRecordingMatch[1],
        connectionName: conn?.name ?? desktopRecordingMatch[1],
        protocol: conn?.protocol ?? "unknown",
        class: "desktop",
        format: body.format ?? "webm_canvas",
        authoritative: false,
        status: "active",
        startedAt: new Date().toISOString(),
        durationMs: 0,
        size: 0,
      };
      recordings().unshift(rec);
      recordingBlobs[String(rec.id)] = [];
      send(res, 201, rec);
    });
  }
  const recordingMatch = path.match(/^\/api\/recordings\/([^/]+)$/);
  if (recordingMatch) {
    const id = recordingMatch[1];
    const rec = recordings().find((r) => r.id === id);
    if (!rec) return send(res, 404, { error: "unknown recording" });
    if (method === "GET") return send(res, 200, rec);
    if (method === "DELETE") {
      recordingsState = recordings().filter((r) => r.id !== id);
      delete recordingBlobs[id];
      return send(res, 200, { ok: true });
    }
  }
  const recordingContentMatch = path.match(
    /^\/api\/recordings\/([^/]+)\/content$/,
  );
  if (recordingContentMatch && method === "GET") {
    const id = recordingContentMatch[1];
    const rec = recordings().find((r) => r.id === id);
    if (!rec) return send(res, 404, { error: "unknown recording" });
    const body = Buffer.concat(recordingBlobs[id] ?? []);
    res.statusCode = 200;
    res.setHeader(
      "Content-Type",
      rec.format === "webm_canvas" ? "video/webm" : "application/x-asciicast",
    );
    res.end(body);
    return;
  }
  const recordingChunkMatch = path.match(
    /^\/api\/recordings\/([^/]+)\/chunks$/,
  );
  if (recordingChunkMatch && method === "POST") {
    const id = recordingChunkMatch[1];
    if (!recordings().some((r) => r.id === id))
      return send(res, 404, { error: "unknown recording" });
    return void readRawBody(req).then((body) => {
      recordingBlobs[id] ??= [];
      recordingBlobs[id].push(body);
      send(res, 200, {
        ok: true,
        index: Number(url.searchParams.get("index")),
      });
    });
  }
  const recordingFinalizeMatch = path.match(
    /^\/api\/recordings\/([^/]+)\/finalize$/,
  );
  if (recordingFinalizeMatch && method === "POST") {
    const id = recordingFinalizeMatch[1];
    const rec = recordings().find((r) => r.id === id);
    if (!rec) return send(res, 404, { error: "unknown recording" });
    rec.status = "finalized";
    rec.endedAt = new Date().toISOString();
    rec.size = Buffer.concat(recordingBlobs[id] ?? []).length;
    send(res, 200, rec);
    return;
  }
  const recordingAbortMatch = path.match(/^\/api\/recordings\/([^/]+)\/abort$/);
  if (recordingAbortMatch && method === "POST") {
    const id = recordingAbortMatch[1];
    const rec = recordings().find((r) => r.id === id);
    if (!rec) return send(res, 404, { error: "unknown recording" });
    rec.status = "discarded";
    delete recordingBlobs[id];
    return send(res, 200, { ok: true });
  }

  if (path === "/api/credentials" && method === "GET") {
    const kinds = url.searchParams.get("kind")?.split(",").filter(Boolean);
    const protocol = url.searchParams.get("protocol");
    const filtered = credentials().filter((c) => {
      const protocols = c.protocols as string[] | undefined;
      if (kinds && kinds.length > 0 && !kinds.includes(String(c.kind)))
        return false;
      if (protocol && !credentialKindSupports(String(c.kind), protocol))
        return false;
      if (protocol && protocols && !protocols.includes(protocol)) return false;
      return true;
    });
    return send(res, 200, filtered);
  }
  if (path === "/api/credentials" && method === "POST") {
    return void readBody(req).then((raw) => {
      const body = raw as Json;
      const cred: Json = {
        id: uid("cred"),
        name: body.name,
        kind: body.kind,
        ownerId: "u-demo",
        identity: body.identity ?? body.username,
        protocols: body.protocols,
        updatedAt: new Date().toISOString(),
      };
      credentials().push(cred);
      send(res, 201, cred);
    });
  }
  const credDetailMatch = path.match(/^\/api\/credentials\/([^/]+)$/);
  if (credDetailMatch) {
    const id = credDetailMatch[1];
    const cred = credentials().find((c) => c.id === id);
    if (!cred) return send(res, 404, { error: "unknown credential" });
    if (method === "PUT") {
      return void readBody(req).then((raw) => {
        const body = raw as Json;
        cred.name = body.name ?? cred.name;
        cred.kind = body.kind ?? cred.kind;
        cred.identity = body.identity ?? body.username;
        cred.protocols = body.protocols;
        cred.updatedAt = new Date().toISOString();
        send(res, 200, cred);
      });
    }
    if (method === "DELETE") {
      credentialsState = credentials().filter((c) => c.id !== id);
      return send(res, 200, { ok: true });
    }
  }

  const agentStateMatch = path.match(
    /^\/api\/connections\/([^/]+)\/agent\/state$/,
  );
  if (agentStateMatch && method === "GET") {
    return send(res, 200, agentStatus(agentStateMatch[1]));
  }

  const enrollMatch = path.match(
    /^\/api\/connections\/([^/]+)\/agent\/enrollments$/,
  );
  if (enrollMatch && method === "POST") {
    const id = enrollMatch[1];
    agentOnlineAt.set(id, Date.now() + 6000);
    return send(res, 201, {
      enrollmentId: `enr-${Date.now()}`,
      expiresAt: new Date(Date.now() + 15 * 60_000).toISOString(),
      artifacts: [
        {
          label: "Docker",
          kind: "docker-run",
          command:
            "docker run -d --restart unless-stopped -v /var/run/docker.sock:/var/run/docker.sock shellcn-proxy:latest -e SHELLCN_CONNECT_URL=wss://localhost/api/agent/connect -e SHELLCN_ENROLL_TOKEN=mock-token",
        },
        {
          label: "Shell",
          kind: "shell",
          command:
            "SHELLCN_ENROLL_TOKEN=mock-token shellcn-agent --connect wss://localhost/api/agent/connect",
        },
      ],
    });
  }

  const ticketMatch = path.match(/^\/api\/connections\/([^/]+)\/tickets$/);
  if (ticketMatch && method === "POST") {
    return send(res, 201, {
      ticket: `mock-ticket-${Date.now()}`,
      expiresAt: new Date(Date.now() + 30_000).toISOString(),
    });
  }

  const routeMatch = path.match(/^\/api\/connections\/([^/]+)\/x\/([^/]+)$/);
  if (routeMatch) {
    const routeId = routeMatch[2];
    const params = paramsFrom(url);

    if (method === "GET") {
      const badge = badgeData(routeId);
      if (badge) return send(res, 200, badge);
      const doc = docData(routeId, params);
      if (doc !== undefined) return send(res, 200, doc);
      const data = listData(routeId, params);
      if (data) return send(res, 200, paginate(data, url));
      return send(res, 200, { items: [], nextCursor: "", total: 0 });
    }
    void readBody(req).then((body) => {
      const action = actionData(routeId, params, body);
      send(res, 200, action ?? { ok: true, routeId });
    });
    return;
  }

  return send(res, 404, { error: "not found" });
}

function streamKind(
  routeId: string,
): "terminal" | "logs" | "metrics" | "watch" | "query" | "desktop" {
  if (/(exec|shell)$/.test(routeId)) return "terminal";
  if (/logs$/.test(routeId)) return "logs";
  if (/(stats|metrics)$/.test(routeId)) return "metrics";
  if (/watch$/.test(routeId)) return "watch";
  if (/query$/.test(routeId)) return "query";
  return "desktop";
}

function driveSocket(ws: WebSocket, routeId: string): void {
  const kind = streamKind(routeId);
  const timers: NodeJS.Timeout[] = [];

  if (kind === "terminal") {
    ws.send("Connected to mock shell. Type and press enter.\r\n$ ");
    ws.on("message", (data) => {
      const text = data.toString();
      ws.send(text.replace(/\r?\n/g, "\r\n"));
      if (/\r?\n/.test(text)) ws.send("$ ");
    });
  } else if (kind === "logs") {
    let n = 0;
    timers.push(
      setInterval(() => {
        n += 1;
        ws.send(
          JSON.stringify({
            ts: new Date().toISOString(),
            line: `[mock] log line ${n}`,
          }),
        );
      }, 700),
    );
  } else if (kind === "metrics") {
    timers.push(
      setInterval(() => {
        ws.send(
          JSON.stringify({
            ts: Date.now(),
            cpu: Math.round(Math.random() * 100),
            mem: Math.round(Math.random() * 100),
          }),
        );
      }, 1000),
    );
  } else if (kind === "watch") {
    timers.push(
      setInterval(() => {
        ws.send(
          JSON.stringify({
            type: "updated",
            ref: { kind: "container", name: "api-2", uid: "c-2" },
            resource: { state: Math.random() > 0.5 ? "running" : "exited" },
          }),
        );
      }, 3000),
    );
  } else if (kind === "query") {
    ws.on("message", (data) => {
      let sql: string;
      try {
        sql = (JSON.parse(data.toString()) as { query?: string }).query ?? "";
      } catch {
        sql = data.toString();
      }
      ws.send(
        JSON.stringify({
          query: sql,
          columns: ["id", "name", "created_at"],
          rows: [
            [1, "alice", "2026-05-01T10:00:00Z"],
            [2, "bob", "2026-05-02T11:30:00Z"],
          ],
          rowCount: 2,
          elapsedMs: 12,
        }),
      );
    });
  } else {
    ws.send(
      JSON.stringify({
        note: "mock desktop stream — validated with the real plugin",
      }),
    );
  }

  ws.on("close", () => timers.forEach(clearInterval));
}

export function mockApiPlugin(): Plugin {
  const wss = new WebSocketServer({ noServer: true });

  return {
    name: "shellcn-mock-api",
    configureServer(server: ViteDevServer) {
      server.middlewares.use((req, res, next) =>
        handleHTTP(req, res, next as Connect.NextFunction),
      );

      server.httpServer?.on(
        "upgrade",
        (req: IncomingMessage, socket: Duplex, head: Buffer) => {
          const url = new URL(req.url ?? "/", "http://localhost");
          const match = url.pathname.match(
            /^\/api\/connections\/[^/]+\/x\/([^/]+)$/,
          );
          if (!match) return; // let Vite's HMR socket handle its own upgrades
          wss.handleUpgrade(req, socket, head, (ws) =>
            driveSocket(ws, match[1]),
          );
        },
      );
    },
  };
}
