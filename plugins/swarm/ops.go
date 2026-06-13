package swarm

import (
	"fmt"
	"strings"

	"github.com/moby/moby/api/types/swarm"
	dockerclient "github.com/moby/moby/client"

	"github.com/charlesng35/shellcn/plugins/shared/dockerengine"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// nodeAvailabilities and nodeRoles are the closed enums the swarm node spec
// accepts; bodies are constrained to these to reject arbitrary input.
var (
	nodeAvailabilities = map[string]swarm.NodeAvailability{
		string(swarm.NodeAvailabilityActive): swarm.NodeAvailabilityActive,
		string(swarm.NodeAvailabilityPause):  swarm.NodeAvailabilityPause,
		string(swarm.NodeAvailabilityDrain):  swarm.NodeAvailabilityDrain,
	}
	nodeRoles = map[string]swarm.NodeRole{
		string(swarm.NodeRoleWorker):  swarm.NodeRoleWorker,
		string(swarm.NodeRoleManager): swarm.NodeRoleManager,
	}
)

func parseAvailability(v string) (swarm.NodeAvailability, error) {
	a, ok := nodeAvailabilities[strings.ToLower(strings.TrimSpace(v))]
	if !ok {
		return "", fmt.Errorf("%w: unknown node availability %q", plugin.ErrInvalidInput, v)
	}
	return a, nil
}

func parseRole(v string) (swarm.NodeRole, error) {
	r, ok := nodeRoles[strings.ToLower(strings.TrimSpace(v))]
	if !ok {
		return "", fmt.Errorf("%w: unknown node role %q", plugin.ErrInvalidInput, v)
	}
	return r, nil
}

// parseEnv turns "KEY=VALUE" lines into a docker env slice, skipping blanks and
// rejecting entries without a key.
func parseEnv(raw string) ([]string, error) {
	out := []string{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, _, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("%w: env entry %q must be KEY=VALUE", plugin.ErrInvalidInput, line)
		}
		out = append(out, line)
	}
	return out, nil
}

// applyServiceUpdate mutates a service spec in place from the update request,
// touching only the fields the caller provided.
func applyServiceUpdate(spec *swarm.ServiceSpec, req serviceUpdateRequest) error {
	if spec.TaskTemplate.ContainerSpec == nil {
		spec.TaskTemplate.ContainerSpec = &swarm.ContainerSpec{}
	}
	if img := strings.TrimSpace(req.Image); img != "" {
		spec.TaskTemplate.ContainerSpec.Image = img
	}
	if req.Env != nil {
		env, err := parseEnv(*req.Env)
		if err != nil {
			return err
		}
		spec.TaskTemplate.ContainerSpec.Env = env
	}
	if req.Replicas != nil {
		if spec.Mode.Replicated == nil {
			return fmt.Errorf("%w: only replicated services accept a replica count", plugin.ErrInvalidInput)
		}
		replicas := *req.Replicas
		spec.Mode.Replicated.Replicas = &replicas
	}
	return nil
}

type serviceUpdateRequest struct {
	Image    string  `json:"image"`
	Env      *string `json:"env"`
	Replicas *uint64 `json:"replicas"`
}

func serviceUpdateSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Update service", Fields: []plugin.Field{
		{Key: "image", Label: "Image", Type: plugin.FieldAutocomplete, Placeholder: "nginx:1.27", Help: "Leave blank to keep the current image."},
		{Key: "replicas", Label: "Replicas", Type: plugin.FieldStepper, Help: "Replicated services only.", Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 0}, {Type: plugin.ValidatorMax, Value: 10000}}},
		{Key: "env", Label: "Environment", Type: plugin.FieldTextarea, Placeholder: "KEY=VALUE", Help: "One KEY=VALUE per line; replaces the current environment when set."},
	}}}}
}

func nodeUpdateSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Update node", Fields: []plugin.Field{
		{Key: "availability", Label: "Availability", Type: plugin.FieldSelect, Options: []plugin.Option{
			{Label: "Active", Value: string(swarm.NodeAvailabilityActive)},
			{Label: "Pause", Value: string(swarm.NodeAvailabilityPause)},
			{Label: "Drain", Value: string(swarm.NodeAvailabilityDrain)},
		}, Help: "Drain reschedules tasks off this node."},
		{Key: "role", Label: "Role", Type: plugin.FieldSelect, Options: []plugin.Option{
			{Label: "Worker", Value: string(swarm.NodeRoleWorker)},
			{Label: "Manager", Value: string(swarm.NodeRoleManager)},
		}, Help: "Promote a worker to manager or demote a manager to worker."},
	}}}}
}

func stackDeploySchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Deploy stack", Fields: []plugin.Field{
		{Key: "name", Label: "Stack name", Type: plugin.FieldText, Required: true, Placeholder: "my-stack", Validators: []plugin.Validator{{Type: plugin.ValidatorRegex, Value: `^[A-Za-z0-9][A-Za-z0-9_.-]*$`, Message: "Use letters, numbers, dots, underscores, or dashes."}}},
		{Key: "spec", Label: "Service spec JSON", Type: plugin.FieldJSON, Required: true, Help: "JSON array of Docker service specs. Each spec is created or updated under the stack namespace; Compose YAML is not accepted here."},
	}}}}
}

// updateService applies an image/replicas/env change against the service's
// current spec version.
func updateService(rc *plugin.RequestContext) (any, error) {
	cli, err := client(rc)
	if err != nil {
		return nil, err
	}
	var req serviceUpdateRequest
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	res, err := cli.ServiceInspect(rc.Ctx, rc.Param("id"), dockerclient.ServiceInspectOptions{})
	if err != nil {
		return nil, dockerengine.DockerErr(err)
	}
	svc := res.Service
	if err := applyServiceUpdate(&svc.Spec, req); err != nil {
		return nil, err
	}
	if _, err := cli.ServiceUpdate(rc.Ctx, svc.ID, dockerclient.ServiceUpdateOptions{Version: svc.Version, Spec: svc.Spec}); err != nil {
		return nil, dockerengine.DockerErr(err)
	}
	return dockerengine.ActionResult{OK: true}, nil
}

