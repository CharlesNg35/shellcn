package dockerengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
	dockerclient "github.com/moby/moby/client"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

const (
	eventKindContainer = "container"
	eventKindImage     = "image"
	eventKindVolume    = "volume"
	eventKindNetwork   = "network"
	eventKindCompose   = "compose"
)

type CommitContainerResult struct {
	OK      bool   `json:"ok"`
	ImageID string `json:"imageId"`
}

type RedeployContainerResult struct {
	OK bool   `json:"ok"`
	ID string `json:"id"`
}

func ContainerStats(rc *plugin.RequestContext, stream plugin.ClientStream) error {
	s, err := sess(rc)
	if err != nil {
		return err
	}
	id := rc.Param("id")
	if id == "" {
		return fmt.Errorf("%w: container id is required", plugin.ErrInvalidInput)
	}
	res, err := s.cli.ContainerStats(rc.Ctx, id, dockerclient.ContainerStatsOptions{Stream: true})
	if err != nil {
		return DockerErr(err)
	}
	defer func() { _ = res.Body.Close() }()

	dec := json.NewDecoder(res.Body)
	enc := json.NewEncoder(stream)
	var prev container.StatsResponse
	for {
		var sample container.StatsResponse
		if err := dec.Decode(&sample); err != nil {
			if err == io.EOF || rc.Ctx.Err() != nil || stream.Context().Err() != nil {
				return nil
			}
			return DockerErr(err)
		}
		frame := statsFrame(sample, prev)
		prev = sample
		if err := enc.Encode(frame); err != nil {
			return nil
		}
	}
}

func ContainerProcesses(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.ContainerTop(rc.Ctx, rc.Param("id"), dockerclient.ContainerTopOptions{})
	if err != nil {
		return nil, DockerErr(err)
	}
	rows := make([]Row, 0, len(res.Processes))
	for i, proc := range res.Processes {
		row := Row{"ref": plugin.ResourceIdentity{Kind: "process", Name: fmt.Sprintf("Process %d", i+1), UID: fmt.Sprintf("%s:%d", rc.Param("id"), i)}}
		for j, title := range res.Titles {
			if j < len(proc) {
				row[processKey(title)] = proc[j]
			}
		}
		rows = append(rows, row)
	}
	return PageRows(rc, rows)
}

func ImageHistory(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.ImageHistory(rc.Ctx, rc.Param("id"))
	if err != nil {
		return nil, DockerErr(err)
	}
	rows := make([]Row, 0, len(res.Items))
	for i, item := range res.Items {
		id := item.ID
		if id == "" || id == "<missing>" {
			id = fmt.Sprintf("layer-%d", i)
		}
		rows = append(rows, Row{
			"id":        item.ID,
			"createdAt": unixTime(item.Created),
			"createdBy": item.CreatedBy,
			"size":      item.Size,
			"tags":      strings.Join(item.Tags, ", "),
			"comment":   item.Comment,
			"ref":       plugin.ResourceIdentity{Kind: "image_layer", Name: id, UID: fmt.Sprintf("%s:%d", rc.Param("id"), i)},
		})
	}
	return PageRows(rc, rows)
}

func NetworkPorts(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.ContainerList(rc.Ctx, dockerclient.ContainerListOptions{All: true})
	if err != nil {
		return nil, DockerErr(err)
	}
	networkID := strings.TrimSpace(rc.Param("id"))
	rows := make([]Row, 0)
	for _, c := range res.Items {
		if networkID != "" && !containerAttachedToNetwork(c, networkID) {
			continue
		}
		name := firstName(c.Names, shortID(c.ID))
		for _, p := range c.Ports {
			private := int(p.PrivatePort)
			proto := p.Type
			if proto == "" {
				proto = "tcp"
			}
			host := p.IP.String()
			if !p.IP.IsValid() {
				host = "0.0.0.0"
			}
			published := p.PublicPort > 0
			uid := fmt.Sprintf("%s:%d/%s:%d", c.ID, private, proto, p.PublicPort)
			rows = append(rows, Row{
				"container":   name,
				"containerId": c.ID,
				"image":       c.Image,
				"privatePort": private,
				"publicPort":  int(p.PublicPort),
				"hostIP":      host,
				"protocol":    proto,
				"published":   published,
				"compose":     c.Labels[ComposeProjectLabel],
				"ref":         plugin.ResourceIdentity{Kind: "port", Name: fmt.Sprintf("%s:%d/%s", name, private, proto), UID: uid},
			})
		}
	}
	return PageRows(rc, rows)
}

func containerAttachedToNetwork(c container.Summary, networkID string) bool {
	if c.NetworkSettings == nil {
		return false
	}
	for name, n := range c.NetworkSettings.Networks {
		if name == networkID {
			return true
		}
		if n != nil && (n.NetworkID == networkID || strings.HasPrefix(n.NetworkID, networkID)) {
			return true
		}
	}
	return false
}

