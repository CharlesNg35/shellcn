import dagre from "@dagrejs/dagre";
import { MarkerType, type Edge, type Node } from "@vue-flow/core";

export interface GraphField {
  name: string;
  type?: string;
  key?: string;
}

export interface GraphNode {
  id: string;
  label?: string;
  group?: string;
  fields?: GraphField[];
  summary?: string;
  properties?: Record<string, unknown>;
}

export interface GraphEdge {
  id?: string;
  source: string;
  target: string;
  label?: string;
  animated?: boolean;
}

export interface GraphPayload {
  nodes?: GraphNode[];
  edges?: GraphEdge[];
}

const HEADER_H = 30;
const ROW_H = 22;
const FIELDS_WIDTH = 220;
const PLAIN_WIDTH = 170;
const PLAIN_HEIGHT = 42;

const EDGE_PALETTE = [
  "#6366f1",
  "#10b981",
  "#f59e0b",
  "#ef4444",
  "#06b6d4",
  "#8b5cf6",
  "#ec4899",
  "#14b8a6",
];

export function edgeColor(label?: string): string {
  if (!label) return "#94a3b8";
  let hash = 0;
  for (let i = 0; i < label.length; i++) {
    hash = (hash * 31 + label.charCodeAt(i)) >>> 0;
  }
  return EDGE_PALETTE[hash % EDGE_PALETTE.length];
}

export function mergeGraph(
  base: GraphPayload,
  incoming: GraphPayload,
): { nodes: GraphNode[]; edges: GraphEdge[] } {
  const edgeKey = (e: GraphEdge) =>
    e.id ?? `${e.source}->${e.target}:${e.label ?? ""}`;
  const nodes = [...(base.nodes ?? [])];
  const nodeIds = new Set(nodes.map((n) => n.id));
  for (const node of incoming.nodes ?? []) {
    if (!nodeIds.has(node.id)) {
      nodes.push(node);
      nodeIds.add(node.id);
    }
  }
  const edges = [...(base.edges ?? [])];
  const edgeKeys = new Set(edges.map(edgeKey));
  for (const edge of incoming.edges ?? []) {
    const key = edgeKey(edge);
    if (!edgeKeys.has(key)) {
      edges.push(edge);
      edgeKeys.add(key);
    }
  }
  return { nodes, edges };
}

function nodeSize(node: GraphNode): { width: number; height: number } {
  if (node.fields?.length) {
    return {
      width: FIELDS_WIDTH,
      height: HEADER_H + node.fields.length * ROW_H,
    };
  }
  return { width: PLAIN_WIDTH, height: PLAIN_HEIGHT };
}

export function buildGraph(payload: GraphPayload): {
  nodes: Node[];
  edges: Edge[];
} {
  const raw = payload.nodes ?? [];
  const g = new dagre.graphlib.Graph();
  g.setDefaultEdgeLabel(() => ({}));
  g.setGraph({
    rankdir: "LR",
    nodesep: 36,
    ranksep: 90,
    marginx: 16,
    marginy: 16,
  });

  const sizes = new Map<string, { width: number; height: number }>();
  for (const node of raw) {
    const size = nodeSize(node);
    sizes.set(node.id, size);
    g.setNode(node.id, size);
  }
  for (const edge of payload.edges ?? []) {
    if (sizes.has(edge.source) && sizes.has(edge.target)) {
      g.setEdge(edge.source, edge.target);
    }
  }
  dagre.layout(g);

  const nodes: Node[] = raw.map((node) => {
    const size = sizes.get(node.id)!;
    const placed = g.node(node.id);
    const fielded = Boolean(node.fields?.length);
    return {
      id: node.id,
      type: fielded ? "record" : "default",
      position: { x: placed.x - size.width / 2, y: placed.y - size.height / 2 },
      data: {
        label: node.label ?? node.id,
        group: node.group,
        fields: node.fields ?? [],
      },
      class: fielded ? undefined : "shellcn-graph-node",
      style: { width: `${size.width}px` },
    };
  });

  const edges: Edge[] = (payload.edges ?? []).map((edge, i) => {
    const color = edgeColor(edge.label);
    return {
      id: edge.id ?? `${edge.source}-${edge.target}-${i}`,
      source: edge.source,
      target: edge.target,
      label: edge.label,
      type: "smoothstep",
      animated: edge.animated,
      style: { stroke: color },
      markerEnd: { type: MarkerType.ArrowClosed, color },
    };
  });

  return { nodes, edges };
}