// rollbackService asks the daemon to roll the service back to its PreviousSpec.
func rollbackService(rc *plugin.RequestContext) (any, error) {
	cli, err := client(rc)
	if err != nil {
		return nil, err
	}
	res, err := cli.ServiceInspect(rc.Ctx, rc.Param("id"), dockerclient.ServiceInspectOptions{})
	if err != nil {
		return nil, dockerengine.DockerErr(err)
	}
	svc := res.Service
	if svc.PreviousSpec == nil {
		return nil, fmt.Errorf("%w: service has no previous spec to roll back to", plugin.ErrInvalidInput)
	}
	if _, err := cli.ServiceUpdate(rc.Ctx, svc.ID, dockerclient.ServiceUpdateOptions{Version: svc.Version, Spec: svc.Spec, Rollback: "previous"}); err != nil {
		return nil, dockerengine.DockerErr(err)
	}
	return dockerengine.ActionResult{OK: true}, nil
}

// updateNode sets a node's availability and/or role against its current spec
// version. Role changes promote (worker->manager) or demote (manager->worker).
func updateNode(rc *plugin.RequestContext) (any, error) {
	cli, err := client(rc)
	if err != nil {
		return nil, err
	}
	var req struct {
		Availability string `json:"availability"`
		Role         string `json:"role"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Availability) == "" && strings.TrimSpace(req.Role) == "" {
		return nil, fmt.Errorf("%w: provide availability and/or role", plugin.ErrInvalidInput)
	}
	res, err := cli.NodeInspect(rc.Ctx, rc.Param("id"), dockerclient.NodeInspectOptions{})
	if err != nil {
		return nil, dockerengine.DockerErr(err)
	}
	node := res.Node
	if strings.TrimSpace(req.Availability) != "" {
		avail, err := parseAvailability(req.Availability)
		if err != nil {
			return nil, err
		}
		node.Spec.Availability = avail
	}
	if strings.TrimSpace(req.Role) != "" {
		role, err := parseRole(req.Role)
		if err != nil {
			return nil, err
		}
		node.Spec.Role = role
	}
	if _, err := cli.NodeUpdate(rc.Ctx, node.ID, dockerclient.NodeUpdateOptions{Version: node.Version, Spec: node.Spec}); err != nil {
		return nil, dockerengine.DockerErr(err)
	}
	return dockerengine.ActionResult{OK: true}, nil
}

type stackDeployRequest struct {
	Name string              `json:"name"`
	Spec []swarm.ServiceSpec `json:"spec"`
}

// deployStack creates or updates the services of a stack from a set of service
// specs. Each spec is stamped with the stack namespace label and matched to an
// existing service by name within that namespace; matches are updated in place,
// the rest are created. This is the API-level subset of `docker stack deploy`:
// it applies services only — declarative configs/secrets/networks and pruning
// of removed services are not handled here (the CLI builds those client-side).
func deployStack(rc *plugin.RequestContext) (any, error) {
	cli, err := client(rc)
	if err != nil {
		return nil, err
	}
	var req stackDeployRequest
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("%w: stack name is required", plugin.ErrInvalidInput)
	}
	if len(req.Spec) == 0 {
		return nil, fmt.Errorf("%w: at least one service spec is required", plugin.ErrInvalidInput)
	}
	for i := range req.Spec {
		if strings.TrimSpace(req.Spec[i].Name) == "" {
			return nil, fmt.Errorf("%w: service spec %d is missing a name", plugin.ErrInvalidInput, i)
		}
	}

	existing, err := stackServiceVersions(rc, cli, name)
	if err != nil {
		return nil, err
	}
	for i := range req.Spec {
		spec := req.Spec[i]
		stampStackNamespace(&spec, name)
		if cur, ok := existing[spec.Name]; ok {
			if _, err := cli.ServiceUpdate(rc.Ctx, cur.id, dockerclient.ServiceUpdateOptions{Version: cur.version, Spec: spec}); err != nil {
				return nil, dockerengine.DockerErr(err)
			}
			continue
		}
		if _, err := cli.ServiceCreate(rc.Ctx, dockerclient.ServiceCreateOptions{Spec: spec}); err != nil {
			return nil, dockerengine.DockerErr(err)
		}
	}
	return dockerengine.ActionResult{OK: true}, nil
}

type serviceVersion struct {
	id      string
	version swarm.Version
}

func stackServiceVersions(rc *plugin.RequestContext, cli *dockerclient.Client, stack string) (map[string]serviceVersion, error) {
	res, err := cli.ServiceList(rc.Ctx, dockerclient.ServiceListOptions{
		Filters: make(dockerclient.Filters).Add("label", stackNamespaceLabel+"="+stack),
	})
	if err != nil {
		return nil, dockerengine.DockerErr(err)
	}
	out := make(map[string]serviceVersion, len(res.Items))
	for _, s := range res.Items {
		out[s.Spec.Name] = serviceVersion{id: s.ID, version: s.Version}
	}
	return out, nil
}

// stampStackNamespace applies the stack namespace label so the service is
// recognised as part of the stack, mirroring `docker stack deploy`.
func stampStackNamespace(spec *swarm.ServiceSpec, stack string) {
	if spec.Labels == nil {
		spec.Labels = map[string]string{}
	}
	spec.Labels[stackNamespaceLabel] = stack
}