func NetworkTopology(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	containers, err := s.cli.ContainerList(rc.Ctx, dockerclient.ContainerListOptions{All: true})
	if err != nil {
		return nil, DockerErr(err)
	}
	networks, err := s.cli.NetworkList(rc.Ctx, dockerclient.NetworkListOptions{})
	if err != nil {
		return nil, DockerErr(err)
	}
	nodes := make([]Row, 0, len(containers.Items)+len(networks.Items))
	edges := make([]Row, 0)
	for _, n := range networks.Items {
		nodes = append(nodes, Row{
			"id":      "network:" + n.ID,
			"label":   n.Name,
			"group":   "Network",
			"summary": n.Driver,
			"properties": Row{
				"driver":     n.Driver,
				"scope":      n.Scope,
				"internal":   n.Internal,
				"attachable": n.Attachable,
			},
		})
	}
	for _, c := range containers.Items {
		cid := "container:" + c.ID
		name := firstName(c.Names, shortID(c.ID))
		nodes = append(nodes, Row{
			"id":      cid,
			"label":   name,
			"group":   "Container",
			"summary": c.State,
			"properties": Row{
				"image":   c.Image,
				"state":   c.State,
				"status":  c.Status,
				"compose": c.Labels[ComposeProjectLabel],
			},
		})
		if c.NetworkSettings == nil {
			continue
		}
		for _, n := range c.NetworkSettings.Networks {
			if n == nil || n.NetworkID == "" {
				continue
			}
			edges = append(edges, Row{
				"id":     fmt.Sprintf("%s->%s", cid, n.NetworkID),
				"source": cid,
				"target": "network:" + n.NetworkID,
				"label":  "attached",
			})
		}
	}
	return Row{"nodes": nodes, "edges": edges}, nil
}

func EventsTimeline(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	since := rc.Query().Get("since")
	if since == "" {
		since = now.Add(-1 * time.Hour).Format(time.RFC3339)
	}
	ctx, cancel := context.WithTimeout(rc.Ctx, 5*time.Second)
	defer cancel()
	result := s.cli.Events(ctx, dockerclient.EventsListOptions{
		Since: since,
		Until: now.Format(time.RFC3339),
	})
	rows := make([]Row, 0, 100)
	for {
		select {
		case msg, ok := <-result.Messages:
			if !ok {
				sortTimeline(rows)
				return PageRows(rc, rows)
			}
			rows = append(rows, eventTimelineRow(msg))
			if len(rows) >= 500 {
				sortTimeline(rows)
				return PageRows(rc, rows)
			}
		case err, ok := <-result.Err:
			if !ok || err == nil || err == io.EOF || err == context.Canceled || err == context.DeadlineExceeded {
				sortTimeline(rows)
				return PageRows(rc, rows)
			}
			return nil, DockerErr(err)
		case <-ctx.Done():
			sortTimeline(rows)
			return PageRows(rc, rows)
		}
	}
}

