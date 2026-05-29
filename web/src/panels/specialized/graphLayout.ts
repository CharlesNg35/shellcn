import dagre from "@dagrejs/dagre";
import { MarkerType, type Edge, type Node } from "@vue-flow/core";

// A field listed inside a node box. Generic: any plugin may attach fields to a
// node (relational columns, document keys, …); `key` simply marks a field as
// significant so the panel can badge it (no relational semantics here).
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

// nodeSize derives a node's footprint from its content so the layout engine can
// pack boxes without overlap: a node that lists fields grows with that list, a
// plain node is a fixed box.
function nodeSize(node: GraphNode): { width: number; height: number } {
  if (node.fields?.length) {
    return {
      width: FIELDS_WIDTH,
      height: HEADER_H + node.fields.length * ROW_H,
    };
  }
  return { width: PLAIN_WIDTH, height: PLAIN_HEIGHT };
}

// buildGraph lays nodes out left-to-right with dagre so connected nodes sit in
// adjacent ranks and edges flow one direction, then maps the result to Vue Flow
// nodes/edges. It is pure — selection highlighting is layered on by the panel
// via Vue Flow's own selection state.
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

  const edges: Edge[] = (payload.edges ?? []).map((edge, i) => ({
    id: edge.id ?? `${edge.source}-${edge.target}-${i}`,
    source: edge.source,
    target: edge.target,
    label: edge.label,
    type: "smoothstep",
    animated: edge.animated,
    markerEnd: MarkerType.ArrowClosed,
  }));

  return { nodes, edges };
}
