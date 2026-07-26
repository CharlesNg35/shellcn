package kubernetes

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/metadata"

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

// helmOwnerSelector matches every Helm v3 storage Secret.
const helmOwnerSelector = "owner=helm"

// helmSecretWindow bounds one revision listing. Helm keeps a Secret per revision,
// so the window counts revisions, not releases.
const helmSecretWindow = plugin.MaxPageLimit

var secretsGVR = schema.GroupVersionResource{Version: "v1", Resource: "secrets"}

// helmRevision is one release Secret, identified from its metadata alone.
type helmRevision struct {
	namespace string
	secret    string
	release   string
	status    string
	version   int
}

func (r helmRevision) key() string { return r.namespace + "/" + r.release }

// helmRevisionOf reads a revision's identity from a release Secret's labels —
// Helm labels every one with name and version — falling back to the
// "sh.helm.release.v1.<release>.v<revision>" Secret name when they are absent.
func helmRevisionOf(m metav1.PartialObjectMetadata) (helmRevision, bool) {
	rev := helmRevision{namespace: m.Namespace, secret: m.Name, release: m.Labels["name"], status: m.Labels["status"]}
	rev.version, _ = strconv.Atoi(m.Labels["version"])
	if rev.release != "" && rev.version > 0 {
		return rev, true
	}
	name, ok := strings.CutPrefix(m.Name, "sh.helm.release.v1.")
	if !ok {
		return helmRevision{}, false
	}
	release, version, ok := strings.Cut(name, ".v")
	if !ok {
		return helmRevision{}, false
	}
	n, err := strconv.Atoi(version)
	if err != nil || release == "" {
		return helmRevision{}, false
	}
	rev.release, rev.version = release, n
	return rev, true
}

// helmLatestRevisions returns the current revision of each release in one bounded
// window, sorted by namespace/name so paging is stable. Only ObjectMeta crosses
// the wire (the PartialObjectMetadata accept header), so the gzipped release
// payloads are never transferred just to work out which revision wins.
func (s *Session) helmLatestRevisions(ctx context.Context, ns, release string) (revs []helmRevision, truncated bool, err error) {
	client, err := metadata.NewForConfig(s.rest)
	if err != nil {
		return nil, false, err
	}
	selector := helmOwnerSelector
	if release != "" {
		selector += ",name=" + release
	}
	list, err := client.Resource(secretsGVR).Namespace(ns).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
		Limit:         helmSecretWindow,
	})
	if err != nil {
		return nil, false, apiErr(err)
	}
	latest := map[string]helmRevision{}
	for i := range list.Items {
		rev, ok := helmRevisionOf(list.Items[i])
		if !ok {
			continue
		}
		if cur, seen := latest[rev.key()]; !seen || rev.version > cur.version {
			latest[rev.key()] = rev
		}
	}
	revs = slices.SortedFunc(maps.Values(latest), func(a, b helmRevision) int {
		return strings.Compare(a.key(), b.key())
	})
	return revs, list.GetContinue() != "", nil
}

// helmReleaseAt gunzips one revision Secret into its release record.
func (s *Session) helmReleaseAt(ctx context.Context, rev helmRevision) (helmRelease, error) {
	secret, err := s.clientset.CoreV1().Secrets(rev.namespace).Get(ctx, rev.secret, metav1.GetOptions{})
	if err != nil {
		return helmRelease{}, apiErr(err)
	}
	return decodeHelmRelease(secret.Data["release"])
}

// helmNamespace scopes a release listing; an empty result means every namespace.
func helmNamespace(rc *plugin.RequestContext, s *Session) string {
	if ns := rc.Param("namespace"); ns != "" {
		return ns
	}
	return s.defaultNS
}

// HelmReleases lists one page of releases. The page is chosen from label metadata
// and only its own revisions are decoded, so a namespace retaining many revisions
// per release costs a page of gunzips, not a cluster's worth.
func HelmReleases(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	revs, truncated, err := s.helmLatestRevisions(rc.Ctx, helmNamespace(rc, s), "")
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]helmRevision, len(revs))
	rows := make([]Row, 0, len(revs))
	for _, rev := range revs {
		byKey[rev.key()] = rev
		// Status is a label, so it filters and sorts across the whole window; the
		// chart fields live inside the payload and arrive with the page below.
		rows = append(rows, Row{
			"name":      rev.release,
			"namespace": rev.namespace,
			"revision":  int64(rev.version),
			"status":    rev.status,
			"ref":       plugin.ResourceIdentity{Kind: helmKind, Namespace: rev.namespace, Name: rev.release, UID: rev.key()},
		})
	}
	page, err := pageSlice(rc, rows, !truncated)
	if err != nil {
		return nil, err
	}
	for _, row := range page.Items {
		ref, _ := row["ref"].(plugin.ResourceIdentity)
		rev, ok := byKey[ref.UID]
		if !ok {
			continue
		}
		rel, err := s.helmReleaseAt(rc.Ctx, rev)
		if err != nil {
			continue
		}
		row["status"] = rel.Info.Status
		row["chart"] = rel.Chart.Metadata.Name + "-" + rel.Chart.Metadata.Version
		row["appVersion"] = rel.Chart.Metadata.AppVersion
		row["updatedAt"] = rel.Info.LastDeployed
	}
	return page, nil
}

// HelmRelease returns one release's detail (status, chart, notes).
func HelmRelease(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	name, ns := rc.Param("name"), rc.Param("namespace")
	revs, _, err := s.helmLatestRevisions(rc.Ctx, helmNamespace(rc, s), name)
	if err != nil {
		return nil, err
	}
	idx := slices.IndexFunc(revs, func(r helmRevision) bool {
		return r.release == name && (ns == "" || r.namespace == ns)
	})
	if idx < 0 {
		return nil, plugin.ErrNotFound
	}
	rel, err := s.helmReleaseAt(rc.Ctx, revs[idx])
	if err != nil {
		return nil, err
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