func CommitContainer(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	var req struct {
		Reference string `json:"reference" validate:"required"`
		Comment   string `json:"comment"`
		Author    string `json:"author"`
		NoPause   bool   `json:"no_pause"`
		Changes   string `json:"changes"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	ref := strings.TrimSpace(req.Reference)
	if ref == "" {
		return nil, fmt.Errorf("%w: image reference is required", plugin.ErrInvalidInput)
	}
	res, err := s.cli.ContainerCommit(rc.Ctx, rc.Param("id"), dockerclient.ContainerCommitOptions{
		Reference: ref,
		Comment:   strings.TrimSpace(req.Comment),
		Author:    strings.TrimSpace(req.Author),
		NoPause:   req.NoPause,
		Changes:   nonEmptyLines(req.Changes),
	})
	if err != nil {
		return nil, DockerErr(err)
	}
	return CommitContainerResult{OK: true, ImageID: res.ID}, nil
}

func RedeployContainer(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	var req struct {
		Pull bool `json:"pull"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	inspect, err := s.cli.ContainerInspect(rc.Ctx, rc.Param("id"), dockerclient.ContainerInspectOptions{})
	if err != nil {
		return nil, DockerErr(err)
	}
	cfg := cloneContainerConfig(inspect.Container.Config)
	host := cloneHostConfig(inspect.Container.HostConfig)
	netCfg := cloneNetworkingConfig(inspect.Container.NetworkSettings)
	name := strings.TrimPrefix(inspect.Container.Name, "/")
	imageRef := cfg.Image
	if req.Pull && imageRef != "" {
		pull, err := s.cli.ImagePull(rc.Ctx, imageRef, dockerclient.ImagePullOptions{})
		if err != nil {
			return nil, DockerErr(err)
		}
		if err := pull.Wait(rc.Ctx); err != nil {
			_ = pull.Close()
			return nil, DockerErr(err)
		}
		if err := pull.Close(); err != nil {
			return nil, DockerErr(err)
		}
	}
	wasRunning := inspect.Container.State != nil && inspect.Container.State.Running
	_, _ = s.cli.ContainerStop(rc.Ctx, rc.Param("id"), dockerclient.ContainerStopOptions{})
	if _, err := s.cli.ContainerRemove(rc.Ctx, rc.Param("id"), dockerclient.ContainerRemoveOptions{Force: true}); err != nil {
		return nil, DockerErr(err)
	}
	created, err := s.cli.ContainerCreate(rc.Ctx, dockerclient.ContainerCreateOptions{
		Config:           cfg,
		HostConfig:       host,
		NetworkingConfig: netCfg,
		Name:             name,
	})
	if err != nil {
		return nil, DockerErr(err)
	}
	if wasRunning {
		if _, err := s.cli.ContainerStart(rc.Ctx, created.ID, dockerclient.ContainerStartOptions{}); err != nil {
			return nil, DockerErr(err)
		}
	}
	return RedeployContainerResult{OK: true, ID: created.ID}, nil
}

func VolumeBackup(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	volumeName := strings.TrimSpace(rc.Param("id"))
	if err := validateResourceName(volumeName); err != nil {
		return nil, err
	}
	body, err := s.volumeBackupStream(rc.Ctx, volumeName)
	if err != nil {
		return nil, err
	}
	return &plugin.Download{
		Name:    volumeName + ".tar",
		MIME:    "application/x-tar",
		Size:    -1,
		ModTime: time.Now(),
		Body:    body,
	}, nil
}

func VolumeBackupURL(rc *plugin.RequestContext) (any, error) {
	return volumeBackupURL(rc, "docker.volume.backup")
}

func VolumeBackupURLFor(routeID string) plugin.Handler {
	return func(rc *plugin.RequestContext) (any, error) {
		return volumeBackupURL(rc, routeID)
	}
}

func volumeBackupURL(rc *plugin.RequestContext, routeID string) (any, error) {
	volumeName := strings.TrimSpace(rc.Param("id"))
	if err := validateResourceName(volumeName); err != nil {
		return nil, err
	}
	prefix := strings.TrimSuffix(rc.ProxyPrefix(), "/proxy")
	if prefix == "" {
		return nil, fmt.Errorf("%w: connection route prefix is unavailable", plugin.ErrInvalidInput)
	}
	q := url.Values{}
	q.Set("p.id", volumeName)
	return Row{"url": prefix + "/x/" + url.PathEscape(routeID) + "?" + q.Encode()}, nil
}

func (s *Session) volumeBackupStream(ctx context.Context, volumeName string) (io.ReadCloser, error) {
	imageName := "busybox:latest"
	created, err := s.cli.ContainerCreate(ctx, dockerclient.ContainerCreateOptions{
		Config: &container.Config{
			Image:        imageName,
			Cmd:          []string{"tar", "-C", "/data", "-cf", "-", "."},
			AttachStdout: true,
			AttachStderr: true,
		},
		HostConfig: &container.HostConfig{
			Mounts: []mount.Mount{{
				Type:     mount.TypeVolume,
				Source:   volumeName,
				Target:   "/data",
				ReadOnly: true,
			}},
		},
		Name: "",
	})
	if err != nil {
		return nil, DockerErr(err)
	}
	resp, err := s.cli.ContainerAttach(ctx, created.ID, dockerclient.ContainerAttachOptions{Stream: true, Stdout: true, Stderr: true})
	if err != nil {
		_, _ = s.cli.ContainerRemove(ctx, created.ID, dockerclient.ContainerRemoveOptions{Force: true})
		return nil, DockerErr(err)
	}
	wait := s.cli.ContainerWait(ctx, created.ID, dockerclient.ContainerWaitOptions{Condition: container.WaitConditionNextExit})
	if _, err := s.cli.ContainerStart(ctx, created.ID, dockerclient.ContainerStartOptions{}); err != nil {
		resp.Close()
		_, _ = s.cli.ContainerRemove(ctx, created.ID, dockerclient.ContainerRemoveOptions{Force: true})
		return nil, DockerErr(err)
	}
	pr, pw := io.Pipe()
	go func() {
		_, copyErr := stdcopy.StdCopy(pw, io.Discard, resp.Reader)
		resp.Close()
		var waitErr error
		var status container.WaitResponse
		select {
		case waitErr = <-wait.Error:
		case status = <-wait.Result:
		case <-ctx.Done():
			waitErr = ctx.Err()
		}
		_, _ = s.cli.ContainerRemove(ctx, created.ID, dockerclient.ContainerRemoveOptions{Force: true})
		if copyErr != nil {
			_ = pw.CloseWithError(copyErr)
			return
		}
		if waitErr != nil && ctx.Err() == nil {
			_ = pw.CloseWithError(waitErr)
			return
		}
		if status.StatusCode != 0 {
			_ = pw.CloseWithError(fmt.Errorf("%w: volume backup helper exited with status %d", plugin.ErrUnavailable, status.StatusCode))
			return
		}
		_ = pw.Close()
	}()
	return pr, nil
}

func DockerEventStream(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, err := sess(rc)
	if err != nil {
		return err
	}
	kind := strings.TrimSpace(rc.Param("kind"))
	if kind == "" {
		kind = strings.TrimSpace(rc.Query().Get("kind"))
	}
	filterType := dockerEventTypeForKind(kind)
	ctx, cancel := context.WithCancel(rc.Ctx)
	defer cancel()
	filters := make(dockerclient.Filters)
	if filterType != "" {
		filters = filters.Add("type", filterType)
	}
	result := s.cli.Events(ctx, dockerclient.EventsListOptions{Filters: filters})
	enc := json.NewEncoder(client)
	for {
		select {
		case <-client.Context().Done():
			return nil
		case err, ok := <-result.Err:
			if !ok {
				return nil
			}
			if err == nil || err == io.EOF || err == context.Canceled {
				return nil
			}
			return DockerErr(err)
		case msg, ok := <-result.Messages:
			if !ok {
				return nil
			}
			ev := resourceEventForDocker(rc.Ctx, s, kind, msg)
			if ev == nil {
				continue
			}
			if err := enc.Encode(ev); err != nil {
				return err
			}
		}
	}
}

func DockerTimelineWatch(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, err := sess(rc)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(rc.Ctx)
	defer cancel()
	result := s.cli.Events(ctx, dockerclient.EventsListOptions{})
	enc := json.NewEncoder(client)
	for {
		select {
		case <-client.Context().Done():
			return nil
		case err, ok := <-result.Err:
			if !ok {
				return nil
			}
			if err == nil || err == io.EOF || err == context.Canceled {
				return nil
			}
			return DockerErr(err)
		case msg, ok := <-result.Messages:
			if !ok {
				return nil
			}
			row := eventTimelineRow(msg)
			ref, _ := row["ref"].(plugin.ResourceIdentity)
			if err := enc.Encode(plugin.ResourceEvent{Type: "added", Ref: ref, Resource: row}); err != nil {
				return err
			}
		}
	}
}

func DockerObjectWatch(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, err := sess(rc)
	if err != nil {
		return err
	}
	kind := strings.TrimSpace(rc.Param("kind"))
	id := strings.TrimSpace(rc.Param("id"))
	if kind == "" || id == "" {
		return fmt.Errorf("%w: kind and id are required", plugin.ErrInvalidInput)
	}
	filterType := dockerEventTypeForKind(kind)
	ctx, cancel := context.WithCancel(rc.Ctx)
	defer cancel()
	filters := make(dockerclient.Filters)
	if filterType != "" {
		filters = filters.Add("type", filterType)
	}
	result := s.cli.Events(ctx, dockerclient.EventsListOptions{Filters: filters})
	enc := json.NewEncoder(client)
	for {
		select {
		case <-client.Context().Done():
			return nil
		case err, ok := <-result.Err:
			if !ok {
				return nil
			}
			if err == nil || err == io.EOF || err == context.Canceled {
				return nil
			}
			return DockerErr(err)
		case msg, ok := <-result.Messages:
			if !ok {
				return nil
			}
			if !eventMatchesObject(kind, id, msg) {
				continue
			}
			ev := objectEventForDocker(rc, s, kind, id, msg)
			if ev == nil {
				continue
			}
			if err := enc.Encode(ev); err != nil {
				return err
			}
		}
	}
}

func CommitContainerSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Commit", Fields: []plugin.Field{
		{Key: "reference", Label: "Image reference", Type: plugin.FieldText, Required: true, Placeholder: "repo/app:snapshot"},
		{Key: "comment", Label: "Comment", Type: plugin.FieldText},
		{Key: "author", Label: "Author", Type: plugin.FieldText},
		{Key: "no_pause", Label: "Do not pause during commit", Type: plugin.FieldToggle},
		{Key: "changes", Label: "Dockerfile changes", Type: plugin.FieldTextarea, Placeholder: "ENV DEBUG=true\nLABEL stage=snapshot"},
	}}}}
}

func RedeployContainerSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Redeploy", Fields: []plugin.Field{
		{Key: "pull", Label: "Pull image first", Type: plugin.FieldToggle, Default: true},
	}}}}
}

func ContainerStatsConfig() plugin.MetricsConfig {
	return plugin.MetricsConfig{
		Stats: []plugin.MetricStat{
			{Key: "cpuPct", Label: "CPU", Unit: "%"},
			{Key: "pids", Label: "PIDs"},
			{Key: "netRx", Label: "Net in", Unit: "bytes"},
			{Key: "netTx", Label: "Net out", Unit: "bytes"},
			{Key: "blockRead", Label: "Block read", Unit: "bytes"},
			{Key: "blockWrite", Label: "Block write", Unit: "bytes"},
		},
		Usage: []plugin.MetricUsage{
			{Key: "memPct", Label: "Memory usage", Type: plugin.ColumnPercent, Usage: &plugin.UsageSpec{PercentKey: "memPct", UsedKey: "memUsed", TotalKey: "memLimit", UsedType: plugin.ColumnBytes, TotalType: plugin.ColumnBytes, WarnAt: 80, CriticalAt: 95}},
		},
		Series: []plugin.MetricSeries{
			{Key: "cpuPct", Label: "CPU", Unit: "%"},
			{Key: "memPct", Label: "Memory", Unit: "%"},
		},
		History: 60,
	}
}

func EventsTimelineConfig(watch *plugin.DataSource) plugin.TimelineConfig {
	return plugin.TimelineConfig{
		TimestampField: "time",
		TitleField:     "title",
		BodyField:      "body",
		SeverityField:  "severity",
		IconField:      "icon",
		EmptyText:      "No Docker events found.",
		Watch:          watch,
	}
}

func ProcessColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "uid", Label: "UID", Sortable: true},
		{Key: "pid", Label: "PID", Sortable: true},
		{Key: "ppid", Label: "PPID", Sortable: true},
		{Key: "c", Label: "CPU", Sortable: true},
		{Key: "stime", Label: "Started", Sortable: true},
		{Key: "tty", Label: "TTY"},
		{Key: "time", Label: "Time"},
		{Key: "cmd", Label: "Command"},
	}
}

func ImageHistoryColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "id", Label: "Layer", Sortable: true},
		{Key: "createdAt", Label: "Created", Type: plugin.ColumnDateTime, Sortable: true},
		{Key: "size", Label: "Size", Type: plugin.ColumnBytes, Sortable: true},
		{Key: "tags", Label: "Tags"},
		{Key: "createdBy", Label: "Command"},
		{Key: "comment", Label: "Comment"},
	}
}

func NetworkPortColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "container", Label: "Container", Sortable: true},
		{Key: "image", Label: "Image", Sortable: true},
		{Key: "hostIP", Label: "Host"},
		{Key: "publicPort", Label: "Published", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "privatePort", Label: "Container", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "protocol", Label: "Protocol", Sortable: true},
		{Key: "published", Label: "Published", Type: plugin.ColumnBool, Sortable: true},
		{Key: "compose", Label: "Project", Sortable: true},
	}
}

