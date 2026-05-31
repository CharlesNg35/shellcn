import type { ResourceRef, ResourceType, TreeGroup } from "../types/projection";
import type { OpenView } from "./workspace";

// The active sidebar-tree view encoded for the `?v=` query param, and back. The
// codec is self-sufficient: a detail carries its ResourceRef, so a pasted/refreshed
// URL reconstructs the view without any open-views state. Each dynamic piece is
// percent-encoded, so the `:` `,` `=` delimiters never collide with a value.

const enc = encodeURIComponent;
const dec = decodeURIComponent;

function paramSuffix(params?: Record<string, string>): string {
  if (!params || !Object.keys(params).length) return "";
  return Object.entries(params)
    .map(([k, v]) => `${k}=${v}`)
    .join(",");
}

// Mirrors TreeWorkspace's OpenView.id scheme so a reconstructed view matches an
// already-open one (group:<key>, list:<kind>[:k=v,...], detail:<uid>).
function listId(resourceKind: string, params?: Record<string, string>): string {
  const suffix = paramSuffix(params);
  return suffix ? `list:${resourceKind}:${suffix}` : `list:${resourceKind}`;
}

export function serializeView(view: OpenView): string {
  if (view.kind === "list") {
    if (view.groupKey) return `group:${enc(view.groupKey)}`;
    const kind = view.resourceKind ?? "";
    const params = view.params
      ? Object.entries(view.params)
          .map(([k, v]) => `${enc(k)}=${enc(v)}`)
          .join(",")
      : "";
    return params ? `list:${enc(kind)}:${params}` : `list:${enc(kind)}`;
  }
  if (view.kind === "detail" && view.ref) {
    const r = view.ref;
    const extras: string[] = [];
    if (r.name) extras.push(`n=${enc(r.name)}`);
    if (r.namespace) extras.push(`ns=${enc(r.namespace)}`);
    if (r.scope) extras.push(`sc=${enc(r.scope)}`);
    const tail = extras.length ? `:${extras.join(",")}` : "";
    return `detail:${enc(r.kind)}:${enc(r.uid)}${tail}`;
  }
  return "";
}

function parsePairs(segment: string): Record<string, string> {
  const out: Record<string, string> = {};
  for (const pair of segment.split(",")) {
    const eq = pair.indexOf("=");
    if (eq < 0) continue;
    out[dec(pair.slice(0, eq))] = dec(pair.slice(eq + 1));
  }
  return out;
}

function refSubtitle(ref: ResourceRef): string {
  const location = [ref.scope, ref.namespace].filter(Boolean).join(" / ");
  return [ref.kind, location].filter(Boolean).join(" · ");
}

// parseView rebuilds the OpenView from `v` + the manifest. Lists/groups resolve
// fully; a detail rebuilds its ref and a minimal row so DetailView renders.
// Returns null when the locator can't be resolved (caller falls back to default).
export function parseView(
  v: string,
  resources: ResourceType[],
  tree: TreeGroup[],
): OpenView | null {
  const segments = v.split(":");
  const type = segments[0];

  if (type === "group") {
    const key = dec(segments[1] ?? "");
    const group = tree.find((g) => g.key === key);
    if (!group) return null;
    return {
      id: `group:${key}`,
      title: group.label,
      icon: group.icon,
      kind: "list",
      groupKey: key,
    };
  }

  if (type === "list") {
    const kind = dec(segments[1] ?? "");
    const resource = resources.find((r) => r.kind === kind);
    if (!resource) return null;
    const params = segments[2] ? parsePairs(segments[2]) : undefined;
    return {
      id: listId(kind, params),
      title: resource.title,
      subtitle: params ? Object.values(params).join(" / ") : undefined,
      kind: "list",
      resourceKind: kind,
      params,
    };
  }

  if (type === "detail") {
    const kind = dec(segments[1] ?? "");
    const uid = dec(segments[2] ?? "");
    if (!kind || !uid || !resources.some((r) => r.kind === kind)) return null;
    const extras = segments[3] ? parsePairs(segments[3]) : {};
    const ref: ResourceRef = {
      kind,
      uid,
      name: extras.n ?? uid,
      namespace: extras.ns,
      scope: extras.sc,
    };
    return {
      id: `detail:${uid}`,
      title: ref.name,
      subtitle: refSubtitle(ref),
      kind: "detail",
      ref,
      row: {
        ref,
        name: ref.name,
        uid: ref.uid,
        namespace: ref.namespace,
        scope: ref.scope,
      },
    };
  }

  return null;
}
