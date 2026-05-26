package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	shellquote "github.com/kballard/go-shellquote"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/volume"
	dockerclient "github.com/moby/moby/client"

	"github.com/charlesng/shellcn/internal/plugin"
)

type row map[string]any

type actionResult struct {
	OK bool `json:"ok"`
}

type createContainerResult struct {
	OK       bool     `json:"ok"`
	ID       string   `json:"id"`
	Name     string   `json:"name,omitempty"`
	Started  bool     `json:"started"`
	Warnings []string `json:"warnings,omitempty"`
}

type createContainerRequest struct {
	Name           string `json:"name"`
	Image          string `json:"image" validate:"required"`
	Pull           *bool  `json:"pull"`
	Start          *bool  `json:"start"`
	Command        string `json:"command"`
	Entrypoint     string `json:"entrypoint"`
	User           string `json:"user"`
	WorkingDir     string `json:"working_dir"`
	Env            string `json:"env"`
	Ports          string `json:"ports"`
	Binds          string `json:"binds"`
	Network        string `json:"network"`
	Restart        string `json:"restart"`
	RestartRetries int    `json:"restart_retries"`
	TTY            bool   `json:"tty"`
	OpenStdin      bool   `json:"open_stdin"`
}

func Routes() []plugin.Route {
	return []plugin.Route{
		{ID: "docker.containers.tree", Method: plugin.MethodGet, Path: "/tree/containers", Permission: "docker.containers.read", Risk: plugin.RiskSafe, AuditEvent: "docker.containers.tree", Handle: treeContainers},
		{ID: "docker.images.tree", Method: plugin.MethodGet, Path: "/tree/images", Permission: "docker.images.read", Risk: plugin.RiskSafe, AuditEvent: "docker.images.tree", Handle: treeImages},
		{ID: "docker.volumes.tree", Method: plugin.MethodGet, Path: "/tree/volumes", Permission: "docker.volumes.read", Risk: plugin.RiskSafe, AuditEvent: "docker.volumes.tree", Handle: treeVolumes},
		{ID: "docker.networks.tree", Method: plugin.MethodGet, Path: "/tree/networks", Permission: "docker.networks.read", Risk: plugin.RiskSafe, AuditEvent: "docker.networks.tree", Handle: treeNetworks},
		{ID: "docker.compose.tree", Method: plugin.MethodGet, Path: "/tree/compose", Permission: "docker.compose.read", Risk: plugin.RiskSafe, AuditEvent: "docker.compose.tree", Handle: treeCompose},
		{ID: "docker.containers.list", Method: plugin.MethodGet, Path: "/containers", Permission: "docker.containers.read", Risk: plugin.RiskSafe, AuditEvent: "docker.containers.list", Handle: listContainers},
		{ID: "docker.images.list", Method: plugin.MethodGet, Path: "/images", Permission: "docker.images.read", Risk: plugin.RiskSafe, AuditEvent: "docker.images.list", Handle: listImages},
		{ID: "docker.volumes.list", Method: plugin.MethodGet, Path: "/volumes", Permission: "docker.volumes.read", Risk: plugin.RiskSafe, AuditEvent: "docker.volumes.list", Handle: listVolumes},
		{ID: "docker.networks.list", Method: plugin.MethodGet, Path: "/networks", Permission: "docker.networks.read", Risk: plugin.RiskSafe, AuditEvent: "docker.networks.list", Handle: listNetworks},
		{ID: "docker.compose.list", Method: plugin.MethodGet, Path: "/compose", Permission: "docker.compose.read", Risk: plugin.RiskSafe, AuditEvent: "docker.compose.list", Handle: listCompose},
		{ID: "docker.container.overview", Method: plugin.MethodGet, Path: "/containers/{id}/overview", Permission: "docker.containers.read", Risk: plugin.RiskSafe, AuditEvent: "docker.container.overview", Handle: containerOverview},
		{ID: "docker.image.overview", Method: plugin.MethodGet, Path: "/images/{id}/overview", Permission: "docker.images.read", Risk: plugin.RiskSafe, AuditEvent: "docker.image.overview", Handle: imageOverview},
		{ID: "docker.volume.overview", Method: plugin.MethodGet, Path: "/volumes/{id}/overview", Permission: "docker.volumes.read", Risk: plugin.RiskSafe, AuditEvent: "docker.volume.overview", Handle: volumeOverview},
		{ID: "docker.network.overview", Method: plugin.MethodGet, Path: "/networks/{id}/overview", Permission: "docker.networks.read", Risk: plugin.RiskSafe, AuditEvent: "docker.network.overview", Handle: networkOverview},
		{ID: "docker.compose.overview", Method: plugin.MethodGet, Path: "/compose/{project}/overview", Permission: "docker.compose.read", Risk: plugin.RiskSafe, AuditEvent: "docker.compose.overview", Handle: composeOverview},
		{ID: "docker.compose.containers", Method: plugin.MethodGet, Path: "/compose/{project}/containers", Permission: "docker.compose.read", Risk: plugin.RiskSafe, AuditEvent: "docker.compose.containers", Handle: composeContainers},
		{ID: "docker.compose.services", Method: plugin.MethodGet, Path: "/compose/{project}/services", Permission: "docker.compose.read", Risk: plugin.RiskSafe, AuditEvent: "docker.compose.services", Handle: composeServices},
		{ID: "docker.container.inspect", Method: plugin.MethodGet, Path: "/containers/{id}/inspect", Permission: "docker.containers.read", Risk: plugin.RiskSafe, AuditEvent: "docker.container.inspect", Handle: inspectContainer},
		{ID: "docker.image.inspect", Method: plugin.MethodGet, Path: "/images/{id}/inspect", Permission: "docker.images.read", Risk: plugin.RiskSafe, AuditEvent: "docker.image.inspect", Handle: inspectImage},
		{ID: "docker.volume.inspect", Method: plugin.MethodGet, Path: "/volumes/{id}/inspect", Permission: "docker.volumes.read", Risk: plugin.RiskSafe, AuditEvent: "docker.volume.inspect", Handle: inspectVolume},
		{ID: "docker.network.inspect", Method: plugin.MethodGet, Path: "/networks/{id}/inspect", Permission: "docker.networks.read", Risk: plugin.RiskSafe, AuditEvent: "docker.network.inspect", Handle: inspectNetwork},
		{ID: "docker.container.env", Method: plugin.MethodGet, Path: "/containers/{id}/env", Permission: "docker.containers.read", Risk: plugin.RiskSafe, AuditEvent: "docker.container.env", Handle: containerEnv},
		{ID: "docker.container.create", Method: plugin.MethodPost, Path: "/containers", Permission: "docker.containers.write", Risk: plugin.RiskWrite, AuditEvent: "docker.container.create", Input: createContainerSchema(), Handle: createContainer},
		{ID: "docker.container.start", Method: plugin.MethodPost, Path: "/containers/{id}/start", Permission: "docker.containers.write", Risk: plugin.RiskWrite, AuditEvent: "docker.container.start", Handle: startContainer},
		{ID: "docker.container.stop", Method: plugin.MethodPost, Path: "/containers/{id}/stop", Permission: "docker.containers.write", Risk: plugin.RiskWrite, AuditEvent: "docker.container.stop", Handle: stopContainer},
		{ID: "docker.container.restart", Method: plugin.MethodPost, Path: "/containers/{id}/restart", Permission: "docker.containers.write", Risk: plugin.RiskWrite, AuditEvent: "docker.container.restart", Handle: restartContainer},
		{ID: "docker.container.remove", Method: plugin.MethodDelete, Path: "/containers/{id}", Permission: "docker.containers.delete", Risk: plugin.RiskDestructive, AuditEvent: "docker.container.remove", Handle: removeContainer},
		{ID: "docker.image.remove", Method: plugin.MethodDelete, Path: "/images/{id}", Permission: "docker.images.delete", Risk: plugin.RiskDestructive, AuditEvent: "docker.image.remove", Handle: removeImage},
		{ID: "docker.volume.remove", Method: plugin.MethodDelete, Path: "/volumes/{id}", Permission: "docker.volumes.delete", Risk: plugin.RiskDestructive, AuditEvent: "docker.volume.remove", Handle: removeVolume},
		{ID: "docker.network.remove", Method: plugin.MethodDelete, Path: "/networks/{id}", Permission: "docker.networks.delete", Risk: plugin.RiskDestructive, AuditEvent: "docker.network.remove", Handle: removeNetwork},
		{ID: "docker.container.exec.prepare", Method: plugin.MethodPost, Path: "/containers/{id}/exec", Permission: "docker.containers.exec", Risk: plugin.RiskPrivileged, AuditEvent: "docker.container.exec.prepare", Handle: prepareExec},
		{ID: "docker.container.logs", Method: plugin.MethodWS, Path: "/containers/{id}/logs/{tail}/{follow}/{timestamps}", Permission: "docker.containers.logs", Risk: plugin.RiskSafe, AuditEvent: "docker.container.logs", Input: logsSchema(), Stream: logsStream},
		{ID: "docker.container.exec", Method: plugin.MethodWS, Path: "/containers/{id}/exec/ws/{cols}/{rows}/{command}", Permission: "docker.containers.exec", Risk: plugin.RiskPrivileged, AuditEvent: "docker.container.exec", Input: execSchema(), Stream: execStream},
		{ID: "docker.events.watch", Method: plugin.MethodWS, Path: "/events", Permission: "docker.events.read", Risk: plugin.RiskSafe, AuditEvent: "docker.events.watch", Stream: watchEvents},
		{ID: "docker.api.execute", Method: plugin.MethodPost, Path: "/api/execute", Permission: "docker.api.execute", Risk: plugin.RiskPrivileged, AuditEvent: "docker.api.execute", Input: apiSchema(), Handle: executeAPI},
	}
}