func containerRowFromInspect(res dockerclient.ContainerInspectResult) Row {
	c := res.Container
	name := strings.TrimPrefix(c.Name, "/")
	if name == "" {
		name = shortID(c.ID)
	}
	row := Row{
		"id":        c.ID,
		"name":      name,
		"image":     c.Image,
		"createdAt": c.Created,
		"ref":       plugin.ResourceIdentity{Kind: "container", Name: name, UID: c.ID},
	}
	if c.Config != nil {
		row["image"] = stringDefault(c.Config.Image, fmt.Sprint(row["image"]))
		row["compose"] = c.Config.Labels[ComposeProjectLabel]
		row["composeService"] = c.Config.Labels["com.docker.compose.service"]
	}
	if c.State != nil {
		row["state"] = c.State.Status
		row["status"] = c.State.Status
	}
	if c.NetworkSettings != nil {
		row["ports"] = portMap(c.NetworkSettings.Ports)
	}
	return row
}

func networkRowFromInspect(res dockerclient.NetworkInspectResult) Row {
	n := res.Network
	return Row{
		"id":         n.ID,
		"name":       n.Name,
		"driver":     n.Driver,
		"scope":      n.Scope,
		"internal":   n.Internal,
		"attachable": n.Attachable,
		"ingress":    n.Ingress,
		"createdAt":  n.Created.String(),
		"compose":    n.Labels[ComposeProjectLabel],
		"ref":        plugin.ResourceIdentity{Kind: "network", Name: n.Name, UID: n.ID},
	}
}

func resourceEventForDocker(ctx context.Context, s *Session, kind string, msg events.Message) *plugin.ResourceEvent {
	switch kind {
	case eventKindContainer, "":
		return containerResourceEvent(ctx, s, msg)
	case eventKindImage:
		return imageResourceEvent(ctx, s, msg)
	case eventKindVolume:
		return volumeResourceEvent(ctx, s, msg)
	case eventKindNetwork:
		return networkResourceEvent(ctx, s, msg)
	case eventKindCompose:
		return composeResourceEvent(ctx, s, msg)
	default:
		return nil
	}
}

func containerResourceEvent(ctx context.Context, s *Session, msg events.Message) *plugin.ResourceEvent {
	id := msg.Actor.ID
	if id == "" {
		return nil
	}
	evType, state, ok := containerEventKind(msg.Action)
	if !ok {
		return nil
	}
	name := msg.Actor.Attributes["name"]
	if name == "" {
		name = shortID(id)
	}
	ref := plugin.ResourceIdentity{Kind: "container", Name: name, UID: id}
	if evType == "deleted" {
		return &plugin.ResourceEvent{Type: evType, Ref: ref}
	}
	inspect, err := s.cli.ContainerInspect(ctx, id, dockerclient.ContainerInspectOptions{})
	if err == nil {
		row := containerRowFromInspect(inspect)
		return &plugin.ResourceEvent{Type: evType, Ref: row["ref"].(plugin.ResourceIdentity), Resource: row}
	}
	resource := Row{"id": id, "name": name, "status": string(msg.Action), "ref": ref}
	if state != "" {
		resource["state"] = state
	}
	return &plugin.ResourceEvent{Type: evType, Ref: ref, Resource: resource}
}

func imageResourceEvent(ctx context.Context, s *Session, msg events.Message) *plugin.ResourceEvent {
	id := msg.Actor.ID
	if id == "" {
		return nil
	}
	name := stringDefault(msg.Actor.Attributes["name"], shortID(id))
	ref := plugin.ResourceIdentity{Kind: "image", Name: name, UID: id}
	if msg.Action == events.ActionDelete || msg.Action == events.ActionUnTag {
		return &plugin.ResourceEvent{Type: "deleted", Ref: ref}
	}
	// Source rows from the image list so live updates carry the same usage and
	// container count as the list (an inspect can't derive them). A burst of
	// events (a pull or a build) reuses one cached listing; only an id the
	// snapshot doesn't know about pays for a refresh.
	items, err := imageSummaries(ctx, s, false)
	if err != nil {
		return nil
	}
	img, ok := findImageSummary(items, id)
	if !ok {
		if items, err = imageSummaries(ctx, s, true); err != nil {
			return nil
		}
		if img, ok = findImageSummary(items, id); !ok {
			return nil
		}
	}
	row := imageRow(img)
	return &plugin.ResourceEvent{Type: "updated", Ref: row["ref"].(plugin.ResourceIdentity), Resource: row}
}

// imageListCache holds the engine's image listing per session for long enough
// to absorb the event burst a single pull or build emits.
var imageListCache = SessionCache[[]image.Summary]{TTL: 3 * time.Second}

func imageSummaries(ctx context.Context, s *Session, refresh bool) ([]image.Summary, error) {
	load := func() ([]image.Summary, error) {
		res, err := s.cli.ImageList(ctx, dockerclient.ImageListOptions{All: true})
		if err != nil {
			return nil, err
		}
		return res.Items, nil
	}
	if refresh {
		return imageListCache.Refresh(s, load)
	}
	return imageListCache.Get(s, load)
}

func findImageSummary(items []image.Summary, id string) (image.Summary, bool) {
	for _, img := range items {
		// Pull/push events set Actor.ID to a reference (e.g. "nginx:latest"), not the digest.
		if img.ID == id || slices.Contains(img.RepoTags, id) || slices.Contains(img.RepoDigests, id) {
			return img, true
		}
	}
	return image.Summary{}, false
}

