import type { Plugin, ViteDevServer, Connect } from "vite";
import type { IncomingMessage, ServerResponse } from "node:http";
import type { Duplex } from "node:stream";
import { readFileSync, readdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { WebSocketServer, type WebSocket } from "ws";
import { listData, badgeData, docData } from "./datasets.ts";

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

  if (path === "/api/connections" && method === "GET") {
    return send(res, 200, readJSON("connections.json"));
  }

  if (path === "/api/credentials" && method === "GET") {
    const all =
      readJSON<{ kind: string; protocols?: string[] }[]>("credentials.json");
    const kinds = url.searchParams.get("kind")?.split(",").filter(Boolean);
    const protocol = url.searchParams.get("protocol");
    const filtered = all.filter((c) => {
      if (kinds && kinds.length > 0 && !kinds.includes(c.kind)) return false;
      if (protocol && c.protocols && !c.protocols.includes(protocol))
        return false;
      return true;
    });
    return send(res, 200, filtered);
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
    // Actions (POST/PUT/PATCH/DELETE) — acknowledge.
    void readBody(req).then(() => send(res, 200, { ok: true, routeId }));
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
