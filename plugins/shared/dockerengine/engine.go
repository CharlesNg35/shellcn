package dockerengine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

	"github.com/charlesng35/shellcn/internal/plugin"
)

// Row is one generic record returned by the shared handlers. It is an alias so
// plugin packages can build rows as plain maps and still feed PageRows/TreeFromRows.
type Row = map[string]any

// ComposeProjectLabel groups containers/volumes into a Compose project.
const ComposeProjectLabel = "com.docker.compose.project"

type ActionResult struct {
	OK bool `json:"ok"`
}

type CreateContainerResult struct {
	OK       bool     `json:"ok"`
	ID       string   `json:"id"`
	Name     string   `json:"name,omitempty"`
	Started  bool     `json:"started"`
	Warnings []string `json:"warnings,omitempty"`
}

type CreateContainerRequest struct {
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

func sess(rc *plugin.RequestContext) (*Session, error) {
	return Unwrap(rc.Session)
}

func ListContainers(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.ContainerList(rc.Ctx, dockerclient.ContainerListOptions{All: true})
	if err != nil {
		return nil, DockerErr(err)
	}
	return PageRows(rc, ContainerRows(res.Items))
}

func ListImages(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.ImageList(rc.Ctx, dockerclient.ImageListOptions{All: true})
	if err != nil {
		return nil, DockerErr(err)
	}
	return PageRows(rc, ImageRows(res.Items))
}

func ListVolumes(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.VolumeList(rc.Ctx, dockerclient.VolumeListOptions{})
	if err != nil {
		return nil, DockerErr(err)
	}
	return PageRows(rc, VolumeRows(res.Items))
}

func ListNetworks(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.NetworkList(rc.Ctx, dockerclient.NetworkListOptions{})
	if err != nil {
		return nil, DockerErr(err)
	}
	return PageRows(rc, NetworkRows(res.Items))
}

func ListCompose(rc *plugin.RequestContext) (any, error) {
	rows, err := composeRows(rc)
	if err != nil {
		return nil, err
	}
	return PageRows(rc, rows)
}

func TreeContainers(rc *plugin.RequestContext) (any, error) {
	return TreeFromRows(rc, "container", ListContainers)
}

func TreeImages(rc *plugin.RequestContext) (any, error) { return TreeFromRows(rc, "image", ListImages) }

func TreeVolumes(rc *plugin.RequestContext) (any, error) {
	return TreeFromRows(rc, "volume", ListVolumes)
}

func TreeNetworks(rc *plugin.RequestContext) (any, error) {
	return TreeFromRows(rc, "network", ListNetworks)
}

func TreeCompose(rc *plugin.RequestContext) (any, error) {
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
			Data:  r,
		})
	}
	return plugin.Page[plugin.TreeNode]{Items: nodes, Total: ptr(len(nodes))}, nil
}

