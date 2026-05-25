import { describe, it, expect } from "vitest";
import ssh from "../../fixtures/ssh.json";
import docker from "../../fixtures/docker.json";
import proxmox from "../../fixtures/proxmox.json";
import postgres from "../../fixtures/postgres.json";
import connections from "../../fixtures/connections.json";
import type {
  ConnectionSummary,
  DataSource,
  PluginProjection,
  RiskLevel,
} from "./projection";

const projections: Record<string, unknown> = { ssh, docker, proxmox, postgres };

const risks: RiskLevel[] = ["safe", "write", "destructive", "privileged"];
const iconTypes = ["name", "url", "base64", "emoji"];

function assertDataSource(ds: DataSource, where: string) {
  expect(typeof ds.routeId, `${where}: routeId`).toBe("string");
  expect(ds.routeId.length, `${where}: routeId non-empty`).toBeGreaterThan(0);
}

function validate(name: string, raw: unknown): PluginProjection {
  const p = raw as PluginProjection;
  expect(p.name, `${name}.name`).toBe(name);
  expect(typeof p.apiVersion).toBe("number");
  expect(typeof p.title).toBe("string");
  expect(iconTypes).toContain(p.icon.type);
  expect(["tabs", "sidebar_tree"]).toContain(p.layout);
  expect(Array.isArray(p.supportedTransports)).toBe(true);
  expect(Array.isArray(p.config.groups)).toBe(true);

  const actionIds = new Set((p.actions ?? []).map((a) => a.id));
  for (const a of p.actions ?? []) {
    expect(risks, `${name}.action ${a.id}.risk`).toContain(a.risk);
    expect(typeof a.requiresConfirm).toBe("boolean");
    expect(typeof a.routeId).toBe("string");
    for (const [key, value] of Object.entries(a.params ?? {})) {
      expect(typeof key, `${name}.action ${a.id}.param key`).toBe("string");
      expect(typeof value, `${name}.action ${a.id}.param ${key}`).toBe(
        "string",
      );
    }
  }
  for (const t of p.tabs ?? []) {
    expect(typeof t.panel).toBe("string");
    if (t.source) assertDataSource(t.source, `${name}.tab ${t.key}`);
  }
  for (const g of p.tree ?? []) {
    assertDataSource(g.source, `${name}.tree ${g.key}`);
  }
  for (const r of p.resources ?? []) {
    assertDataSource(r.list, `${name}.resource ${r.kind}.list`);
    if (r.watch) assertDataSource(r.watch, `${name}.resource ${r.kind}.watch`);
    expect(r.columns.length).toBeGreaterThan(0);
    for (const id of r.actionIds) {
      expect(
        actionIds.has(id),
        `${name}.resource ${r.kind} references undeclared action ${id}`,
      ).toBe(true);
    }
    for (const tab of r.detail.tabs) {
      expect(typeof tab.panel).toBe("string");
    }
  }
  return p;
}

describe("fixtures", () => {
  it("every projection conforms to the contract", () => {
    for (const [name, raw] of Object.entries(projections)) {
      validate(name, raw);
    }
  });

  it("covers tabs, tree, resource-detail, enroll and query shapes", () => {
    const sshP = validate("ssh", ssh);
    expect(sshP.layout).toBe("tabs");
    expect((sshP.tabs ?? []).map((t) => t.panel)).toContain("terminal");

    const dockerP = validate("docker", docker);
    expect(dockerP.layout).toBe("sidebar_tree");
    expect((dockerP.tree ?? []).length).toBeGreaterThan(0);
    expect((dockerP.resources ?? [])[0].detail.tabs.length).toBeGreaterThan(0);
    expect(dockerP.supportedTransports).toContain("agent");

    const pgP = validate("postgres", postgres);
    const panels = (pgP.resources ?? [])[0].detail.tabs.map((t) => t.panel);
    expect(panels).toContain("query_editor");
  });

  it("connections reference known plugins and shape", () => {
    const list = connections as ConnectionSummary[];
    const names = new Set(Object.keys(projections));
    for (const c of list) {
      expect(typeof c.id).toBe("string");
      expect(
        names.has(c.protocol),
        `connection ${c.id} unknown protocol ${c.protocol}`,
      ).toBe(true);
      expect(["direct", "agent"]).toContain(c.transport);
    }
    expect(
      list.some((c) => c.transport === "agent" && c.online === false),
    ).toBe(true);
  });
});
