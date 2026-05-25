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
const mockUsers: Json[] = [
  { id: "u-demo", username: "demo", displayName: "Demo User" },
  { id: "u-alice", username: "alice", displayName: "Alice Ng" },
  { id: "u-bob", username: "bob", displayName: "Bob Reyes" },
];

function uid(prefix: string): string {
  return `${prefix}-${Math.random().toString(36).slice(2, 8)}`;
}

function resetControlPlaneState(): void {
  connectionsState = null;
  credentialsState = null;
  for (const key of Object.keys(grantsState)) delete grantsState[key];
  agentOnlineAt.clear();
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
      });
    }
    if (method === "PUT") {
      return void readBody(req).then((raw) => {
        const body = raw as Json;
        conn.name = body.name ?? conn.name;
        conn.transport = body.transport ?? conn.transport;
        conn.config = body.config ?? {};
        send(res, 200, {
          id,
          name: conn.name,
          protocol: conn.protocol,
          transport: conn.transport,
          ownerId: "u-demo",
          config: conn.config,
          secrets: {},
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

  if (path === "/api/credentials" && method === "GET") {
    const kinds = url.searchParams.get("kind")?.split(",").filter(Boolean);
    const protocol = url.searchParams.get("protocol");
    const filtered = credentials().filter((c) => {
      const protocols = c.protocols as string[] | undefined;
      if (kinds && kinds.length > 0 && !kinds.includes(String(c.kind)))
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
        username: body.username,
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
        cred.username = body.username;
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
