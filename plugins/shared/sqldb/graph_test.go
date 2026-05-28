package sqldb

import "testing"

func TestRelationGraphBuildsNodesAndEdges(t *testing.T) {
	g := RelationGraph([]ForeignKey{
		{Constraint: "fk_order_customer", ChildSchema: "public", ChildTable: "orders", ChildColumn: "customer_id", ParentSchema: "public", ParentTable: "customers", ParentColumn: "id"},
		{Constraint: "fk_item_order", ChildSchema: "public", ChildTable: "order_items", ChildColumn: "order_id", ParentSchema: "public", ParentTable: "orders", ParentColumn: "id"},
	})
	if len(g.Nodes) != 3 {
		t.Fatalf("expected 3 deduped table nodes, got %d: %#v", len(g.Nodes), g.Nodes)
	}
	if len(g.Edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(g.Edges))
	}
	// Single schema: labels are not schema-qualified.
	for _, n := range g.Nodes {
		if n.Label == "" || n.Label != n.ID[len("public."):] {
			t.Fatalf("single-schema node label should be the bare table name, got %q (id %q)", n.Label, n.ID)
		}
	}
}

func TestRelationGraphMergesCompositeForeignKey(t *testing.T) {
	g := RelationGraph([]ForeignKey{
		{Constraint: "fk", ChildSchema: "s", ChildTable: "a", ChildColumn: "x", ParentSchema: "s", ParentTable: "b", ParentColumn: "p"},
		{Constraint: "fk", ChildSchema: "s", ChildTable: "a", ChildColumn: "y", ParentSchema: "s", ParentTable: "b", ParentColumn: "q"},
	})
	if len(g.Edges) != 1 {
		t.Fatalf("composite FK should collapse to one edge, got %d", len(g.Edges))
	}
	if g.Edges[0].Label != "x, y" {
		t.Fatalf("composite edge label = %q, want %q", g.Edges[0].Label, "x, y")
	}
}

func TestRelationGraphQualifiesLabelsAcrossSchemas(t *testing.T) {
	g := RelationGraph([]ForeignKey{
		{Constraint: "fk", ChildSchema: "sales", ChildTable: "orders", ChildColumn: "uid", ParentSchema: "auth", ParentTable: "users", ParentColumn: "id"},
	})
	for _, n := range g.Nodes {
		if n.Label != n.ID {
			t.Fatalf("multi-schema node label should be schema-qualified, got %q (id %q)", n.Label, n.ID)
		}
	}
}