func sess(rc *plugin.RequestContext) (*Session, error) {
	return Unwrap(rc.Session)
}

func listContainers(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.ContainerList(rc.Ctx, dockerclient.ContainerListOptions{All: true})
	if err != nil {
		return nil, dockerErr(err)
	}
	return pageRows(rc, containerRows(res.Items))
}

func listImages(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.ImageList(rc.Ctx, dockerclient.ImageListOptions{All: true})
	if err != nil {
		return nil, dockerErr(err)
	}
	return pageRows(rc, imageRows(res.Items))
}

func listVolumes(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.VolumeList(rc.Ctx, dockerclient.VolumeListOptions{})
	if err != nil {
		return nil, dockerErr(err)
	}
	return pageRows(rc, volumeRows(res.Items))
}

func listNetworks(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.NetworkList(rc.Ctx, dockerclient.NetworkListOptions{})
	if err != nil {
		return nil, dockerErr(err)
	}
	return pageRows(rc, networkRows(res.Items))
}

func listCompose(rc *plugin.RequestContext) (any, error) {
	rows, err := composeRows(rc)
	if err != nil {
		return nil, err
	}
	return pageRows(rc, rows)
}

func treeContainers(rc *plugin.RequestContext) (any, error) {
	return treeFromRows(rc, "container", listContainers)
}
func treeImages(rc *plugin.RequestContext) (any, error) { return treeFromRows(rc, "image", listImages) }
func treeVolumes(rc *plugin.RequestContext) (any, error) {
	return treeFromRows(rc, "volume", listVolumes)
}

func treeNetworks(rc *plugin.RequestContext) (any, error) {
	return treeFromRows(rc, "network", listNetworks)
}

func treeCompose(rc *plugin.RequestContext) (any, error) {
	rows, err := composeRows(rc)
	if err != nil {
		return nil, err
	}
	nodes := make([]plugin.TreeNode, 0, len(rows))
	for _, r := range rows {
		name, _ := r["name"].(string)
		if name == "" {
			continue
		}
		nodes = append(nodes, plugin.TreeNode{
			Key:   "compose:" + name,
			Label: name,
			Icon:  plugin.Icon{Type: plugin.IconLucide, Value: "workflow"},
			Ref:   &plugin.ResourceRef{Kind: "compose", Name: name, UID: name},
			Leaf:  true,
			Badge: &plugin.Badge{Value: r["containers"], Severity: plugin.SeverityInfo},
		})
	}
	return plugin.Page[plugin.TreeNode]{Items: nodes, Total: ptr(len(nodes))}, nil
}