func volumeResourceEvent(ctx context.Context, s *Session, msg events.Message) *plugin.ResourceEvent {
	name := stringDefault(msg.Actor.Attributes["name"], msg.Actor.ID)
	if name == "" {
		return nil
	}
	ref := plugin.ResourceIdentity{Kind: "volume", Name: name, UID: name}
	if msg.Action == events.ActionDestroy || msg.Action == events.ActionRemove {
		return &plugin.ResourceEvent{Type: "deleted", Ref: ref}
	}
	inspect, err := s.cli.VolumeInspect(ctx, name, dockerclient.VolumeInspectOptions{})
	if err != nil {
		return nil
	}
	refs, err := volumeRefCounts(ctx, s)
	if err != nil {
		return nil
	}
	row := volumeRowFromVolume(inspect.Volume, refs, volumeSizes(ctx, s))
	return &plugin.ResourceEvent{Type: "updated", Ref: row["ref"].(plugin.ResourceIdentity), Resource: row}
}

func networkResourceEvent(ctx context.Context, s *Session, msg events.Message) *plugin.ResourceEvent {
	id := msg.Actor.ID
	if id == "" {
		return nil
	}
	name := stringDefault(msg.Actor.Attributes["name"], shortID(id))
	ref := plugin.ResourceIdentity{Kind: "network", Name: name, UID: id}
	if msg.Action == events.ActionDestroy || msg.Action == events.ActionRemove {
		return &plugin.ResourceEvent{Type: "deleted", Ref: ref}
	}
	inspect, err := s.cli.NetworkInspect(ctx, id, dockerclient.NetworkInspectOptions{})
	if err != nil {
		return nil
	}
	row := networkRowFromInspect(inspect)
	return &plugin.ResourceEvent{Type: "updated", Ref: row["ref"].(plugin.ResourceIdentity), Resource: row}
}

func composeResourceEvent(ctx context.Context, s *Session, msg events.Message) *plugin.ResourceEvent {
	project := msg.Actor.Attributes[ComposeProjectLabel]
	if project == "" {
		return nil
	}
	ref := plugin.ResourceIdentity{Kind: "compose", Name: project, UID: project}
	row, err := composeRowForProject(ctx, s, project)
	switch {
	case errors.Is(err, plugin.ErrNotFound):
		return &plugin.ResourceEvent{Type: "deleted", Ref: ref}
	case err != nil:
		return nil
	}
	return &plugin.ResourceEvent{Type: "updated", Ref: ref, Resource: row}
}

func objectEventForDocker(rc *plugin.RequestContext, s *Session, kind, id string, msg events.Message) *plugin.ResourceEvent {
	ref := plugin.ResourceIdentity{Kind: kind, Name: id, UID: id}
	// A Compose project is derived from its members, so one member's destroy isn't
	// the project's deletion; recompute and let composeOverviewForProject decide.
	if kind != eventKindCompose && (msg.Action == events.ActionDestroy || msg.Action == events.ActionRemove || msg.Action == events.ActionDelete) {
		return &plugin.ResourceEvent{Type: "deleted", Ref: ref}
	}
	var resource any
	var err error
	switch kind {
	case eventKindContainer:
		resource, err = containerOverviewForID(rc.Ctx, s, id)
	case eventKindImage:
		resource, err = imageOverviewForID(rc.Ctx, s, id)
	case eventKindVolume:
		resource, err = volumeOverviewForID(rc.Ctx, s, id)
	case eventKindNetwork:
		resource, err = networkOverviewForID(rc.Ctx, s, id)
	case eventKindCompose:
		resource, err = composeOverviewForProject(rc.Ctx, s, id)
		if errors.Is(err, plugin.ErrNotFound) {
			return &plugin.ResourceEvent{Type: "deleted", Ref: ref}
		}
	}
	if err != nil || resource == nil {
		return nil
	}
	return &plugin.ResourceEvent{Type: "updated", Ref: ref, Resource: resource}
}

func eventMatchesObject(kind, id string, msg events.Message) bool {
	switch kind {
	case eventKindCompose:
		return msg.Actor.Attributes[ComposeProjectLabel] == id
	default:
		return msg.Actor.ID == id || msg.Actor.Attributes["name"] == id
	}
}

func dockerEventTypeForKind(kind string) string {
	switch kind {
	case eventKindImage:
		return string(events.ImageEventType)
	case eventKindVolume:
		return string(events.VolumeEventType)
	case eventKindNetwork:
		return string(events.NetworkEventType)
	case eventKindContainer, eventKindCompose, "":
		return string(events.ContainerEventType)
	default:
		return ""
	}
}

func eventTimelineRow(msg events.Message) Row {
	name := stringDefault(msg.Actor.Attributes["name"], msg.Actor.ID)
	title := strings.TrimSpace(string(msg.Type) + " " + string(msg.Action))
	body := name
	if project := msg.Actor.Attributes[ComposeProjectLabel]; project != "" {
		body = body + " - project " + project
	}
	return Row{
		"time":     time.Unix(msg.Time, 0).UTC().Format(time.RFC3339),
		"title":    title,
		"body":     body,
		"severity": eventSeverity(msg),
		"icon":     eventIcon(msg),
		"resource": name,
		"ref":      plugin.ResourceIdentity{Kind: "event", Name: title, UID: fmt.Sprintf("%d:%s:%s:%s", msg.TimeNano, msg.Type, msg.Action, msg.Actor.ID)},
	}
}

func sortTimeline(rows []Row) {
	sort.SliceStable(rows, func(i, j int) bool {
		return fmt.Sprint(rows[i]["time"]) > fmt.Sprint(rows[j]["time"])
	})
}

