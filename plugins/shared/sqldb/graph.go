package sqldb

import (
	"fmt"
	"strings"
)

// GraphPayload is the {nodes, edges} document the generic graph panel renders.
// It is the same shape every graph-capable plugin emits, so relational schemas
// reuse the panel that the graph databases already use.
type GraphPayload struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

type GraphNode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Group string `json:"group,omitempty"`
}

type GraphEdge struct {
	ID     string `json:"id,omitempty"`
	Source string `json:"source"`
	Target string `json:"target"`
	Label  string `json:"label,omitempty"`
}

// ForeignKey is one introspected relationship: a child table column that
// references a parent table column.
type ForeignKey struct {
	Constraint   string
	ChildSchema  string
	ChildTable   string
	ChildColumn  string
	ParentSchema string
	ParentTable  string
	ParentColumn string
}

// ForeignKeyFromRow reads a foreign key from an introspection row whose columns
// are aliased to the standard names, so each dialect only writes its own query.
func ForeignKeyFromRow(r map[string]any) ForeignKey {
	return ForeignKey{
		Constraint:   rowString(r, "constraint_name"),
		ChildSchema:  rowString(r, "child_schema"),
		ChildTable:   rowString(r, "child_table"),
		ChildColumn:  rowString(r, "child_column"),
		ParentSchema: rowString(r, "parent_schema"),
		ParentTable:  rowString(r, "parent_table"),
		ParentColumn: rowString(r, "parent_column"),
	}
}

// RelationGraph turns foreign keys into a graph: one node per participating
// table, one edge per foreign key (child to parent), labeled by the referencing
// column(s). Tables with no relations are omitted. Node IDs are schema-qualified
// so identically named tables in different schemas stay distinct; labels are
// qualified only when more than one schema is present, to keep the common
// single-schema diagram clean.
func RelationGraph(fks []ForeignKey) GraphPayload {
	schemas := map[string]struct{}{}
	for _, fk := range fks {
		schemas[fk.ChildSchema] = struct{}{}
		schemas[fk.ParentSchema] = struct{}{}
	}
	qualify := len(schemas) > 1

	payload := GraphPayload{Nodes: []GraphNode{}, Edges: []GraphEdge{}}
	seen := map[string]struct{}{}
	addNode := func(schema, table string) string {
		id := tableID(schema, table)
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			label := table
			if qualify && schema != "" {
				label = id
			}
			payload.Nodes = append(payload.Nodes, GraphNode{ID: id, Label: label, Group: schema})
		}
		return id
	}

	edgeAt := map[string]int{}
	for _, fk := range fks {
		source := addNode(fk.ChildSchema, fk.ChildTable)
		target := addNode(fk.ParentSchema, fk.ParentTable)
		key := fmt.Sprintf("%s->%s:%s", source, target, fk.Constraint)
		if i, ok := edgeAt[key]; ok {
			payload.Edges[i].Label += ", " + fk.ChildColumn
			continue
		}
		edgeAt[key] = len(payload.Edges)
		payload.Edges = append(payload.Edges, GraphEdge{ID: key, Source: source, Target: target, Label: fk.ChildColumn})
	}
	return payload
}

func tableID(schema, table string) string {
	if schema == "" {
		return table
	}
	return schema + "." + table
}

func rowString(r map[string]any, key string) string {
	v, ok := r[key]
	if !ok || v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}