func ContainerOverview(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.ContainerInspect(rc.Ctx, rc.Param("id"), dockerclient.ContainerInspectOptions{})
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
		out["composeProject"] = res.Container.Config.Labels[ComposeProjectLabel]
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

func ImageOverview(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.ImageInspect(rc.Ctx, rc.Param("id"))
	if err != nil {
		return nil, DockerErr(err)
	}
	return pickStruct(res, "Id", "RepoTags", "RepoDigests", "Created", "Size", "VirtualSize", "Architecture", "Os", "DockerVersion", "Author", "Comment"), nil
}

func VolumeOverview(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.VolumeInspect(rc.Ctx, rc.Param("id"), dockerclient.VolumeInspectOptions{})
	if err != nil {
		return nil, DockerErr(err)
	}
	out := Row{
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

func NetworkOverview(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.NetworkInspect(rc.Ctx, rc.Param("id"), dockerclient.NetworkInspectOptions{})
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

func ComposeOverview(rc *plugin.RequestContext) (any, error) {
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

func ComposeContainers(rc *plugin.RequestContext) (any, error) {
	rows, err := containersForCompose(rc, rc.Param("project"))
	if err != nil {
		return nil, err
	}
	return PageRows(rc, rows)
}

func ComposeServices(rc *plugin.RequestContext) (any, error) {
	containers, err := containersForCompose(rc, rc.Param("project"))
	if err != nil {
		return nil, err
	}
	services := map[string]Row{}
	for _, c := range containers {
		name, _ := c["composeService"].(string)
		if name == "" {
			name = "(default)"
		}
		r, ok := services[name]
		if !ok {
			r = Row{"name": name, "image": c["image"], "containers": 0, "running": 0, "ports": c["ports"]}
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
	rows := make([]Row, 0, len(services))
	for _, r := range services {
		rows = append(rows, r)
	}
	return PageRows(rc, rows)
}

func InspectContainer(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.ContainerInspect(rc.Ctx, rc.Param("id"), dockerclient.ContainerInspectOptions{})
	if err != nil {
		return nil, DockerErr(err)
	}
	return rawOrValue(res.Raw, res.Container)
}

func InspectImage(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.ImageInspect(rc.Ctx, rc.Param("id"))
	if err != nil {
		return nil, DockerErr(err)
	}
	return res, nil
}

func InspectVolume(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.VolumeInspect(rc.Ctx, rc.Param("id"), dockerclient.VolumeInspectOptions{})
	if err != nil {
		return nil, DockerErr(err)
	}
	return rawOrValue(res.Raw, res.Volume)
}

func InspectNetwork(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.NetworkInspect(rc.Ctx, rc.Param("id"), dockerclient.NetworkInspectOptions{})
	if err != nil {
		return nil, DockerErr(err)
	}
	return rawOrValue(res.Raw, res.Network)
}

func ContainerEnv(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.ContainerInspect(rc.Ctx, rc.Param("id"), dockerclient.ContainerInspectOptions{})
	if err != nil {
		return nil, DockerErr(err)
	}
	var rows []Row
	if res.Container.Config != nil {
		for _, env := range res.Container.Config.Env {
			key, value, _ := strings.Cut(env, "=")
			rows = append(rows, Row{
				"key":   key,
				"value": value,
				"ref":   plugin.ResourceRef{Kind: "env", Name: key, UID: key},
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool { return fmt.Sprint(rows[i]["key"]) < fmt.Sprint(rows[j]["key"]) })
	return PageRows(rc, rows)
}

func CreateContainer(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	var in CreateContainerRequest
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
		return nil, DockerErr(err)
	}
	started := false
	if boolDefault(in.Start, true) {
		if _, err := s.cli.ContainerStart(rc.Ctx, created.ID, dockerclient.ContainerStartOptions{}); err != nil {
			return nil, DockerErr(err)
		}
		started = true
	}
	return CreateContainerResult{
		OK:       true,
		ID:       shortID(created.ID),
		Name:     strings.TrimSpace(in.Name),
		Started:  started,
		Warnings: created.Warnings,
	}, nil
}

func StartContainer(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	_, err = s.cli.ContainerStart(rc.Ctx, rc.Param("id"), dockerclient.ContainerStartOptions{})
	return ActionResult{OK: err == nil}, DockerErr(err)
}

func StopContainer(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	_, err = s.cli.ContainerStop(rc.Ctx, rc.Param("id"), dockerclient.ContainerStopOptions{})
	return ActionResult{OK: err == nil}, DockerErr(err)
}

func RestartContainer(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	_, err = s.cli.ContainerRestart(rc.Ctx, rc.Param("id"), dockerclient.ContainerRestartOptions{})
	return ActionResult{OK: err == nil}, DockerErr(err)
}

func RemoveContainer(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	_, err = s.cli.ContainerRemove(rc.Ctx, rc.Param("id"), dockerclient.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
	return ActionResult{OK: err == nil}, DockerErr(err)
}

func RemoveImage(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	_, err = s.cli.ImageRemove(rc.Ctx, rc.Param("id"), dockerclient.ImageRemoveOptions{Force: true, PruneChildren: true})
	return ActionResult{OK: err == nil}, DockerErr(err)
}

func RemoveVolume(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	_, err = s.cli.VolumeRemove(rc.Ctx, rc.Param("id"), dockerclient.VolumeRemoveOptions{Force: true})
	return ActionResult{OK: err == nil}, DockerErr(err)
}

func RemoveNetwork(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	_, err = s.cli.NetworkRemove(rc.Ctx, rc.Param("id"), dockerclient.NetworkRemoveOptions{})
	return ActionResult{OK: err == nil}, DockerErr(err)
}

// ImagePullSchema returns the input form for the shared image pull handler.
func ImagePullSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Image", Fields: []plugin.Field{
		{Key: "image", Label: "Image", Type: plugin.FieldText, Required: true, Placeholder: "nginx:latest", Help: "Image reference (repository:tag)."},
	}}}}
}

func VolumeCreateSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Volume", Fields: []plugin.Field{
		{Key: "name", Label: "Name", Type: plugin.FieldText, Required: true},
		{Key: "driver", Label: "Driver", Type: plugin.FieldText, Default: "local", Placeholder: "local", Help: "Volume driver. Use local unless a custom volume plugin is installed."},
	}}}}
}

func NetworkCreateSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Network", Fields: []plugin.Field{
		{Key: "name", Label: "Name", Type: plugin.FieldText, Required: true},
		{Key: "driver", Label: "Driver", Type: plugin.FieldText, Default: "bridge", Placeholder: "bridge", Help: "Network driver, e.g. bridge, overlay, macvlan, ipvlan, host, none."},
	}}}}
}

// PullImage pulls an image by reference, waiting for the pull to finish.
func PullImage(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	var req struct {
		Image string `json:"image" validate:"required"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	ref := strings.TrimSpace(req.Image)
	if ref == "" {
		return nil, fmt.Errorf("%w: image reference is required", plugin.ErrInvalidInput)
	}
	pull, err := s.cli.ImagePull(rc.Ctx, ref, dockerclient.ImagePullOptions{})
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
	return ActionResult{OK: true}, nil
}

// CreateVolume creates a named volume.
func CreateVolume(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	var req struct {
		Name   string `json:"name" validate:"required"`
		Driver string `json:"driver"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("%w: volume name is required", plugin.ErrInvalidInput)
	}
	if _, err := s.cli.VolumeCreate(rc.Ctx, dockerclient.VolumeCreateOptions{Name: strings.TrimSpace(req.Name), Driver: strings.TrimSpace(req.Driver)}); err != nil {
		return nil, DockerErr(err)
	}
	return ActionResult{OK: true}, nil
}