func containerOverview(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.ContainerInspect(rc.Ctx, rc.Param("id"), dockerclient.ContainerInspectOptions{})
	if err != nil {
		return nil, dockerErr(err)
	}
	out := row{
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
		out["composeProject"] = res.Container.Config.Labels["com.docker.compose.project"]
		out["composeService"] = res.Container.Config.Labels["com.docker.compose.service"]
	}
	if res.Container.State != nil {
		out["state"] = res.Container.State.Status
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
	}
	return out, nil
}

func imageOverview(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.ImageInspect(rc.Ctx, rc.Param("id"))
	if err != nil {
		return nil, dockerErr(err)
	}
	return pickStruct(res, "Id", "RepoTags", "RepoDigests", "Created", "Size", "VirtualSize", "Architecture", "Os", "DockerVersion", "Author", "Comment"), nil
}

func volumeOverview(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.VolumeInspect(rc.Ctx, rc.Param("id"), dockerclient.VolumeInspectOptions{})
	if err != nil {
		return nil, dockerErr(err)
	}
	out := row{
		"name":       res.Volume.Name,
		"driver":     res.Volume.Driver,
		"scope":      res.Volume.Scope,
		"mountpoint": res.Volume.Mountpoint,
		"createdAt":  res.Volume.CreatedAt,
		"labels":     res.Volume.Labels,
		"options":    res.Volume.Options,
	}
	if res.Volume.UsageData != nil {
		out["size"] = res.Volume.UsageData.Size
		out["refs"] = res.Volume.UsageData.RefCount
	}
	return out, nil
}

