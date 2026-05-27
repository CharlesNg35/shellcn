package kubernetes

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	"github.com/charlesng/shellcn/internal/plugin"
)

func isCRD(k kind) bool { return strings.HasPrefix(k.name, crdParamPrefix) }

// tableList fetches a kind via the server-side Table API, returning the server's
// column names (in order) and rows keyed by those names. This gives custom
// resources their own printer columns (Lens/kubectl parity) with no hardcoding.
func (s *Session) tableList(ctx context.Context, k kind, ns string, limit int64) (cols []string, rows []Row, err error) {
	cfg := rest.CopyConfig(s.rest)
	gv := k.gvr.GroupVersion()
	cfg.GroupVersion = &gv
	cfg.APIPath = "/apis"
	if gv.Group == "" {
		cfg.APIPath = "/api"
	}
	cfg.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	rc, err := rest.RESTClientFor(cfg)
	if err != nil {
		return nil, nil, err
	}
	req := rc.Get().Resource(k.gvr.Resource).SetHeader("Accept", "application/json;as=Table;v=v1;g=meta.k8s.io")
	if ns != "" {
		req = req.Namespace(ns)
	}
	if limit > 0 {
		req = req.Param("limit", strconv.FormatInt(limit, 10))
	}
	raw, err := req.DoRaw(ctx)
	if err != nil {
		return nil, nil, apiErr(err)
	}
	var table metav1.Table
	if err := json.Unmarshal(raw, &table); err != nil {
		return nil, nil, err
	}
	for i := range table.ColumnDefinitions {
		cols = append(cols, table.ColumnDefinitions[i].Name)
	}
	for i := range table.Rows {
		tr := &table.Rows[i]
		row := Row{}
		for j := range table.ColumnDefinitions {
			if j < len(tr.Cells) {
				row[table.ColumnDefinitions[j].Name] = tr.Cells[j]
			}
		}
		name, namespace, uid := rowObjectMeta(tr.Object.Raw)
		row["ref"] = plugin.ResourceRef{Kind: customResourceKind, Scope: k.name, Namespace: namespace, Name: name, UID: uid}
		rows = append(rows, row)
	}
	return cols, rows, nil
}

func rowObjectMeta(raw []byte) (name, namespace, uid string) {
	if len(raw) == 0 {
		return "", "", ""
	}
	var pom struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
			UID       string `json:"uid"`
		} `json:"metadata"`
	}
	_ = json.Unmarshal(raw, &pom)
	return pom.Metadata.Name, pom.Metadata.Namespace, pom.Metadata.UID
}

// ColumnsForKind returns the column definitions for a kind's list, used by the
// generic table's columnsSource. CRDs report their server-side printer columns;
// built-in kinds report their declared columns.
func ColumnsForKind(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	k, err := resolveKind(s, rc.Param("kind"))
	if err != nil {
		return nil, err
	}
	var rows []Row
	if isCRD(k) {
		cols, _, err := s.tableList(rc.Ctx, k, "", 1)
		if err != nil {
			return nil, err
		}
		for _, c := range cols {
			rows = append(rows, Row{"name": c, "label": c})
		}
	} else {
		for _, c := range k.columns {
			rows = append(rows, Row{"name": c.Key, "label": c.Label})
		}
	}
	return plugin.Page[Row]{Items: rows, Total: ptr(len(rows))}, nil
}
