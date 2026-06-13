package dockerengine

import (
	"context"
	"fmt"
	"io"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	dockerclient "github.com/moby/moby/client"

	"github.com/charlesng35/shellcn/plugins/shared/termshell"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

const (
	engineShellImage     = "docker.io/library/docker:28-cli"
	engineShellName      = "shellcn-docker-shell"
	engineShellLabel     = "shellcn.io/docker-shell"
	engineShellHome      = "/root"
	engineShellKeepalive = "trap : TERM INT; sleep 2147483647 & wait"
)

func EngineShellStream(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, err := sess(rc)
	if err != nil {
		return err
	}
	id, err := s.ensureEngineShellContainer(client.Context())
	if err != nil {
		termshell.WriteExecError(client, err)
		return err
	}

	params := streamParams(rc)
	params["id"] = id
	params["workdir"] = engineShellHome
	ch, err := s.openExec(client.Context(), params)
	if err != nil {
		termshell.WriteExecError(client, err)
		return err
	}
	defer func() { _ = ch.Close() }()

	errc := make(chan error, 2)
	go func() {
		_, err := io.Copy(client, ch)
		errc <- err
	}()
	go func() {
		errc <- plugin.CopyTerminalInput(ch, client)
	}()
	select {
	case <-client.Context().Done():
		return nil
	case err := <-errc:
		if err == io.EOF {
			return nil
		}
		if err != nil {
			termshell.WriteExecError(client, err)
		}
		return err
	}
}

func (s *Session) ensureEngineShellContainer(ctx context.Context) (string, error) {
	inspect, err := s.cli.ContainerInspect(ctx, engineShellName, dockerclient.ContainerInspectOptions{})
	if err == nil {
		if inspect.Container.State != nil && inspect.Container.State.Running {
			return inspect.Container.ID, nil
		}
		if inspect.Container.State != nil && inspect.Container.State.Dead {
			_, _ = s.cli.ContainerRemove(ctx, inspect.Container.ID, dockerclient.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
			return s.createEngineShellContainer(ctx)
		}
		if _, err := s.cli.ContainerStart(ctx, inspect.Container.ID, dockerclient.ContainerStartOptions{}); err != nil {
			return "", DockerErr(err)
		}
		return inspect.Container.ID, nil
	}
	if !cerrdefs.IsNotFound(err) {
		return "", DockerErr(err)
	}
	return s.createEngineShellContainer(ctx)
}

func (s *Session) createEngineShellContainer(ctx context.Context) (string, error) {
	if err := s.pullImage(ctx, engineShellImage); err != nil {
		return "", err
	}
	created, err := s.cli.ContainerCreate(ctx, engineShellCreateOptions(s.endpoint))
	if err != nil {
		return "", DockerErr(err)
	}
	if _, err := s.cli.ContainerStart(ctx, created.ID, dockerclient.ContainerStartOptions{}); err != nil {
		return "", DockerErr(err)
	}
	return created.ID, nil
}

func (s *Session) pullImage(ctx context.Context, ref string) error {
	pull, err := s.cli.ImagePull(ctx, ref, dockerclient.ImagePullOptions{})
	if err != nil {
		return DockerErr(err)
	}
	if err := pull.Wait(ctx); err != nil {
		_ = pull.Close()
		return DockerErr(err)
	}
	if err := pull.Close(); err != nil {
		return DockerErr(err)
	}
	return nil
}

func engineShellCreateOptions(ep endpoint) dockerclient.ContainerCreateOptions {
	env := []string{"DOCKER_HOST=unix:///var/run/docker.sock"}
	binds := []string{"/var/run/docker.sock:/var/run/docker.sock:rw"}
	if ep.network == "unix" && ep.address != "" {
		binds = []string{fmt.Sprintf("%s:/var/run/docker.sock:rw", ep.address)}
	}
	if ep.network == "tcp" && ep.address != "" {
		env = []string{fmt.Sprintf("DOCKER_HOST=tcp://%s", ep.address)}
		binds = nil
	}
	return dockerclient.ContainerCreateOptions{
		Name: engineShellName,
		Config: &container.Config{
			Image:      engineShellImage,
			Cmd:        []string{"/bin/sh", "-c", engineShellKeepalive},
			Env:        append(env, "HOME="+engineShellHome),
			OpenStdin:  true,
			Tty:        true,
			WorkingDir: engineShellHome,
			Labels:     engineShellLabels(),
			StopSignal: "SIGTERM",
		},
		HostConfig: &container.HostConfig{
			Binds:         binds,
			RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyUnlessStopped},
		},
	}
}

func engineShellLabels() map[string]string {
	return map[string]string{
		"app.kubernetes.io/managed-by": "shellcn",
		engineShellLabel:               "true",
	}
}
