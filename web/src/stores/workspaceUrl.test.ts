import { describe, it, expect } from "vitest";
import { serializeView, parseView } from "./workspaceUrl";
import type { OpenView } from "./workspace";
import type { ResourceType, TreeGroup } from "../types/projection";

const resources = [
  {
    kind: "container",
    title: "Containers",
    list: { routeId: "x.containers" },
    columns: [],
    detail: { header: {}, tabs: [] },
  },
  {
    kind: "pod",
    title: "Pods",
    list: { routeId: "x.pods" },
    columns: [],
    detail: { header: {}, tabs: [] },
  },
] as unknown as ResourceType[];

const tree: TreeGroup[] = [
  { key: "containers", label: "Containers", resourceKind: "container" },
];

describe("workspaceUrl codec", () => {
  it("round-trips a group view", () => {
    const v: OpenView = {
      id: "group:containers",
      title: "Containers",
      kind: "list",
      groupKey: "containers",
    };
    const s = serializeView(v);
    expect(s).toBe("group:containers");
    const back = parseView(s, resources, tree);
    expect(back).toMatchObject({
      id: "group:containers",
      groupKey: "containers",
    });
    expect(serializeView(back!)).toBe(s);
  });

  it("round-trips a list view with params", () => {
    const v: OpenView = {
      id: "list:pod:namespace=default",
      title: "Pods",
      kind: "list",
      resourceKind: "pod",
      params: { namespace: "default" },
    };
    const s = serializeView(v);
    expect(s).toBe("list:pod:namespace=default");
    const back = parseView(s, resources, tree)!;
    expect(back.id).toBe("list:pod:namespace=default");
    expect(back.params).toEqual({ namespace: "default" });
  });

  it("round-trips a detail view carrying its ref", () => {
    const v: OpenView = {
      id: "detail:abc",
      title: "web",
      kind: "detail",
      ref: { kind: "container", uid: "abc", name: "web" },
    };
    const s = serializeView(v);
    expect(s).toBe("detail:container:abc:n=web");
    const back = parseView(s, resources, tree)!;
    expect(back.id).toBe("detail:abc");
    expect(back.ref).toMatchObject({
      kind: "container",
      uid: "abc",
      name: "web",
    });
    // A minimal row is synthesized so DetailView renders.
    expect(back.row).toMatchObject({ uid: "abc", name: "web" });
    expect(serializeView(back)).toBe(s);
  });

  it("encodes special characters safely", () => {
    const v: OpenView = {
      id: "detail:a:b",
      title: "n",
      kind: "detail",
      ref: {
        kind: "table",
        uid: "a:b,c",
        name: "my table",
        namespace: "pub/lic",
      },
    };
    const s = serializeView(v);
    const back = parseView(
      s,
      [...resources, { kind: "table" } as ResourceType],
      tree,
    )!;
    expect(back.ref).toMatchObject({
      uid: "a:b,c",
      name: "my table",
      namespace: "pub/lic",
    });
  });

  it("returns null for an unknown kind or garbage", () => {
    expect(parseView("group:nope", resources, tree)).toBeNull();
    expect(parseView("list:ghost", resources, tree)).toBeNull();
    expect(parseView("detail:ghost:1", resources, tree)).toBeNull();
    expect(parseView("nonsense", resources, tree)).toBeNull();
  });
});
