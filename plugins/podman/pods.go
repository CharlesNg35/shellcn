package podman

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/charlesng35/shellcn/plugins/shared/dockerengine"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// Pods are a Podman-native concept absent from the Docker-compatible API, so
// they are read through the libpod endpoints using the session's HTTP client.
// Podman accepts any parseable version in the libpod path prefix.
const libpodPrefix = "http://docker/v4.0.0/libpod"

type podSummary struct {
	ID         string          `json:"Id"`
	Name       string          `json:"Name"`
	Status     string          `json:"Status"`
	Created    string          `json:"Created"`
	Containers []podSummaryCtr `json:"Containers"`
}

type podSummaryCtr struct {
	ID string `json:"Id"`
}

type podInspectReport struct {
	ID         string            `json:"Id"`
	Name       string            `json:"Name"`
	Created    string            `json:"Created"`
	State      string            `json:"State"`
	Labels     map[string]string `json:"Labels"`
	InfraID    string            `json:"InfraContainerID"`
	Containers []podInspectCtr   `json:"Containers"`
}

type podInspectCtr struct {
	ID    string `json:"Id"`
	Name  string `json:"Name"`
	State string `json:"State"`
}

func listPods(rc *plugin.RequestContext) (any, error) {
	var pods []podSummary
	if err := libpodGet(rc, "/pods/json", &pods); err != nil {
		return nil, err
	}
	rows := make([]dockerengine.Row, 0, len(pods))
	for _, p := range pods {
		rows = append(rows, dockerengine.Row{
			"id":         p.ID,
			"name":       p.Name,
			"status":     p.Status,
			"containers": len(p.Containers),
			"createdAt":  p.Created,
			"ref":        plugin.ResourceRef{Kind: "pod", Name: p.Name, UID: p.ID},
		})
	}
	return dockerengine.PageRows(rc, rows)
}

func treePods(rc *plugin.RequestContext) (any, error) {
	res, err := listPods(rc)
	if err != nil {
		return nil, err
	}
	page := res.(plugin.Page[dockerengine.Row])
	nodes := make([]plugin.TreeNode, 0, len(page.Items))
	for _, r := range page.Items {
		ref, ok := r["ref"].(plugin.ResourceRef)
		if !ok {
			continue
		}
		refCopy := ref
		nodes = append(nodes, plugin.TreeNode{Key: "pod:" + ref.UID, Label: ref.Name, Icon: icon("boxes"), Ref: &refCopy, Leaf: true, Data: r})
	}
	return plugin.Page[plugin.TreeNode]{Items: nodes, NextCursor: page.NextCursor, Total: page.Total}, nil
}

func podOverview(rc *plugin.RequestContext) (any, error) {
	var p podInspectReport
	if err := libpodGet(rc, "/pods/"+rc.Param("id")+"/json", &p); err != nil {
		return nil, err
	}
	return dockerengine.Row{
		"id":         p.ID,
		"name":       p.Name,
		"state":      p.State,
		"created":    p.Created,
		"containers": len(p.Containers),
		"infraId":    dockerengine.ShortID(p.InfraID),
		"labels":     p.Labels,
	}, nil
}

func podInspect(rc *plugin.RequestContext) (any, error) {
	var raw any
	if err := libpodGet(rc, "/pods/"+rc.Param("id")+"/json", &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func podContainers(rc *plugin.RequestContext) (any, error) {
	var p podInspectReport
	if err := libpodGet(rc, "/pods/"+rc.Param("id")+"/json", &p); err != nil {
		return nil, err
	}
	rows := make([]dockerengine.Row, 0, len(p.Containers))
	for _, c := range p.Containers {
		rows = append(rows, dockerengine.Row{
			"id":    c.ID,
			"name":  c.Name,
			"state": c.State,
			"ref":   plugin.ResourceRef{Kind: "container", Name: c.Name, UID: c.ID},
		})
	}
	return dockerengine.PageRows(rc, rows)
}

func startPod(rc *plugin.RequestContext) (any, error) {
	return podAction(rc, http.MethodPost, "/pods/"+rc.Param("id")+"/start")
}

func stopPod(rc *plugin.RequestContext) (any, error) {
	return podAction(rc, http.MethodPost, "/pods/"+rc.Param("id")+"/stop")
}

func restartPod(rc *plugin.RequestContext) (any, error) {
	return podAction(rc, http.MethodPost, "/pods/"+rc.Param("id")+"/restart")
}

func removePod(rc *plugin.RequestContext) (any, error) {
	return podAction(rc, http.MethodDelete, "/pods/"+rc.Param("id")+"?force=true")
}

func podAction(rc *plugin.RequestContext, method, path string) (any, error) {
	if err := libpodReq(rc, method, path); err != nil {
		return nil, err
	}
	return dockerengine.ActionResult{OK: true}, nil
}

// libpodGet issues a GET against Podman's libpod API and decodes the JSON body.
func libpodGet(rc *plugin.RequestContext, path string, out any) error {
	resp, err := libpodDo(rc, http.MethodGet, path)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if status := checkLibpodStatus(resp, path); status != nil {
		return status
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// libpodReq issues a non-GET libpod request (lifecycle action) and discards the
// body, returning only the mapped error.
func libpodReq(rc *plugin.RequestContext, method, path string) error {
	resp, err := libpodDo(rc, method, path)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	return checkLibpodStatus(resp, path)
}

func libpodDo(rc *plugin.RequestContext, method, path string) (*http.Response, error) {
	s, err := dockerengine.Unwrap(rc.Session)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(rc.Ctx, method, libpodPrefix+path, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", plugin.ErrInvalidInput, err)
	}
	resp, err := s.HTTPClient().Do(req)
	if err != nil {
		return nil, dockerengine.DockerErr(err)
	}
	return resp, nil
}

func checkLibpodStatus(resp *http.Response, path string) error {
	if resp.StatusCode == http.StatusNotFound {
		return plugin.ErrNotFound
	}
	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("%w: libpod %s: %s", plugin.ErrUnavailable, path, strings.TrimSpace(string(body)))
	}
	return nil
}