func eventSeverity(msg events.Message) plugin.Severity {
	switch msg.Action {
	case events.ActionDie, events.ActionKill, events.ActionOOM, events.ActionDestroy, events.ActionDelete, events.ActionRemove:
		return plugin.SeverityDanger
	case events.ActionStop, events.ActionPause, events.ActionRestart:
		return plugin.SeverityWarn
	case events.ActionStart, events.ActionCreate, events.ActionPull, events.ActionPush, events.ActionTag, events.ActionConnect:
		return plugin.SeveritySuccess
	default:
		return plugin.SeverityInfo
	}
}

func eventIcon(msg events.Message) string {
	switch msg.Type {
	case events.ContainerEventType:
		return "box"
	case events.ImageEventType:
		return "layers"
	case events.VolumeEventType:
		return "database"
	case events.NetworkEventType:
		return "globe"
	default:
		return "activity"
	}
}

func statsFrame(sample, prev container.StatsResponse) map[string]any {
	frame := map[string]any{"metricsAvailable": true}
	cpuPct := cpuPercent(sample, prev)
	memUsed := memoryUsage(sample.MemoryStats)
	memLimit := sample.MemoryStats.Limit
	memPct := 0.0
	if memLimit > 0 {
		memPct = float64(memUsed) / float64(memLimit) * 100
	}
	rx, tx := networkBytes(sample.Networks)
	read, write := blockBytes(sample.BlkioStats.IoServiceBytesRecursive)
	frame["cpuPct"] = round(cpuPct)
	frame["memUsed"] = memUsed
	frame["memLimit"] = memLimit
	frame["memPct"] = round(memPct)
	frame["pids"] = sample.PidsStats.Current
	frame["netRx"] = rx
	frame["netTx"] = tx
	frame["blockRead"] = read
	frame["blockWrite"] = write
	return frame
}

func cpuPercent(sample, prev container.StatsResponse) float64 {
	prevCPU := prev.CPUStats.CPUUsage.TotalUsage
	prevSystem := prev.CPUStats.SystemUsage
	if !sample.PreRead.IsZero() {
		prevCPU = sample.PreCPUStats.CPUUsage.TotalUsage
		prevSystem = sample.PreCPUStats.SystemUsage
	}
	cpuDelta := float64(sample.CPUStats.CPUUsage.TotalUsage - prevCPU)
	systemDelta := float64(sample.CPUStats.SystemUsage - prevSystem)
	online := float64(sample.CPUStats.OnlineCPUs)
	if online == 0 {
		online = float64(len(sample.CPUStats.CPUUsage.PercpuUsage))
	}
	if cpuDelta <= 0 || systemDelta <= 0 || online <= 0 {
		return 0
	}
	return (cpuDelta / systemDelta) * online * 100
}

func memoryUsage(stats container.MemoryStats) uint64 {
	usage := stats.Usage
	if cache, ok := stats.Stats["cache"]; ok && usage > cache {
		return usage - cache
	}
	return usage
}

func networkBytes(networks map[string]container.NetworkStats) (uint64, uint64) {
	var rx, tx uint64
	for _, n := range networks {
		rx += n.RxBytes
		tx += n.TxBytes
	}
	return rx, tx
}

func blockBytes(entries []container.BlkioStatEntry) (uint64, uint64) {
	var read, write uint64
	for _, e := range entries {
		switch strings.ToLower(e.Op) {
		case "read":
			read += e.Value
		case "write":
			write += e.Value
		}
	}
	return read, write
}

func round(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return math.Round(v*100) / 100
}

func processKey(title string) string {
	key := strings.ToLower(strings.TrimSpace(title))
	key = strings.ReplaceAll(key, "%", "")
	key = strings.ReplaceAll(key, "/", "_")
	key = strings.ReplaceAll(key, " ", "_")
	if key == "command" {
		return "cmd"
	}
	return key
}

func cloneContainerConfig(cfg *container.Config) *container.Config {
	if cfg == nil {
		return &container.Config{}
	}
	next := *cfg
	next.Env = append([]string(nil), cfg.Env...)
	next.Cmd = append([]string(nil), cfg.Cmd...)
	next.Entrypoint = append([]string(nil), cfg.Entrypoint...)
	next.Labels = cloneMap(cfg.Labels)
	return &next
}

func cloneHostConfig(cfg *container.HostConfig) *container.HostConfig {
	if cfg == nil {
		return &container.HostConfig{}
	}
	next := *cfg
	next.Binds = append([]string(nil), cfg.Binds...)
	next.Mounts = append([]mount.Mount(nil), cfg.Mounts...)
	return &next
}

func cloneNetworkingConfig(ns *container.NetworkSettings) *network.NetworkingConfig {
	if ns == nil || len(ns.Networks) == 0 {
		return nil
	}
	endpoints := make(map[string]*network.EndpointSettings, len(ns.Networks))
	for name, ep := range ns.Networks {
		if ep == nil {
			endpoints[name] = &network.EndpointSettings{}
			continue
		}
		next := *ep
		endpoints[name] = &next
	}
	return &network.NetworkingConfig{EndpointsConfig: endpoints}
}

func cloneMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func containerOverviewForID(ctx context.Context, s *Session, id string) (Row, error) {
	res, err := s.cli.ContainerInspect(ctx, id, dockerclient.ContainerInspectOptions{})
	if err != nil {
		return nil, DockerErr(err)
	}
	out := Row{
		"id":           shortID(res.Container.ID),
		"name":         strings.TrimPrefix(res.Container.Name, "/"),
		"image":        res.Container.Image,
		"created":      res.Container.Created,
		"command":      strings.TrimSpace(res.Container.Path + " " + strings.Join(res.Container.Args, " ")),
		"restartCount": res.Container.RestartCount,
		"driver":       res.Container.Driver,
		"platform":     res.Container.Platform,
		"mounts":       len(res.Container.Mounts),
	}
	if res.Container.Config != nil {
		out["image"] = stringDefault(res.Container.Config.Image, fmt.Sprint(out["image"]))
		out["entrypoint"] = strings.Join(res.Container.Config.Entrypoint, " ")
		out["env"] = len(res.Container.Config.Env)
		out["labels"] = res.Container.Config.Labels
		out["composeProject"] = res.Container.Config.Labels[ComposeProjectLabel]
		out["composeService"] = res.Container.Config.Labels["com.docker.compose.service"]
	}
	if res.Container.HostConfig != nil {
		out["restartPolicy"] = res.Container.HostConfig.RestartPolicy.Name
		out["networkMode"] = string(res.Container.HostConfig.NetworkMode)
	}
	if res.Container.State != nil {
		out["state"] = res.Container.State.Status
		out["status"] = res.Container.State.Status
		out["running"] = res.Container.State.Running
		out["startedAt"] = res.Container.State.StartedAt
		out["finishedAt"] = res.Container.State.FinishedAt
		out["exitCode"] = res.Container.State.ExitCode
		if res.Container.State.Health != nil {
			out["health"] = res.Container.State.Health.Status
		}
	}
	if res.Container.NetworkSettings != nil {
		out["networks"] = mapKeys(res.Container.NetworkSettings.Networks)
		out["ports"] = portMap(res.Container.NetworkSettings.Ports)
	}
	return out, nil
}

func imageOverviewForID(ctx context.Context, s *Session, id string) (Row, error) {
	res, err := s.cli.ImageInspect(ctx, id)
	if err != nil {
		return nil, DockerErr(err)
	}
	return pickStruct(res, "Id", "RepoTags", "RepoDigests", "Created", "Size", "VirtualSize", "Architecture", "Os", "DockerVersion", "Author", "Comment"), nil
}

func volumeOverviewForID(ctx context.Context, s *Session, id string) (Row, error) {
	res, err := s.cli.VolumeInspect(ctx, id, dockerclient.VolumeInspectOptions{})
	if err != nil {
		return nil, DockerErr(err)
	}
	refs, err := volumeRefCounts(ctx, s)
	if err != nil {
		return nil, DockerErr(err)
	}
	return volumeOverviewRow(res.Volume, refs, volumeSizes(ctx, s)), nil
}

func networkOverviewForID(ctx context.Context, s *Session, id string) (Row, error) {
	res, err := s.cli.NetworkInspect(ctx, id, dockerclient.NetworkInspectOptions{})
	if err != nil {
		return nil, DockerErr(err)
	}
	n := res.Network
	return Row{
		"id":         shortID(n.ID),
		"name":       n.Name,
		"driver":     n.Driver,
		"scope":      n.Scope,
		"created":    n.Created.Format(time.RFC3339),
		"internal":   n.Internal,
		"attachable": n.Attachable,
		"ingress":    n.Ingress,
		"containers": len(n.Containers),
		"services":   len(n.Services),
		"labels":     n.Labels,
	}, nil
}

func composeOverviewForProject(ctx context.Context, s *Session, project string) (Row, error) {
	return composeRowForProject(ctx, s, project)
}

// composeRowForProject resolves one project's row from a label-filtered container
// list, so a per-container Compose event costs only that project's members.
func composeRowForProject(ctx context.Context, s *Session, project string) (Row, error) {
	if project == "" {
		return nil, plugin.ErrNotFound
	}
	res, err := s.cli.ContainerList(ctx, dockerclient.ContainerListOptions{
		All:     true,
		Filters: make(dockerclient.Filters).Add("label", ComposeProjectLabel+"="+project),
	})
	if err != nil {
		return nil, DockerErr(err)
	}
	for _, r := range composeRowsFromContainers(res.Items) {
		if r["name"] == project {
			return r, nil
		}
	}
	return nil, plugin.ErrNotFound
}

func composeRowsForSession(ctx context.Context, s *Session) ([]Row, error) {
	res, err := s.cli.ContainerList(ctx, dockerclient.ContainerListOptions{All: true})
	if err != nil {
		return nil, DockerErr(err)
	}
	return composeRowsFromContainers(res.Items), nil
}

func composeRowsFromContainers(items []container.Summary) []Row {
	projects := map[string]Row{}
	services := map[string]map[string]bool{}
	for _, c := range items {
		project := c.Labels[ComposeProjectLabel]
		if project == "" {
			continue
		}
		r, ok := projects[project]
		if !ok {
			r = Row{
				"name":       project,
				"workingDir": c.Labels["com.docker.compose.project.working_dir"],
				"config":     c.Labels["com.docker.compose.project.config_files"],
				"containers": 0,
				"running":    0,
				"services":   0,
				"status":     "Stopped",
				"ref":        plugin.ResourceIdentity{Kind: "compose", Name: project, UID: project},
			}
			projects[project] = r
		}
		if services[project] == nil {
			services[project] = map[string]bool{}
		}
		if service := c.Labels["com.docker.compose.service"]; service != "" {
			services[project][service] = true
			r["services"] = len(services[project])
		}
		r["containers"] = r["containers"].(int) + 1
		if c.State == "running" {
			r["running"] = r["running"].(int) + 1
		}
		r["status"] = composeStatus(r["running"].(int), r["containers"].(int))
	}
	rows := make([]Row, 0, len(projects))
	for _, r := range projects {
		rows = append(rows, r)
	}
	return rows
}