func networkOverview(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.NetworkInspect(rc.Ctx, rc.Param("id"), dockerclient.NetworkInspectOptions{})
	if err != nil {
		return nil, dockerErr(err)
	}
	n := res.Network
	return row{
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

func composeOverview(rc *plugin.RequestContext) (any, error) {
	rows, err := composeRows(rc)
	if err != nil {
		return nil, err
	}
	project := rc.Param("project")
	for _, r := range rows {
		if r["name"] == project {
			return r, nil
		}
	}
	return nil, plugin.ErrNotFound
}

func composeContainers(rc *plugin.RequestContext) (any, error) {
	rows, err := containersForCompose(rc, rc.Param("project"))
	if err != nil {
		return nil, err
	}
	return pageRows(rc, rows)
}

func composeServices(rc *plugin.RequestContext) (any, error) {
	containers, err := containersForCompose(rc, rc.Param("project"))
	if err != nil {
		return nil, err
	}
	services := map[string]row{}
	for _, c := range containers {
		name, _ := c["composeService"].(string)
		if name == "" {
			name = "(default)"
		}
		r, ok := services[name]
		if !ok {
			r = row{"name": name, "image": c["image"], "containers": 0, "running": 0, "ports": c["ports"]}
			services[name] = r
		}
		r["containers"] = r["containers"].(int) + 1
		if c["state"] == "running" {
			r["running"] = r["running"].(int) + 1
		}
		if r["ports"] == "" {
			r["ports"] = c["ports"]
		}
	}
	rows := make([]row, 0, len(services))
	for _, r := range services {
		rows = append(rows, r)
	}
	return pageRows(rc, rows)
}

func inspectContainer(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.ContainerInspect(rc.Ctx, rc.Param("id"), dockerclient.ContainerInspectOptions{})
	if err != nil {
		return nil, dockerErr(err)
	}
	return rawOrValue(res.Raw, res.Container)
}

func inspectImage(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.ImageInspect(rc.Ctx, rc.Param("id"))
	if err != nil {
		return nil, dockerErr(err)
	}
	return res, nil
}

func inspectVolume(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.VolumeInspect(rc.Ctx, rc.Param("id"), dockerclient.VolumeInspectOptions{})
	if err != nil {
		return nil, dockerErr(err)
	}
	return rawOrValue(res.Raw, res.Volume)
}

func inspectNetwork(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.NetworkInspect(rc.Ctx, rc.Param("id"), dockerclient.NetworkInspectOptions{})
	if err != nil {
		return nil, dockerErr(err)
	}
	return rawOrValue(res.Raw, res.Network)
}

func containerEnv(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.ContainerInspect(rc.Ctx, rc.Param("id"), dockerclient.ContainerInspectOptions{})
	if err != nil {
		return nil, dockerErr(err)
	}
	var rows []row
	if res.Container.Config != nil {
		for _, env := range res.Container.Config.Env {
			key, value, _ := strings.Cut(env, "=")
			rows = append(rows, row{
				"key":   key,
				"value": value,
				"ref":   plugin.ResourceRef{Kind: "env", Name: key, UID: key},
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool { return fmt.Sprint(rows[i]["key"]) < fmt.Sprint(rows[j]["key"]) })
	return pageRows(rc, rows)
}

func createContainer(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	var in createContainerRequest
	if err := rc.Bind(&in); err != nil {
		return nil, err
	}
	in.Image = strings.TrimSpace(in.Image)
	if in.Image == "" {
		return nil, fmt.Errorf("%w: image is required", plugin.ErrInvalidInput)
	}
	env, err := parseEnvLines(in.Env)
	if err != nil {
		return nil, err
	}
	exposed, bindings, err := parsePublishedPorts(in.Ports)
	if err != nil {
		return nil, err
	}
	restart, err := parseRestartPolicy(in.Restart, in.RestartRetries)
	if err != nil {
		return nil, err
	}
	if boolDefault(in.Pull, true) {
		pull, err := s.cli.ImagePull(rc.Ctx, in.Image, dockerclient.ImagePullOptions{})
		if err != nil {
			return nil, dockerErr(err)
		}
		if err := pull.Wait(rc.Ctx); err != nil {
			_ = pull.Close()
			return nil, dockerErr(err)
		}
		if err := pull.Close(); err != nil {
			return nil, dockerErr(err)
		}
	}
	cmd, err := shellFields("command", in.Command)
	if err != nil {
		return nil, err
	}
	entrypoint, err := shellFields("entrypoint", in.Entrypoint)
	if err != nil {
		return nil, err
	}
	cfg := &container.Config{
		Image:        in.Image,
		Cmd:          cmd,
		Entrypoint:   entrypoint,
		Env:          env,
		ExposedPorts: exposed,
		User:         strings.TrimSpace(in.User),
		WorkingDir:   strings.TrimSpace(in.WorkingDir),
		Tty:          in.TTY,
		OpenStdin:    in.OpenStdin,
	}
	host := &container.HostConfig{
		Binds:         nonEmptyLines(in.Binds),
		PortBindings:  bindings,
		RestartPolicy: restart,
	}
	var networking *network.NetworkingConfig
	if n := strings.TrimSpace(in.Network); n != "" {
		host.NetworkMode = container.NetworkMode(n)
		networking = &network.NetworkingConfig{EndpointsConfig: map[string]*network.EndpointSettings{n: {}}}
	}
	created, err := s.cli.ContainerCreate(rc.Ctx, dockerclient.ContainerCreateOptions{
		Config:           cfg,
		HostConfig:       host,
		NetworkingConfig: networking,
		Name:             strings.TrimSpace(in.Name),
	})
	if err != nil {
		return nil, dockerErr(err)
	}
	started := false
	if boolDefault(in.Start, true) {
		if _, err := s.cli.ContainerStart(rc.Ctx, created.ID, dockerclient.ContainerStartOptions{}); err != nil {
			return nil, dockerErr(err)
		}
		started = true
	}
	return createContainerResult{
		OK:       true,
		ID:       shortID(created.ID),
		Name:     strings.TrimSpace(in.Name),
		Started:  started,
		Warnings: created.Warnings,
	}, nil
}

func startContainer(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	_, err = s.cli.ContainerStart(rc.Ctx, rc.Param("id"), dockerclient.ContainerStartOptions{})
	return actionResult{OK: err == nil}, dockerErr(err)
}

func stopContainer(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	_, err = s.cli.ContainerStop(rc.Ctx, rc.Param("id"), dockerclient.ContainerStopOptions{})
	return actionResult{OK: err == nil}, dockerErr(err)
}

func restartContainer(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	_, err = s.cli.ContainerRestart(rc.Ctx, rc.Param("id"), dockerclient.ContainerRestartOptions{})
	return actionResult{OK: err == nil}, dockerErr(err)
}

func removeContainer(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	_, err = s.cli.ContainerRemove(rc.Ctx, rc.Param("id"), dockerclient.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
	return actionResult{OK: err == nil}, dockerErr(err)
}

func removeImage(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	_, err = s.cli.ImageRemove(rc.Ctx, rc.Param("id"), dockerclient.ImageRemoveOptions{Force: true, PruneChildren: true})
	return actionResult{OK: err == nil}, dockerErr(err)
}

func removeVolume(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	_, err = s.cli.VolumeRemove(rc.Ctx, rc.Param("id"), dockerclient.VolumeRemoveOptions{Force: true})
	return actionResult{OK: err == nil}, dockerErr(err)
}

func removeNetwork(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	_, err = s.cli.NetworkRemove(rc.Ctx, rc.Param("id"), dockerclient.NetworkRemoveOptions{})
	return actionResult{OK: err == nil}, dockerErr(err)
}

func prepareExec(_ *plugin.RequestContext) (any, error) {
	return actionResult{OK: true}, nil
}

func logsStream(rc *plugin.RequestContext, client plugin.ClientStream) error {
	ch, err := rc.Session.OpenChannel(rc.Ctx, plugin.ChannelRequest{Kind: plugin.StreamLogs, Params: streamParams(rc)})
	if err != nil {
		return err
	}
	defer func() { _ = ch.Close() }()
	_, err = io.Copy(client, ch)
	if err == io.EOF {
		return nil
	}
	return err
}

func execStream(rc *plugin.RequestContext, client plugin.ClientStream) error {
	ch, err := rc.Session.OpenChannel(rc.Ctx, plugin.ChannelRequest{Kind: plugin.StreamTerminal, Params: streamParams(rc)})
	if err != nil {
		return err
	}
	defer func() { _ = ch.Close() }()

	errc := make(chan error, 2)
	go func() {
		_, err := io.Copy(client, ch)
		errc <- err
	}()
	go func() {
		errc <- copyTerminalInput(ch, client)
	}()
	select {
	case <-client.Context().Done():
		return nil
	case err := <-errc:
		if err == io.EOF {
			return nil
		}
		return err
	}
}

func watchEvents(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, err := sess(rc)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(rc.Ctx)
	defer cancel()
	result := s.cli.Events(ctx, dockerclient.EventsListOptions{
		Filters: make(dockerclient.Filters).Add("type", string(events.ContainerEventType)),
	})
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
			return dockerErr(err)
		case msg, ok := <-result.Messages:
			if !ok {
				return nil
			}
			ev := resourceEventFromDocker(msg)
			if ev == nil {
				continue
			}
			if err := enc.Encode(ev); err != nil {
				return err
			}
		}
	}
}

type apiRequest struct {
	Method  string      `json:"method" validate:"required"`
	URL     string      `json:"url" validate:"required"`
	Headers []apiHeader `json:"headers"`
	Body    string      `json:"body"`
}

type apiHeader struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type apiResponse struct {
	OK         bool        `json:"ok"`
	Status     int         `json:"status"`
	StatusText string      `json:"statusText"`
	DurationMS float64     `json:"durationMs"`
	Headers    []apiHeader `json:"headers"`
	Body       any         `json:"body"`
}

func executeAPI(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	var in apiRequest
	if err := rc.Bind(&in); err != nil {
		return nil, err
	}
	method := strings.ToUpper(strings.TrimSpace(in.Method))
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return nil, fmt.Errorf("%w: unsupported Docker API method %q", plugin.ErrInvalidInput, in.Method)
	}
	apiPath, err := rawAPIPath(in.URL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(rc.Ctx, method, "http://docker"+apiPath, strings.NewReader(in.Body))
	if err != nil {
		return nil, fmt.Errorf("%w: invalid Docker API request", plugin.ErrInvalidInput)
	}
	for _, h := range in.Headers {
		key := http.CanonicalHeaderKey(strings.TrimSpace(h.Key))
		if key == "" || strings.EqualFold(key, "Host") {
			continue
		}
		req.Header.Set(key, h.Value)
	}
	start := time.Now()
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, dockerErr(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, dockerErr(err)
	}
	out := apiResponse{
		OK:         true,
		Status:     resp.StatusCode,
		StatusText: resp.Status,
		DurationMS: float64(time.Since(start).Microseconds()) / 1000,
		Headers:    headers(resp.Header),
		Body:       decodeBody(body, resp.Header.Get("Content-Type")),
	}
	return out, nil
}

func (s *Session) openLogs(ctx context.Context, params map[string]string) (plugin.Channel, error) {
	id := params["id"]
	if id == "" {
		return nil, fmt.Errorf("%w: container id is required", plugin.ErrInvalidInput)
	}
	inspect, err := s.cli.ContainerInspect(ctx, id, dockerclient.ContainerInspectOptions{})
	if err != nil {
		return nil, dockerErr(err)
	}
	logs, err := s.cli.ContainerLogs(ctx, id, dockerclient.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     boolParam(params, "follow", true),
		Timestamps: boolParam(params, "timestamps", true),
		Since:      params["since"],
		Tail:       stringDefault(params["tail"], "200"),
	})
	if err != nil {
		return nil, dockerErr(err)
	}
	if inspect.Container.Config != nil && inspect.Container.Config.Tty {
		return &logsChannel{Reader: logs, close: logs.Close}, nil
	}
	pr, pw := io.Pipe()
	go func() {
		_, err := stdcopy.StdCopy(pw, pw, logs)
		_ = logs.Close()
		_ = pw.CloseWithError(err)
	}()
	return &logsChannel{Reader: pr, close: pr.Close}, nil
}

func (s *Session) openExec(ctx context.Context, params map[string]string) (plugin.Channel, error) {
	id := params["id"]
	if id == "" {
		return nil, fmt.Errorf("%w: container id is required", plugin.ErrInvalidInput)
	}
	cols := uintParam(params, "cols", 80)
	rows := uintParam(params, "rows", 24)
	cmd := execCommand(params["command"])
	created, err := s.cli.ExecCreate(ctx, id, dockerclient.ExecCreateOptions{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		TTY:          true,
		ConsoleSize:  dockerclient.ConsoleSize{Height: rows, Width: cols},
		Cmd:          cmd,
	})
	if err != nil {
		return nil, dockerErr(err)
	}
	resp, err := s.cli.ExecAttach(ctx, created.ID, dockerclient.ExecAttachOptions{
		TTY:         true,
		ConsoleSize: dockerclient.ConsoleSize{Height: rows, Width: cols},
	})
	if err != nil {
		return nil, dockerErr(err)
	}
	return &execChannel{cli: s.cli, execID: created.ID, resp: resp.HijackedResponse}, nil
}

func containerRows(items []container.Summary) []row {
	rows := make([]row, 0, len(items))
	for _, c := range items {
		name := firstName(c.Names, shortID(c.ID))
		rows = append(rows, row{
			"id":             c.ID,
			"name":           name,
			"image":          c.Image,
			"state":          string(c.State),
			"status":         c.Status,
			"createdAt":      unixTime(c.Created),
			"ports":          ports(c.Ports),
			"compose":        c.Labels["com.docker.compose.project"],
			"composeService": c.Labels["com.docker.compose.service"],
			"ref":            plugin.ResourceRef{Kind: "container", Name: name, UID: c.ID},
		})
	}
	return rows
}

func imageRows(items []image.Summary) []row {
	rows := make([]row, 0, len(items))
	for _, img := range items {
		name := firstString(img.RepoTags, firstString(img.RepoDigests, shortID(img.ID)))
		rows = append(rows, row{
			"id":         img.ID,
			"name":       name,
			"tags":       strings.Join(img.RepoTags, ", "),
			"size":       img.Size,
			"containers": img.Containers,
			"createdAt":  unixTime(img.Created),
			"ref":        plugin.ResourceRef{Kind: "image", Name: name, UID: img.ID},
		})
	}
	return rows
}

func volumeRows(items []volume.Volume) []row {
	rows := make([]row, 0, len(items))
	for _, v := range items {
		size := int64(-1)
		refs := int64(-1)
		if v.UsageData != nil {
			size = v.UsageData.Size
			refs = v.UsageData.RefCount
		}
		rows = append(rows, row{
			"id":         v.Name,
			"name":       v.Name,
			"driver":     v.Driver,
			"scope":      v.Scope,
			"mountpoint": v.Mountpoint,
			"size":       size,
			"refs":       refs,
			"createdAt":  v.CreatedAt,
			"compose":    v.Labels["com.docker.compose.project"],
			"ref":        plugin.ResourceRef{Kind: "volume", Name: v.Name, UID: v.Name},
		})
	}
	return rows
}

func networkRows(items []network.Summary) []row {
	rows := make([]row, 0, len(items))
	for _, n := range items {
		rows = append(rows, row{
			"id":         n.ID,
			"name":       n.Name,
			"driver":     n.Driver,
			"scope":      n.Scope,
			"internal":   n.Internal,
			"attachable": n.Attachable,
			"createdAt":  n.Created.String(),
			"compose":    n.Labels["com.docker.compose.project"],
			"ref":        plugin.ResourceRef{Kind: "network", Name: n.Name, UID: n.ID},
		})
	}
	return rows
}

func composeRows(rc *plugin.RequestContext) ([]row, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.ContainerList(rc.Ctx, dockerclient.ContainerListOptions{All: true})
	if err != nil {
		return nil, dockerErr(err)
	}
	projects := map[string]row{}
	for _, c := range res.Items {
		project := c.Labels["com.docker.compose.project"]
		if project == "" {
			continue
		}
		r, ok := projects[project]
		if !ok {
			r = row{
				"name":       project,
				"workingDir": c.Labels["com.docker.compose.project.working_dir"],
				"config":     c.Labels["com.docker.compose.project.config_files"],
				"containers": 0,
				"running":    0,
				"ref":        plugin.ResourceRef{Kind: "compose", Name: project, UID: project},
			}
			projects[project] = r
		}
		r["containers"] = r["containers"].(int) + 1
		if c.State == "running" {
			r["running"] = r["running"].(int) + 1
		}
	}
	rows := make([]row, 0, len(projects))
	for _, r := range projects {
		rows = append(rows, r)
	}
	return rows, nil
}

func containersForCompose(rc *plugin.RequestContext, project string) ([]row, error) {
	if project == "" {
		return nil, fmt.Errorf("%w: compose project is required", plugin.ErrInvalidInput)
	}
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.ContainerList(rc.Ctx, dockerclient.ContainerListOptions{All: true})
	if err != nil {
		return nil, dockerErr(err)
	}
	filtered := make([]container.Summary, 0, len(res.Items))
	for _, c := range res.Items {
		if c.Labels["com.docker.compose.project"] == project {
			filtered = append(filtered, c)
		}
	}
	return containerRows(filtered), nil
}

func treeFromRows(rc *plugin.RequestContext, kind string, fn func(*plugin.RequestContext) (any, error)) (any, error) {
	result, err := fn(rc)
	if err != nil {
		return nil, err
	}
	page, ok := result.(plugin.Page[row])
	if !ok {
		return nil, fmt.Errorf("%w: unexpected tree data", plugin.ErrUnavailable)
	}
	nodes := make([]plugin.TreeNode, 0, len(page.Items))
	for _, r := range page.Items {
		ref, ok := r["ref"].(plugin.ResourceRef)
		if !ok {
			continue
		}
		nodes = append(nodes, plugin.TreeNode{
			Key:   kind + ":" + ref.UID,
			Label: ref.Name,
			Icon:  iconForKind(kind),
			Ref:   &ref,
			Leaf:  true,
		})
	}
	return plugin.Page[plugin.TreeNode]{Items: nodes, NextCursor: page.NextCursor, Total: page.Total}, nil
}

func pageRows(rc *plugin.RequestContext, rows []row) (plugin.Page[row], error) {
	req, err := rc.Page()
	if err != nil {
		return plugin.Page[row]{}, err
	}
	rows = filterRows(rows, req.Filter["q"])
	sortRows(rows, req.Sort)
	total := len(rows)
	start := 0
	if req.Cursor != "" {
		start, err = strconv.Atoi(req.Cursor)
		if err != nil || start < 0 {
			return plugin.Page[row]{}, fmt.Errorf("%w: cursor must be an offset", plugin.ErrInvalidInput)
		}
	}
	if start > len(rows) {
		start = len(rows)
	}
	end := start + req.Limit
	if end > len(rows) {
		end = len(rows)
	}
	next := ""
	if end < len(rows) {
		next = strconv.Itoa(end)
	}
	return plugin.Page[row]{Items: rows[start:end], NextCursor: next, Total: &total}, nil
}

func filterRows(rows []row, q string) []row {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return rows
	}
	out := rows[:0]
	for _, r := range rows {
		if strings.Contains(strings.ToLower(fmt.Sprint(r)), q) {
			out = append(out, r)
		}
	}
	return out
}

func sortRows(rows []row, sortKeys []plugin.SortKey) {
	if len(sortKeys) == 0 {
		sortKeys = []plugin.SortKey{{Field: "name"}}
	}
	key := sortKeys[0]
	sort.SliceStable(rows, func(i, j int) bool {
		a := fmt.Sprint(rows[i][key.Field])
		b := fmt.Sprint(rows[j][key.Field])
		if key.Desc {
			return a > b
		}
		return a < b
	})
}

func resourceEventFromDocker(msg events.Message) *plugin.ResourceEvent {
	id := msg.Actor.ID
	if id == "" {
		return nil
	}
	name := msg.Actor.Attributes["name"]
	if name == "" {
		name = shortID(id)
	}
	evType := "updated"
	switch msg.Action {
	case "create":
		evType = "added"
	case "destroy", "die":
		evType = "deleted"
	}
	ref := plugin.ResourceRef{Kind: "container", Name: name, UID: id}
	return &plugin.ResourceEvent{
		Type: evType,
		Ref:  ref,
		Resource: row{
			"id":     id,
			"name":   name,
			"state":  string(msg.Action),
			"status": string(msg.Action),
			"ref":    ref,
		},
	}
}

func copyTerminalInput(ch plugin.Channel, client plugin.ClientStream) error {
	buf := make([]byte, 32<<10)
	for {
		n, err := client.Read(buf)
		if n > 0 {
			frame := buf[:n]
			if len(frame) > 1 && frame[0] == 0 {
				_ = handleTerminalControl(ch, frame[1:])
			} else if _, werr := ch.Write(frame); werr != nil {
				return werr
			}
		}
		if err != nil {
			return err
		}
	}
}

type resizer interface {
	Resize(cols, rows int) error
}

func handleTerminalControl(ch plugin.Channel, frame []byte) error {
	var msg struct {
		Type string `json:"type"`
		Cols int    `json:"cols"`
		Rows int    `json:"rows"`
	}
	if err := json.Unmarshal(frame, &msg); err != nil || msg.Type != "resize" {
		return err
	}
	if r, ok := ch.(resizer); ok {
		return r.Resize(msg.Cols, msg.Rows)
	}
	return nil
}

func streamParams(rc *plugin.RequestContext) map[string]string {
	params := map[string]string{"id": rc.Param("id")}
	for _, key := range []string{"cols", "rows", "command", "tail", "since", "follow", "timestamps"} {
		if v := rc.Param(key); v != "" {
			params[key] = v
		} else if v := rc.Query().Get(key); v != "" {
			params[key] = v
		}
	}
	for key, vals := range rc.Query() {
		if strings.HasPrefix(key, "p.") || len(vals) == 0 {
			continue
		}
		if _, ok := params[key]; !ok {
			params[key] = vals[0]
		}
	}
	return params
}

func createContainerSchema() *plugin.Schema {
	onFailure := plugin.Condition{AllOf: []plugin.Rule{{Field: "restart", Op: plugin.OpEq, Value: string(container.RestartPolicyOnFailure)}}}
	return &plugin.Schema{Groups: []plugin.Group{
		{Name: "Container", Fields: []plugin.Field{
			{Key: "name", Label: "Name", Type: plugin.FieldText, Placeholder: "web", Validators: []plugin.Validator{{Type: plugin.ValidatorRegex, Value: `^[A-Za-z0-9][A-Za-z0-9_.-]*$`, Message: "Use letters, numbers, dots, underscores, or dashes."}}},
			{Key: "image", Label: "Image", Type: plugin.FieldText, Required: true, Placeholder: "nginx:latest"},
			{Key: "pull", Label: "Pull image first", Type: plugin.FieldToggle, Default: true},
			{Key: "start", Label: "Start after create", Type: plugin.FieldToggle, Default: true},
		}},
		{Name: "Runtime", Fields: []plugin.Field{
			{Key: "command", Label: "Command", Type: plugin.FieldText, Placeholder: "sleep 3600"},
			{Key: "entrypoint", Label: "Entrypoint", Type: plugin.FieldText, Placeholder: "/bin/sh"},
			{Key: "user", Label: "User", Type: plugin.FieldText, Placeholder: "1000:1000"},
			{Key: "working_dir", Label: "Working directory", Type: plugin.FieldText, Placeholder: "/app"},
			{Key: "env", Label: "Environment", Type: plugin.FieldTextarea, Placeholder: "APP_ENV=prod\nLOG_LEVEL=info"},
			{Key: "tty", Label: "TTY", Type: plugin.FieldToggle},
			{Key: "open_stdin", Label: "Open stdin", Type: plugin.FieldToggle},
		}},
		{Name: "Host", Fields: []plugin.Field{
			{Key: "ports", Label: "Published ports", Type: plugin.FieldTextarea, Placeholder: "8080:80/tcp\n127.0.0.1:5432:5432/tcp"},
			{Key: "binds", Label: "Bind mounts", Type: plugin.FieldTextarea, Placeholder: "/srv/app:/app:ro\n/var/log/app:/logs"},
			{Key: "network", Label: "Network", Type: plugin.FieldText, Placeholder: "bridge"},
			{Key: "restart", Label: "Restart policy", Type: plugin.FieldSelect, Default: string(container.RestartPolicyDisabled), Options: []plugin.Option{
				{Label: "No", Value: string(container.RestartPolicyDisabled)},
				{Label: "Always", Value: string(container.RestartPolicyAlways)},
				{Label: "Unless stopped", Value: string(container.RestartPolicyUnlessStopped)},
				{Label: "On failure", Value: string(container.RestartPolicyOnFailure)},
			}},
			{Key: "restart_retries", Label: "Restart retries", Type: plugin.FieldNumber, VisibleWhen: &onFailure, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 0}}},
		}},
	}}
}

func logsSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Logs", Fields: []plugin.Field{
		{Key: "tail", Label: "Tail", Type: plugin.FieldNumber},
		{Key: "since", Label: "Since", Type: plugin.FieldText},
		{Key: "follow", Label: "Follow", Type: plugin.FieldToggle},
		{Key: "timestamps", Label: "Timestamps", Type: plugin.FieldToggle},
	}}}}
}

func execSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Exec", Fields: []plugin.Field{
		{Key: "cols", Label: "Columns", Type: plugin.FieldNumber},
		{Key: "rows", Label: "Rows", Type: plugin.FieldNumber},
		{Key: "command", Label: "Command", Type: plugin.FieldText},
	}}}}
}

func apiSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Request", Fields: []plugin.Field{
		{Key: "method", Label: "Method", Type: plugin.FieldSelect, Required: true, Options: []plugin.Option{
			{Label: "GET", Value: "GET"},
			{Label: "POST", Value: "POST"},
			{Label: "PUT", Value: "PUT"},
			{Label: "PATCH", Value: "PATCH"},
			{Label: "DELETE", Value: "DELETE"},
		}},
		{Key: "url", Label: "Path", Type: plugin.FieldText, Required: true},
		{Key: "headers", Label: "Headers", Type: plugin.FieldJSON},
		{Key: "body", Label: "Body", Type: plugin.FieldTextarea},
	}}}}
}

func dockerErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case cerrdefs.IsNotFound(err):
		return fmt.Errorf("%w: %v", plugin.ErrNotFound, err)
	case cerrdefs.IsInvalidArgument(err):
		return fmt.Errorf("%w: %v", plugin.ErrInvalidInput, err)
	case cerrdefs.IsConflict(err):
		return fmt.Errorf("%w: %v", plugin.ErrConflict, err)
	case cerrdefs.IsUnavailable(err):
		return fmt.Errorf("%w: %v", plugin.ErrUnavailable, err)
	default:
		return err
	}
}

func rawOrValue(raw json.RawMessage, value any) (any, error) {
	if len(raw) == 0 {
		return value, nil
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return value, nil
	}
	return out, nil
}

func headers(h http.Header) []apiHeader {
	out := make([]apiHeader, 0, len(h))
	for key, values := range h {
		out = append(out, apiHeader{Key: key, Value: strings.Join(values, ", ")})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func decodeBody(b []byte, contentType string) any {
	if len(bytes.TrimSpace(b)) == 0 {
		return ""
	}
	if strings.Contains(strings.ToLower(contentType), "json") {
		var out any
		if json.Unmarshal(b, &out) == nil {
			return out
		}
	}
	var out any
	if json.Unmarshal(b, &out) == nil {
		return out
	}
	return string(b)
}

func pickStruct(value any, keys ...string) row {
	var m map[string]any
	b, err := json.Marshal(value)
	if err != nil || json.Unmarshal(b, &m) != nil {
		return row{}
	}
	out := row{}
	for _, key := range keys {
		if v, ok := m[key]; ok {
			out[key] = v
		}
	}
	return out
}

func mapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func iconForKind(kind string) plugin.Icon {
	switch kind {
	case "container":
		return plugin.Icon{Type: plugin.IconLucide, Value: "box"}
	case "image":
		return plugin.Icon{Type: plugin.IconLucide, Value: "layers"}
	case "volume":
		return plugin.Icon{Type: plugin.IconLucide, Value: "database"}
	case "network":
		return plugin.Icon{Type: plugin.IconLucide, Value: "globe"}
	default:
		return plugin.Icon{Type: plugin.IconLucide, Value: "box"}
	}
}

func firstName(names []string, fallback string) string {
	if len(names) == 0 {
		return fallback
	}
	return strings.TrimPrefix(names[0], "/")
}

func firstString(values []string, fallback string) string {
	if len(values) == 0 || values[0] == "" || values[0] == "<none>:<none>" {
		return fallback
	}
	return values[0]
}

func boolDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func shellFields(field string, value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parts, err := shellquote.Split(value)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid %s: %v", plugin.ErrInvalidInput, field, err)
	}
	return parts, nil
}

func nonEmptyLines(value string) []string {
	lines := strings.Split(value, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

func parseEnvLines(value string) ([]string, error) {
	lines := nonEmptyLines(value)
	for _, line := range lines {
		key, _, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("%w: environment entries must be KEY=value", plugin.ErrInvalidInput)
		}
	}
	return lines, nil
}

func parseRestartPolicy(value string, retries int) (container.RestartPolicy, error) {
	switch container.RestartPolicyMode(strings.TrimSpace(value)) {
	case "", container.RestartPolicyDisabled:
		return container.RestartPolicy{Name: container.RestartPolicyDisabled}, nil
	case container.RestartPolicyAlways:
		return container.RestartPolicy{Name: container.RestartPolicyAlways}, nil
	case container.RestartPolicyUnlessStopped:
		return container.RestartPolicy{Name: container.RestartPolicyUnlessStopped}, nil
	case container.RestartPolicyOnFailure:
		if retries < 0 {
			return container.RestartPolicy{}, fmt.Errorf("%w: restart retries cannot be negative", plugin.ErrInvalidInput)
		}
		return container.RestartPolicy{Name: container.RestartPolicyOnFailure, MaximumRetryCount: retries}, nil
	default:
		return container.RestartPolicy{}, fmt.Errorf("%w: unsupported restart policy %q", plugin.ErrInvalidInput, value)
	}
}

func parsePublishedPorts(value string) (network.PortSet, network.PortMap, error) {
	lines := nonEmptyLines(value)
	if len(lines) == 0 {
		return nil, nil, nil
	}
	exposed := network.PortSet{}
	bindings := network.PortMap{}
	for _, line := range lines {
		hostIP, hostPort, containerPort, err := splitPublishedPort(line)
		if err != nil {
			return nil, nil, err
		}
		port, err := network.ParsePort(containerPort)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: invalid container port %q", plugin.ErrInvalidInput, containerPort)
		}
		binding := network.PortBinding{HostPort: hostPort}
		if hostIP != "" {
			ip, err := netip.ParseAddr(hostIP)
			if err != nil {
				return nil, nil, fmt.Errorf("%w: invalid host IP %q", plugin.ErrInvalidInput, hostIP)
			}
			binding.HostIP = ip
		}
		exposed[port] = struct{}{}
		bindings[port] = append(bindings[port], binding)
	}
	return exposed, bindings, nil
}

func splitPublishedPort(value string) (hostIP string, hostPort string, containerPort string, err error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", "", fmt.Errorf("%w: port entry is empty", plugin.ErrInvalidInput)
	}
	if strings.HasPrefix(value, "[") {
		end := strings.Index(value, "]")
		if end < 0 || len(value) <= end+2 || value[end+1] != ':' {
			return "", "", "", fmt.Errorf("%w: invalid published port %q", plugin.ErrInvalidInput, value)
		}
		hostIP = value[1:end]
		rest := strings.Split(value[end+2:], ":")
		if len(rest) != 2 {
			return "", "", "", fmt.Errorf("%w: invalid published port %q", plugin.ErrInvalidInput, value)
		}
		return hostIP, rest[0], rest[1], nil
	}
	parts := strings.Split(value, ":")
	switch len(parts) {
	case 1:
		return "", "", parts[0], nil
	case 2:
		return "", parts[0], parts[1], nil
	case 3:
		return parts[0], parts[1], parts[2], nil
	default:
		return "", "", "", fmt.Errorf("%w: invalid published port %q", plugin.ErrInvalidInput, value)
	}
}

func shortID(id string) string {
	id = strings.TrimPrefix(id, "sha256:")
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func unixTime(sec int64) string {
	if sec <= 0 {
		return ""
	}
	return time.Unix(sec, 0).UTC().Format(time.RFC3339)
}

func ports(ports []container.PortSummary) string {
	out := make([]string, 0, len(ports))
	for _, p := range ports {
		target := strconv.Itoa(int(p.PrivatePort)) + "/" + p.Type
		if p.PublicPort > 0 {
			target = fmt.Sprintf("%s:%d->%s", p.IP, p.PublicPort, target)
		}
		out = append(out, target)
	}
	return strings.Join(out, ", ")
}

func stringDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func boolParam(params map[string]string, key string, fallback bool) bool {
	raw := params[key]
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return v
}

func uintParam(params map[string]string, key string, fallback uint) uint {
	raw := params[key]
	if raw == "" {
		return fallback
	}
	n, err := strconv.ParseUint(raw, 10, 32)
	if err != nil || n == 0 {
		return fallback
	}
	return uint(n)
}

func execCommand(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{"/bin/sh"}
	}
	return []string{"/bin/sh", "-lc", raw}
}

func ptr[T any](v T) *T {
	return &v
}