// CreateNetwork creates a network.
func CreateNetwork(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	var req struct {
		Name   string `json:"name" validate:"required"`
		Driver string `json:"driver"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("%w: network name is required", plugin.ErrInvalidInput)
	}
	if _, err := s.cli.NetworkCreate(rc.Ctx, name, dockerclient.NetworkCreateOptions{Driver: strings.TrimSpace(req.Driver)}); err != nil {
		return nil, DockerErr(err)
	}
	return ActionResult{OK: true}, nil
}

// PruneResult summarises a prune sweep for the generic action toast.
type PruneResult struct {
	OK             bool   `json:"ok"`
	Deleted        int    `json:"deleted"`
	SpaceReclaimed uint64 `json:"spaceReclaimed"`
}

// PruneContainers removes all stopped containers.
func PruneContainers(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.ContainerPrune(rc.Ctx, dockerclient.ContainerPruneOptions{})
	if err != nil {
		return nil, DockerErr(err)
	}
	return PruneResult{OK: true, Deleted: len(res.Report.ContainersDeleted), SpaceReclaimed: res.Report.SpaceReclaimed}, nil
}

// PruneImages removes dangling images.
func PruneImages(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.ImagePrune(rc.Ctx, dockerclient.ImagePruneOptions{})
	if err != nil {
		return nil, DockerErr(err)
	}
	return PruneResult{OK: true, Deleted: len(res.Report.ImagesDeleted), SpaceReclaimed: res.Report.SpaceReclaimed}, nil
}

// PruneVolumes removes unused volumes.
func PruneVolumes(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.VolumePrune(rc.Ctx, dockerclient.VolumePruneOptions{})
	if err != nil {
		return nil, DockerErr(err)
	}
	return PruneResult{OK: true, Deleted: len(res.Report.VolumesDeleted), SpaceReclaimed: res.Report.SpaceReclaimed}, nil
}

// PruneNetworks removes unused networks.
func PruneNetworks(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.NetworkPrune(rc.Ctx, dockerclient.NetworkPruneOptions{})
	if err != nil {
		return nil, DockerErr(err)
	}
	return PruneResult{OK: true, Deleted: len(res.Report.NetworksDeleted)}, nil
}

func LogsStream(rc *plugin.RequestContext, client plugin.ClientStream) error {
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

func ExecStream(rc *plugin.RequestContext, client plugin.ClientStream) error {
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

func WatchEvents(rc *plugin.RequestContext, client plugin.ClientStream) error {
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
			return DockerErr(err)
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

func (s *Session) openLogs(ctx context.Context, params map[string]string) (plugin.Channel, error) {
	id := params["id"]
	if id == "" {
		return nil, fmt.Errorf("%w: container id is required", plugin.ErrInvalidInput)
	}
	inspect, err := s.cli.ContainerInspect(ctx, id, dockerclient.ContainerInspectOptions{})
	if err != nil {
		return nil, DockerErr(err)
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
		return nil, DockerErr(err)
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
		return nil, DockerErr(err)
	}
	resp, err := s.cli.ExecAttach(ctx, created.ID, dockerclient.ExecAttachOptions{
		TTY:         true,
		ConsoleSize: dockerclient.ConsoleSize{Height: rows, Width: cols},
	})
	if err != nil {
		return nil, DockerErr(err)
	}
	return &execChannel{cli: s.cli, execID: created.ID, resp: resp.HijackedResponse}, nil
}

// WhenState gates an action on the row's "state" field.
func WhenState(states ...string) *plugin.Condition {
	return &plugin.Condition{AllOf: []plugin.Rule{{Field: "state", Op: plugin.OpIn, Value: states}}}
}

// StateSeverities colors a container "state" badge by value.
func StateSeverities() map[string]plugin.Severity {
	return map[string]plugin.Severity{
		"running": plugin.SeveritySuccess,
		"paused":  plugin.SeverityWarn, "restarting": plugin.SeverityWarn, "removing": plugin.SeverityWarn,
		"created": plugin.SeverityInfo, "configured": plugin.SeverityInfo,
		"exited": plugin.SeveritySecondary, "stopped": plugin.SeveritySecondary,
		"dead": plugin.SeverityDanger,
	}
}

func ContainerRows(items []container.Summary) []Row {
	rows := make([]Row, 0, len(items))
	for _, c := range items {
		name := firstName(c.Names, shortID(c.ID))
		rows = append(rows, Row{
			"id":             c.ID,
			"name":           name,
			"image":          c.Image,
			"state":          string(c.State),
			"status":         c.Status,
			"createdAt":      unixTime(c.Created),
			"ports":          ports(c.Ports),
			"compose":        c.Labels[ComposeProjectLabel],
			"composeService": c.Labels["com.docker.compose.service"],
			"ref":            plugin.ResourceRef{Kind: "container", Name: name, UID: c.ID},
		})
	}
	return rows
}

func ImageRows(items []image.Summary) []Row {
	rows := make([]Row, 0, len(items))
	for _, img := range items {
		name := firstString(img.RepoTags, firstString(img.RepoDigests, shortID(img.ID)))
		rows = append(rows, Row{
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

func VolumeRows(items []volume.Volume) []Row {
	rows := make([]Row, 0, len(items))
	for _, v := range items {
		size := int64(-1)
		refs := int64(-1)
		if v.UsageData != nil {
			size = v.UsageData.Size
			refs = v.UsageData.RefCount
		}
		rows = append(rows, Row{
			"id":         v.Name,
			"name":       v.Name,
			"driver":     v.Driver,
			"scope":      v.Scope,
			"mountpoint": v.Mountpoint,
			"size":       size,
			"refs":       refs,
			"createdAt":  v.CreatedAt,
			"compose":    v.Labels[ComposeProjectLabel],
			"ref":        plugin.ResourceRef{Kind: "volume", Name: v.Name, UID: v.Name},
		})
	}
	return rows
}

func NetworkRows(items []network.Summary) []Row {
	rows := make([]Row, 0, len(items))
	for _, n := range items {
		rows = append(rows, Row{
			"id":         n.ID,
			"name":       n.Name,
			"driver":     n.Driver,
			"scope":      n.Scope,
			"internal":   n.Internal,
			"attachable": n.Attachable,
			"createdAt":  n.Created.String(),
			"compose":    n.Labels[ComposeProjectLabel],
			"ref":        plugin.ResourceRef{Kind: "network", Name: n.Name, UID: n.ID},
		})
	}
	return rows
}

func composeRows(rc *plugin.RequestContext) ([]Row, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.ContainerList(rc.Ctx, dockerclient.ContainerListOptions{All: true})
	if err != nil {
		return nil, DockerErr(err)
	}
	projects := map[string]Row{}
	for _, c := range res.Items {
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
				"ref":        plugin.ResourceRef{Kind: "compose", Name: project, UID: project},
			}
			projects[project] = r
		}
		r["containers"] = r["containers"].(int) + 1
		if c.State == "running" {
			r["running"] = r["running"].(int) + 1
		}
	}
	rows := make([]Row, 0, len(projects))
	for _, r := range projects {
		rows = append(rows, r)
	}
	return rows, nil
}

func containersForCompose(rc *plugin.RequestContext, project string) ([]Row, error) {
	if project == "" {
		return nil, fmt.Errorf("%w: compose project is required", plugin.ErrInvalidInput)
	}
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.ContainerList(rc.Ctx, dockerclient.ContainerListOptions{All: true})
	if err != nil {
		return nil, DockerErr(err)
	}
	filtered := make([]container.Summary, 0, len(res.Items))
	for _, c := range res.Items {
		if c.Labels[ComposeProjectLabel] == project {
			filtered = append(filtered, c)
		}
	}
	return ContainerRows(filtered), nil
}

// TreeFromRows turns a list handler's Page[Row] into sidebar tree nodes, using
// each row's "ref" to build the node. fn is typically one of the List* handlers.
func TreeFromRows(rc *plugin.RequestContext, kind string, fn func(*plugin.RequestContext) (any, error)) (any, error) {
	result, err := fn(rc)
	if err != nil {
		return nil, err
	}
	page, ok := result.(plugin.Page[Row])
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
			Data:  r,
		})
	}
	return plugin.Page[plugin.TreeNode]{Items: nodes, NextCursor: page.NextCursor, Total: page.Total}, nil
}

// PageRows applies the request's filter, sort, and cursor pagination to rows.
func PageRows(rc *plugin.RequestContext, rows []Row) (plugin.Page[Row], error) {
	req, err := rc.Page()
	if err != nil {
		return plugin.Page[Row]{}, err
	}
	rows = filterRows(rows, req.Search())
	sortRows(rows, req.Sort)
	total := len(rows)
	start := 0
	if req.Cursor != "" {
		start, err = strconv.Atoi(req.Cursor)
		if err != nil || start < 0 {
			return plugin.Page[Row]{}, fmt.Errorf("%w: cursor must be an offset", plugin.ErrInvalidInput)
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
	return plugin.Page[Row]{Items: rows[start:end], NextCursor: next, Total: &total}, nil
}

func filterRows(rows []Row, q string) []Row {
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

func sortRows(rows []Row, sortKeys []plugin.SortKey) {
	if len(sortKeys) == 0 {
		sortKeys = []plugin.SortKey{{Field: "name"}}
	}
	key := sortKeys[0]
	sort.SliceStable(rows, func(i, j int) bool {
		cmp := compareCells(rows[i][key.Field], rows[j][key.Field])
		if key.Desc {
			return cmp > 0
		}
		return cmp < 0
	})
}

// compareCells orders two cell values numerically when both are numbers,
// otherwise by case-insensitive string. Numeric columns (sizes, counts) would
// sort lexicographically otherwise (e.g. "1000" before "9").
func compareCells(a, b any) int {
	if an, ok := numericCell(a); ok {
		if bn, ok := numericCell(b); ok {
			switch {
			case an < bn:
				return -1
			case an > bn:
				return 1
			default:
				return 0
			}
		}
	}
	as, bs := strings.ToLower(fmt.Sprint(a)), strings.ToLower(fmt.Sprint(b))
	switch {
	case as < bs:
		return -1
	case as > bs:
		return 1
	default:
		return 0
	}
}

func numericCell(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

func resourceEventFromDocker(msg events.Message) *plugin.ResourceEvent {
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
	ref := plugin.ResourceRef{Kind: "container", Name: name, UID: id}
	resource := Row{
		"id":     id,
		"name":   name,
		"status": string(msg.Action),
		"ref":    ref,
	}
	// Patch state only for transitions with a known state, so a merge never
	// clobbers the live state with an empty or non-state action label.
	if state != "" {
		resource["state"] = state
	}
	return &plugin.ResourceEvent{Type: evType, Ref: ref, Resource: resource}
}

// containerEventKind maps a Docker lifecycle event to a list patch. Only destroy
// removes a row; a container that merely exits (die/stop/kill) stays listed as
// "exited". Non-lifecycle noise (exec_*, attach, top, resize, …) is dropped.
func containerEventKind(action events.Action) (evType, state string, ok bool) {
	switch action {
	case events.ActionCreate:
		return "added", "created", true
	case events.ActionDestroy:
		return "deleted", "", true
	case events.ActionStart, events.ActionUnPause, events.ActionRestart:
		return "updated", "running", true
	case events.ActionDie, events.ActionStop, events.ActionKill, events.ActionOOM:
		return "updated", "exited", true
	case events.ActionPause:
		return "updated", "paused", true
	case events.ActionRename, events.ActionUpdate:
		return "updated", "", true
	default:
		return "", "", false
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

// CreateContainerSchema is the manifest input schema for the create form.
func CreateContainerSchema() *plugin.Schema {
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

// LogsSchema is the manifest input schema for the log stream controls.
func LogsSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Logs", Fields: []plugin.Field{
		{Key: "tail", Label: "Tail", Type: plugin.FieldNumber},
		{Key: "since", Label: "Since", Type: plugin.FieldText},
		{Key: "follow", Label: "Follow", Type: plugin.FieldToggle},
		{Key: "timestamps", Label: "Timestamps", Type: plugin.FieldToggle},
	}}}}
}

// ExecSchema is the manifest input schema for the exec terminal controls.
func ExecSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Exec", Fields: []plugin.Field{
		{Key: "cols", Label: "Columns", Type: plugin.FieldNumber},
		{Key: "rows", Label: "Rows", Type: plugin.FieldNumber},
		{Key: "command", Label: "Command", Type: plugin.FieldText},
	}}}}
}

// DockerErr maps a moby client error to a platform sentinel so the response
// layer can pick the right status code.
func DockerErr(err error) error {
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

// RawOrValue returns the daemon's raw inspect JSON when present, falling back to
// the typed value. Plugin packages use it for their own inspect routes.
func RawOrValue(raw json.RawMessage, value any) (any, error) {
	return rawOrValue(raw, value)
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

func pickStruct(value any, keys ...string) Row {
	var m map[string]any
	b, err := json.Marshal(value)
	if err != nil || json.Unmarshal(b, &m) != nil {
		return Row{}
	}
	out := Row{}
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

// ShortID trims a sha256 prefix and truncates to 12 chars for display.
func ShortID(id string) string { return shortID(id) }

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
