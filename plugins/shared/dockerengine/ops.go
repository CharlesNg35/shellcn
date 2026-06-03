package dockerengine

import (
	"archive/tar"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/registry"
	dockerclient "github.com/moby/moby/client"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

// killSignals is the allow-list of signals the kill action accepts. Constraining
// it keeps an arbitrary value from reaching the daemon and gives the form a
// closed vocabulary.
var killSignals = []string{
	"SIGKILL", "SIGTERM", "SIGINT", "SIGHUP", "SIGQUIT", "SIGUSR1", "SIGUSR2", "SIGSTOP", "SIGCONT",
}

func PauseContainer(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	_, err = s.cli.ContainerPause(rc.Ctx, rc.Param("id"), dockerclient.ContainerPauseOptions{})
	return ActionResult{OK: err == nil}, DockerErr(err)
}

func UnpauseContainer(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	_, err = s.cli.ContainerUnpause(rc.Ctx, rc.Param("id"), dockerclient.ContainerUnpauseOptions{})
	return ActionResult{OK: err == nil}, DockerErr(err)
}

// KillContainer sends a signal to a container, defaulting to SIGKILL.
func KillContainer(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	var req struct {
		Signal string `json:"signal"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	signal, err := normalizeKillSignal(req.Signal)
	if err != nil {
		return nil, err
	}
	_, err = s.cli.ContainerKill(rc.Ctx, rc.Param("id"), dockerclient.ContainerKillOptions{Signal: signal})
	return ActionResult{OK: err == nil}, DockerErr(err)
}

func RenameContainer(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	var req struct {
		Name string `json:"name" validate:"required"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if err := validateResourceName(name); err != nil {
		return nil, err
	}
	_, err = s.cli.ContainerRename(rc.Ctx, rc.Param("id"), dockerclient.ContainerRenameOptions{NewName: name})
	return ActionResult{OK: err == nil}, DockerErr(err)
}

func TagImage(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	var req struct {
		Target string `json:"target" validate:"required"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	target := strings.TrimSpace(req.Target)
	if target == "" {
		return nil, fmt.Errorf("%w: target reference is required", plugin.ErrInvalidInput)
	}
	source := strings.TrimSpace(rc.Param("id"))
	if source == "" {
		return nil, fmt.Errorf("%w: source image is required", plugin.ErrInvalidInput)
	}
	_, err = s.cli.ImageTag(rc.Ctx, dockerclient.ImageTagOptions{Source: source, Target: target})
	return ActionResult{OK: err == nil}, DockerErr(err)
}

// PushImage pushes an image reference to a registry, optionally authenticating.
func PushImage(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	var req struct {
		Image    string `json:"image"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	ref := strings.TrimSpace(stringDefault(strings.TrimSpace(req.Image), strings.TrimSpace(rc.Param("id"))))
	if ref == "" {
		return nil, fmt.Errorf("%w: image reference is required", plugin.ErrInvalidInput)
	}
	opts := dockerclient.ImagePushOptions{}
	if auth, err := encodeRegistryAuth(req.Username, req.Password); err != nil {
		return nil, err
	} else if auth != "" {
		opts.RegistryAuth = auth
	}
	resp, err := s.cli.ImagePush(rc.Ctx, ref, opts)
	if err != nil {
		return nil, DockerErr(err)
	}
	defer func() { _ = resp.Close() }()
	if err := resp.Wait(rc.Ctx); err != nil {
		return nil, DockerErr(err)
	}
	return ActionResult{OK: true}, nil
}

// BuildImage builds an image from an inline Dockerfile, tagging the result.
func BuildImage(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	var req struct {
		Tag        string `json:"tag" validate:"required"`
		Dockerfile string `json:"dockerfile" validate:"required"`
		Pull       bool   `json:"pull"`
		NoCache    bool   `json:"no_cache"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	tag := strings.TrimSpace(req.Tag)
	if tag == "" {
		return nil, fmt.Errorf("%w: image tag is required", plugin.ErrInvalidInput)
	}
	if strings.TrimSpace(req.Dockerfile) == "" {
		return nil, fmt.Errorf("%w: Dockerfile is required", plugin.ErrInvalidInput)
	}
	const dockerfileName = "Dockerfile"
	ctxTar, err := tarDockerfile(dockerfileName, req.Dockerfile)
	if err != nil {
		return nil, err
	}
	res, err := s.cli.ImageBuild(rc.Ctx, ctxTar, dockerclient.ImageBuildOptions{
		Tags:        []string{tag},
		Dockerfile:  dockerfileName,
		Remove:      true,
		ForceRemove: true,
		PullParent:  req.Pull,
		NoCache:     req.NoCache,
	})
	if err != nil {
		return nil, DockerErr(err)
	}
	defer func() { _ = res.Body.Close() }()
	output, buildErr := collectBuildOutput(res.Body)
	if buildErr != nil {
		return nil, buildErr
	}
	return BuildResult{OK: true, Tag: tag, Output: output}, nil
}

// BuildResult carries the collected daemon build log back to the generic toast.
type BuildResult struct {
	OK     bool   `json:"ok"`
	Tag    string `json:"tag"`
	Output string `json:"output,omitempty"`
}

func ConnectNetwork(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	var req struct {
		Container string `json:"container" validate:"required"`
		Aliases   string `json:"aliases"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	target := strings.TrimSpace(req.Container)
	if err := validateResourceName(target); err != nil {
		return nil, err
	}
	var endpoint *network.EndpointSettings
	if aliases := nonEmptyLines(req.Aliases); len(aliases) > 0 {
		endpoint = &network.EndpointSettings{Aliases: aliases}
	}
	_, err = s.cli.NetworkConnect(rc.Ctx, rc.Param("id"), dockerclient.NetworkConnectOptions{
		Container:      target,
		EndpointConfig: endpoint,
	})
	return ActionResult{OK: err == nil}, DockerErr(err)
}

func DisconnectNetwork(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	var req struct {
		Container string `json:"container" validate:"required"`
		Force     bool   `json:"force"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	target := strings.TrimSpace(req.Container)
	if err := validateResourceName(target); err != nil {
		return nil, err
	}
	_, err = s.cli.NetworkDisconnect(rc.Ctx, rc.Param("id"), dockerclient.NetworkDisconnectOptions{
		Container: target,
		Force:     req.Force,
	})
	return ActionResult{OK: err == nil}, DockerErr(err)
}

// ComposeResult summarises how many of a project's containers an op touched.
type ComposeResult struct {
	OK        bool `json:"ok"`
	Affected  int  `json:"affected"`
	Succeeded int  `json:"succeeded"`
}

// ComposeUp starts every container belonging to a Compose project. The daemon
// has no Compose engine, so this orchestrates over the project's existing
// (label-derived) containers rather than reading a Compose file.
func ComposeUp(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	ids, err := composeContainerIDs(rc, s, rc.Param("project"))
	if err != nil {
		return nil, err
	}
	res := ComposeResult{OK: true, Affected: len(ids)}
	for _, id := range ids {
		if _, err := s.cli.ContainerStart(rc.Ctx, id, dockerclient.ContainerStartOptions{}); err == nil {
			res.Succeeded++
		}
	}
	return res, nil
}

// ComposeDown stops and removes every container belonging to a Compose project.
func ComposeDown(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	ids, err := composeContainerIDs(rc, s, rc.Param("project"))
	if err != nil {
		return nil, err
	}
	res := ComposeResult{OK: true, Affected: len(ids)}
	for _, id := range ids {
		_, _ = s.cli.ContainerStop(rc.Ctx, id, dockerclient.ContainerStopOptions{})
		_, err := s.cli.ContainerRemove(rc.Ctx, id, dockerclient.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
		// A container started with --rm self-removes on stop, so a follow-up
		// remove can race to a not-found or an in-progress conflict; the
		// teardown still succeeded.
		if err == nil || cerrdefs.IsNotFound(err) || cerrdefs.IsConflict(err) {
			res.Succeeded++
		}
	}
	return res, nil
}

func composeContainerIDs(rc *plugin.RequestContext, s *Session, project string) ([]string, error) {
	project = strings.TrimSpace(project)
	if project == "" {
		return nil, fmt.Errorf("%w: compose project is required", plugin.ErrInvalidInput)
	}
	list, err := s.cli.ContainerList(rc.Ctx, dockerclient.ContainerListOptions{All: true})
	if err != nil {
		return nil, DockerErr(err)
	}
	ids := make([]string, 0, len(list.Items))
	for _, c := range list.Items {
		if c.Labels[ComposeProjectLabel] == project {
			ids = append(ids, c.ID)
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("%w: no containers for compose project %q", plugin.ErrNotFound, project)
	}
	return ids, nil
}

// normalizeKillSignal upper-cases, normalises (SIG prefix), and validates the
// requested signal against the allow-list, defaulting to SIGKILL.
func normalizeKillSignal(raw string) (string, error) {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	if raw == "" {
		return "SIGKILL", nil
	}
	if !strings.HasPrefix(raw, "SIG") {
		raw = "SIG" + raw
	}
	for _, allowed := range killSignals {
		if raw == allowed {
			return raw, nil
		}
	}
	return "", fmt.Errorf("%w: unsupported kill signal %q", plugin.ErrInvalidInput, raw)
}

// validateResourceName rejects empty or whitespace-bearing container/network
// references before they reach the daemon.
func validateResourceName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: name is required", plugin.ErrInvalidInput)
	}
	if strings.ContainsAny(name, " \t\n\r/") {
		return fmt.Errorf("%w: invalid name %q", plugin.ErrInvalidInput, name)
	}
	return nil
}

// encodeRegistryAuth base64-encodes registry credentials for the push header.
// It returns "" when no credentials were supplied so the daemon falls back to
// its configured auth. Credentials are never logged.
func encodeRegistryAuth(username, password string) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" && password == "" {
		return "", nil
	}
	cfg := registry.AuthConfig{Username: username, Password: password}
	buf, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("%w: encode registry auth", plugin.ErrInvalidInput)
	}
	return base64.URLEncoding.EncodeToString(buf), nil
}

// tarDockerfile wraps an inline Dockerfile in a tar build context.
func tarDockerfile(name, content string) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	body := []byte(content)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(body))}); err != nil {
		return nil, fmt.Errorf("%w: build context: %v", plugin.ErrUnavailable, err)
	}
	if _, err := tw.Write(body); err != nil {
		return nil, fmt.Errorf("%w: build context: %v", plugin.ErrUnavailable, err)
	}
	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("%w: build context: %v", plugin.ErrUnavailable, err)
	}
	return &buf, nil
}

// collectBuildOutput drains the daemon's streamed build log, surfacing a build
// error reported mid-stream (the HTTP call itself succeeds even when the build
// fails).
func collectBuildOutput(r io.Reader) (string, error) {
	dec := json.NewDecoder(r)
	var out strings.Builder
	for {
		var msg struct {
			Stream string `json:"stream"`
			Error  string `json:"error"`
		}
		if err := dec.Decode(&msg); err != nil {
			if err == io.EOF {
				break
			}
			return out.String(), nil
		}
		if msg.Error != "" {
			return out.String(), fmt.Errorf("%w: %s", plugin.ErrInvalidInput, strings.TrimSpace(msg.Error))
		}
		out.WriteString(msg.Stream)
	}
	return out.String(), nil
}

func KillContainerSchema() *plugin.Schema {
	options := make([]plugin.Option, 0, len(killSignals))
	for _, sig := range killSignals {
		options = append(options, plugin.Option{Label: sig, Value: sig})
	}
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Kill", Fields: []plugin.Field{
		{Key: "signal", Label: "Signal", Type: plugin.FieldSelect, Default: "SIGKILL", Options: options, Help: "Signal to send to the container's main process."},
	}}}}
}

func RenameContainerSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Rename", Fields: []plugin.Field{
		{Key: "name", Label: "New name", Type: plugin.FieldText, Required: true, Placeholder: "web", Validators: []plugin.Validator{{Type: plugin.ValidatorRegex, Value: `^[A-Za-z0-9][A-Za-z0-9_.-]*$`, Message: "Use letters, numbers, dots, underscores, or dashes."}}},
	}}}}
}

func ImageTagSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Tag", Fields: []plugin.Field{
		{Key: "target", Label: "Target reference", Type: plugin.FieldText, Required: true, Placeholder: "registry.example.com/app:1.0", Help: "New repository:tag for this image."},
	}}}}
}

func ImagePushSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Push", Fields: []plugin.Field{
		{Key: "image", Label: "Image reference", Type: plugin.FieldText, Placeholder: "registry.example.com/app:1.0", Help: "Reference to push. Defaults to the selected image."},
		{Key: "username", Label: "Registry username", Type: plugin.FieldText, Placeholder: "robot$ci"},
		{Key: "password", Label: "Registry password", Type: plugin.FieldPassword, Secret: true},
	}}}}
}

func ImageBuildSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Build", Fields: []plugin.Field{
		{Key: "tag", Label: "Image tag", Type: plugin.FieldText, Required: true, Placeholder: "app:latest"},
		{Key: "dockerfile", Label: "Dockerfile", Type: plugin.FieldTextarea, Required: true, Placeholder: "FROM alpine:3.20\nRUN echo hello"},
		{Key: "pull", Label: "Always pull base image", Type: plugin.FieldToggle},
		{Key: "no_cache", Label: "No cache", Type: plugin.FieldToggle},
	}}}}
}

func NetworkConnectSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Connect", Fields: []plugin.Field{
		{Key: "container", Label: "Container", Type: plugin.FieldText, Required: true, Placeholder: "web", Help: "Container name or id to attach to this network."},
		{Key: "aliases", Label: "Network aliases", Type: plugin.FieldTextarea, Placeholder: "api\ninternal", Help: "Optional DNS aliases, one per line."},
	}}}}
}

func NetworkDisconnectSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Disconnect", Fields: []plugin.Field{
		{Key: "container", Label: "Container", Type: plugin.FieldText, Required: true, Placeholder: "web"},
		{Key: "force", Label: "Force", Type: plugin.FieldToggle, Help: "Force the container to disconnect even if it is running."},
	}}}}
}
