package kubernetes

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

const helmKind = "helmrelease"

// helmStatusSeverities colors a Helm release status badge by value.
var helmStatusSeverities = map[string]plugin.Severity{
	"deployed": plugin.SeveritySuccess, "failed": plugin.SeverityDanger,
	"pending-install": plugin.SeverityWarn, "pending-upgrade": plugin.SeverityWarn, "pending-rollback": plugin.SeverityWarn,
	"superseded": plugin.SeveritySecondary, "uninstalled": plugin.SeveritySecondary, "uninstalling": plugin.SeveritySecondary,
}

// helmRelease is the subset of a Helm v3 release object the cockpit shows.
type helmRelease struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Version   int    `json:"version"`
	Info      struct {
		Status       string `json:"status"`
		LastDeployed string `json:"last_deployed"`
		Description  string `json:"description"`
		Notes        string `json:"notes"`
	} `json:"info"`
	Chart struct {
		Metadata struct {
			Name       string `json:"name"`
			Version    string `json:"version"`
			AppVersion string `json:"appVersion"`
		} `json:"metadata"`
	} `json:"chart"`
}

// decodeHelmRelease decodes a Helm v3 release Secret payload (base64 → gzip →
// JSON). The typed client already base64-decodes the Secret's data values.
func decodeHelmRelease(data []byte) (helmRelease, error) {
	var rel helmRelease
	gz, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return rel, err
	}
	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return rel, err
	}
	defer func() { _ = zr.Close() }()
	raw, err := io.ReadAll(io.LimitReader(zr, 16<<20))
	if err != nil {
		return rel, err
	}
	return rel, json.Unmarshal(raw, &rel)
}

// helmReleases lists the latest revision of each Helm release (one Secret per
// revision labelled owner=helm).
func (s *Session) helmReleases(rc *plugin.RequestContext) (map[string]helmRelease, error) {
	ns := rc.Param("namespace")
	if ns == "" {
		ns = s.defaultNS
	}
	secrets, err := s.clientset.CoreV1().Secrets(ns).List(rc.Ctx, metav1.ListOptions{LabelSelector: "owner=helm"})
	if err != nil {
		return nil, apiErr(err)
	}
	latest := map[string]helmRelease{}
	for i := range secrets.Items {
		rel, err := decodeHelmRelease(secrets.Items[i].Data["release"])
		if err != nil {
			continue
		}
		key := rel.Namespace + "/" + rel.Name
		if cur, ok := latest[key]; !ok || rel.Version > cur.Version {
			latest[key] = rel
		}
	}
	return latest, nil
}

func HelmReleases(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	releases, err := s.helmReleases(rc)
	if err != nil {
		return nil, err
	}
	rows := make([]Row, 0, len(releases))
	for _, rel := range releases {
		rows = append(rows, Row{
			"name":       rel.Name,
			"namespace":  rel.Namespace,
			"revision":   int64(rel.Version),
			"status":     rel.Info.Status,
			"chart":      rel.Chart.Metadata.Name + "-" + rel.Chart.Metadata.Version,
			"appVersion": rel.Chart.Metadata.AppVersion,
			"updatedAt":  rel.Info.LastDeployed,
			"ref":        plugin.ResourceIdentity{Kind: helmKind, Namespace: rel.Namespace, Name: rel.Name, UID: rel.Namespace + "/" + rel.Name},
		})
	}
	return pageRows(rc, rows)
}

// HelmRelease returns one release's detail (status, chart, notes).
func HelmRelease(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	releases, err := s.helmReleases(rc)
	if err != nil {
		return nil, err
	}
	name := rc.Param("name")
	ns := rc.Param("namespace")
	rel, ok := releases[ns+"/"+name]
	if !ok {
		return nil, plugin.ErrNotFound
	}
	return map[string]any{
		"name":        rel.Name,
		"namespace":   rel.Namespace,
		"revision":    rel.Version,
		"status":      rel.Info.Status,
		"chart":       rel.Chart.Metadata.Name + "-" + rel.Chart.Metadata.Version,
		"appVersion":  rel.Chart.Metadata.AppVersion,
		"updatedAt":   rel.Info.LastDeployed,
		"description": rel.Info.Description,
		"notes":       rel.Info.Notes,
	}, nil
}

// helmReleaseResourceType is the Helm Releases list/detail (derived from release
// Secrets, not a Kubernetes API kind, so it has its own routes).
func helmReleaseResourceType() plugin.ResourceType {
	return plugin.ResourceType{
		Kind:  helmKind,
		Title: "Releases",
		List:  plugin.DataSource{RouteID: "kubernetes.helm.releases"},
		Columns: []plugin.Column{
			nameCol(), nsCol(), col("revision", "Rev", num), col("status", "Status", statusBadge(helmStatusSeverities)),
			col("chart", "Chart"), col("appVersion", "App version"), col("updatedAt", "Updated", func(c *plugin.Column) { c.Type = plugin.ColumnDateTime }),
		},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}", StatusField: "status", Severities: helmStatusSeverities},
			Tabs: []plugin.Panel{
				{
					Key: "overview", Label: "Overview", Icon: lucide("info"), Type: plugin.PanelObjectDetail,
					Source: &plugin.DataSource{RouteID: "kubernetes.helm.release", Params: map[string]string{"namespace": "${resource.namespace}", "name": "${resource.name}"}},
					Config: helmReleaseDetailConfig(),
				},
			},
		},
	}
}

func helmReleaseDetailConfig() plugin.ObjectDetailConfig {
	return plugin.ObjectDetailConfig{
		RawToggle: true,
		Sections: []plugin.ObjectDetailSection{
			{Title: "Release", Fields: []plugin.ObjectDetailField{
				{Key: "name", Label: "Name", Copy: true},
				{Key: "namespace", Label: "Namespace", Copy: true},
				{Key: "revision", Label: "Revision", Type: plugin.ColumnNumber},
				{Key: "status", Label: "Status", Type: plugin.ColumnBadge, Severities: helmStatusSeverities},
				{Key: "chart", Label: "Chart"},
				{Key: "appVersion", Label: "App version"},
				{Key: "updatedAt", Label: "Updated", Type: plugin.ColumnDateTime},
				{Key: "notes", Label: "Notes"},
			}},
		},
	}
}

// TreeHelm lists the Helm sub-items (Releases; Charts/repos are out of scope).
func TreeHelm(_ *plugin.RequestContext) (any, error) {
	return plugin.Page[plugin.TreeNode]{Items: []plugin.TreeNode{
		{Key: "helm:releases", Label: "Releases", Icon: lucide("package"), Leaf: true, ResourceKind: helmKind},
	}, Total: ptr(1)}, nil
}
