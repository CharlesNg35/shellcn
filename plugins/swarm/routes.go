package swarm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/api/types/swarm"
	dockerclient "github.com/moby/moby/client"

	"github.com/charlesng35/shellcn/plugins/shared/dockerengine"
	"github.com/charlesng35/shellcn/sdk/plugin"
	"github.com/charlesng35/shellcn/sdk/plugin/webproxy"
)

const stackNamespaceLabel = "com.docker.stack.namespace"

// Routes wires Swarm-namespaced route IDs to the orchestration handlers.
func Routes() []plugin.Route {
	return []plugin.Route{
		{ID: "swarm.overview.list", Method: plugin.MethodGet, Path: "/overview", Permission: "swarm.services.read", Risk: plugin.RiskSafe, AuditEvent: "swarm.overview.list", Handle: dockerengine.OverviewList},
		{ID: "swarm.overview.metrics", Method: plugin.MethodWS, Path: "/overview/metrics", Permission: "swarm.services.read", Risk: plugin.RiskSafe, AuditEvent: "swarm.overview.metrics", Stream: overviewMetrics},
		{ID: "swarm.services.tree", Method: plugin.MethodGet, Path: "/tree/services", Permission: "swarm.services.read", Risk: plugin.RiskSafe, AuditEvent: "swarm.services.tree", Handle: treeServices},
		{ID: "swarm.stacks.tree", Method: plugin.MethodGet, Path: "/tree/stacks", Permission: "swarm.stacks.read", Risk: plugin.RiskSafe, AuditEvent: "swarm.stacks.tree", Handle: treeStacks},
		{ID: "swarm.nodes.tree", Method: plugin.MethodGet, Path: "/tree/nodes", Permission: "swarm.nodes.read", Risk: plugin.RiskSafe, AuditEvent: "swarm.nodes.tree", Handle: treeSwarmNodes},
		{ID: "swarm.tasks.tree", Method: plugin.MethodGet, Path: "/tree/tasks", Permission: "swarm.tasks.read", Risk: plugin.RiskSafe, AuditEvent: "swarm.tasks.tree", Handle: treeTasks},
		{ID: "swarm.services.list", Method: plugin.MethodGet, Path: "/services", Permission: "swarm.services.read", Risk: plugin.RiskSafe, AuditEvent: "swarm.services.list", Handle: listServices},
		{ID: "swarm.stacks.list", Method: plugin.MethodGet, Path: "/stacks", Permission: "swarm.stacks.read", Risk: plugin.RiskSafe, AuditEvent: "swarm.stacks.list", Handle: listStacks},
		{ID: "swarm.nodes.list", Method: plugin.MethodGet, Path: "/nodes", Permission: "swarm.nodes.read", Risk: plugin.RiskSafe, AuditEvent: "swarm.nodes.list", Handle: listNodes},
		{ID: "swarm.tasks.list", Method: plugin.MethodGet, Path: "/tasks", Permission: "swarm.tasks.read", Risk: plugin.RiskSafe, AuditEvent: "swarm.tasks.list", Handle: listTasks},
		{ID: "swarm.service.overview", Method: plugin.MethodGet, Path: "/services/{id}/overview", Permission: "swarm.services.read", Risk: plugin.RiskSafe, AuditEvent: "swarm.service.overview", Handle: serviceOverview},
		{ID: "swarm.service.inspect", Method: plugin.MethodGet, Path: "/services/{id}/inspect", Permission: "swarm.services.read", Risk: plugin.RiskSafe, AuditEvent: "swarm.service.inspect", Handle: inspectService},
		{ID: "swarm.service.tasks", Method: plugin.MethodGet, Path: "/services/{id}/tasks", Permission: "swarm.tasks.read", Risk: plugin.RiskSafe, AuditEvent: "swarm.service.tasks", Handle: serviceTasks},
		{ID: "swarm.service.open", Method: plugin.MethodGet, Path: "/services/{id}/open", Permission: "swarm.services.read", Risk: plugin.RiskSafe, AuditEvent: "swarm.service.open", Input: serviceOpenSchema(), Handle: serviceProxyURL},
		{ID: "swarm.service.open.ports", Method: plugin.MethodGet, Path: "/services/{id}/open/ports", Permission: "swarm.services.read", Risk: plugin.RiskSafe, AuditEvent: "swarm.service.open.ports", Handle: serviceOpenPorts},
		{ID: "swarm.node.overview", Method: plugin.MethodGet, Path: "/nodes/{id}/overview", Permission: "swarm.nodes.read", Risk: plugin.RiskSafe, AuditEvent: "swarm.node.overview", Handle: nodeOverview},
		{ID: "swarm.node.inspect", Method: plugin.MethodGet, Path: "/nodes/{id}/inspect", Permission: "swarm.nodes.read", Risk: plugin.RiskSafe, AuditEvent: "swarm.node.inspect", Handle: inspectNode},
		{ID: "swarm.node.tasks", Method: plugin.MethodGet, Path: "/nodes/{id}/tasks", Permission: "swarm.tasks.read", Risk: plugin.RiskSafe, AuditEvent: "swarm.node.tasks", Handle: nodeTasks},
		{ID: "swarm.task.overview", Method: plugin.MethodGet, Path: "/tasks/{id}/overview", Permission: "swarm.tasks.read", Risk: plugin.RiskSafe, AuditEvent: "swarm.task.overview", Handle: taskOverview},
		{ID: "swarm.task.inspect", Method: plugin.MethodGet, Path: "/tasks/{id}/inspect", Permission: "swarm.tasks.read", Risk: plugin.RiskSafe, AuditEvent: "swarm.task.inspect", Handle: inspectTask},
		{ID: "swarm.stack.overview", Method: plugin.MethodGet, Path: "/stacks/{stack}/overview", Permission: "swarm.stacks.read", Risk: plugin.RiskSafe, AuditEvent: "swarm.stack.overview", Handle: stackOverview},
		{ID: "swarm.stack.services", Method: plugin.MethodGet, Path: "/stacks/{stack}/services", Permission: "swarm.services.read", Risk: plugin.RiskSafe, AuditEvent: "swarm.stack.services", Handle: stackServices},
		{ID: "swarm.service.remove", Method: plugin.MethodDelete, Path: "/services/{id}", Permission: "swarm.services.delete", Risk: plugin.RiskDestructive, AuditEvent: "swarm.service.remove", Handle: removeService},
		{ID: "swarm.service.scale", Method: plugin.MethodPost, Path: "/services/{id}/scale", Permission: "swarm.services.write", Risk: plugin.RiskWrite, AuditEvent: "swarm.service.scale", Input: scaleSchema(), Handle: scaleService},
		{ID: "swarm.service.update", Method: plugin.MethodPost, Path: "/services/{id}/update", Permission: "swarm.services.write", Risk: plugin.RiskWrite, AuditEvent: "swarm.service.update", Input: serviceUpdateSchema(), Handle: updateService},
		{ID: "swarm.service.rollback", Method: plugin.MethodPost, Path: "/services/{id}/rollback", Permission: "swarm.services.write", Risk: plugin.RiskWrite, AuditEvent: "swarm.service.rollback", Handle: rollbackService},
		{ID: "swarm.node.update", Method: plugin.MethodPost, Path: "/nodes/{id}/update", Permission: "swarm.nodes.write", Risk: plugin.RiskWrite, AuditEvent: "swarm.node.update", Input: nodeUpdateSchema(), Handle: updateNode},
		{ID: "swarm.stack.deploy", Method: plugin.MethodPost, Path: "/stacks/deploy", Permission: "swarm.stacks.write", Risk: plugin.RiskWrite, AuditEvent: "swarm.stack.deploy", Input: stackDeploySchema(), Handle: deployStack},
		{ID: "swarm.stack.remove", Method: plugin.MethodDelete, Path: "/stacks/{stack}", Permission: "swarm.stacks.delete", Risk: plugin.RiskDestructive, AuditEvent: "swarm.stack.remove", Handle: removeStack},
		{ID: "swarm.service.logs", Method: plugin.MethodWS, Path: "/services/{id}/logs", Permission: "swarm.services.logs", Risk: plugin.RiskSafe, AuditEvent: "swarm.service.logs", Input: dockerengine.LogsSchema(), Stream: serviceLogsStream},
		{ID: "swarm.events.watch", Method: plugin.MethodWS, Path: "/events", Permission: "swarm.services.read", Risk: plugin.RiskSafe, AuditEvent: "swarm.events.watch", Stream: watchServiceEvents},
	}
}

func client(rc *plugin.RequestContext) (*dockerclient.Client, error) {
	s, err := dockerengine.Unwrap(rc.Session)
	if err != nil {
		return nil, err
	}
	return s.Client(), nil
}

func overviewMetrics(rc *plugin.RequestContext, stream plugin.ClientStream) error {
	return dockerengine.MetricsLoop(rc, stream, func(context.Context) map[string]any {
		return swarmFrame(rc)
	})
}

func swarmFrame(rc *plugin.RequestContext) map[string]any {
	frame := map[string]any{}
	cli, err := client(rc)
	if err != nil {
		return frame
	}
	if res, err := cli.ServiceList(rc.Ctx, dockerclient.ServiceListOptions{}); err == nil {
		frame["services"] = len(res.Items)
	}
	if res, err := cli.NodeList(rc.Ctx, dockerclient.NodeListOptions{}); err == nil {
		frame["nodes"] = len(res.Items)
	}
	if res, err := cli.TaskList(rc.Ctx, dockerclient.TaskListOptions{}); err == nil {
		frame["tasks"] = len(res.Items)
	}
	if rows, err := stackRows(rc); err == nil {
		frame["stacks"] = len(rows)
	}
	return frame
}

func listServices(rc *plugin.RequestContext) (any, error) {
	cli, err := client(rc)
	if err != nil {
		return nil, err
	}
	res, err := cli.ServiceList(rc.Ctx, dockerclient.ServiceListOptions{Status: true})
	if err != nil {
		return nil, dockerengine.DockerErr(err)
	}
	return dockerengine.PageRows(rc, serviceRows(res.Items))
}

func listNodes(rc *plugin.RequestContext) (any, error) {
	cli, err := client(rc)
	if err != nil {
		return nil, err
	}
	res, err := cli.NodeList(rc.Ctx, dockerclient.NodeListOptions{})
	if err != nil {
		return nil, dockerengine.DockerErr(err)
	}
	return dockerengine.PageRows(rc, nodeRows(res.Items))
}

func listTasks(rc *plugin.RequestContext) (any, error) {
	cli, err := client(rc)
	if err != nil {
		return nil, err
	}
	res, err := cli.TaskList(rc.Ctx, dockerclient.TaskListOptions{})
	if err != nil {
		return nil, dockerengine.DockerErr(err)
	}
	return dockerengine.PageRows(rc, taskRows(res.Items, serviceNames(rc)))
}

func listStacks(rc *plugin.RequestContext) (any, error) {
	rows, err := stackRows(rc)
	if err != nil {
		return nil, err
	}
	return dockerengine.PageRows(rc, rows)
}

func treeServices(rc *plugin.RequestContext) (any, error) {
	return buildTree(rc, "service", icon("workflow"), listServices)
}

func treeStacks(rc *plugin.RequestContext) (any, error) {
	return buildTree(rc, "stack", icon("layers"), listStacks)
}

func treeSwarmNodes(rc *plugin.RequestContext) (any, error) {
	return buildTree(rc, "node", icon("server"), listNodes)
}

func treeTasks(rc *plugin.RequestContext) (any, error) {
	return buildTree(rc, "task", icon("list-checks"), listTasks)
}

func serviceOverview(rc *plugin.RequestContext) (any, error) {
	cli, err := client(rc)
	if err != nil {
		return nil, err
	}
	res, err := cli.ServiceInspect(rc.Ctx, rc.Param("id"), dockerclient.ServiceInspectOptions{})
	if err != nil {
		return nil, dockerengine.DockerErr(err)
	}
	s := res.Service
	mode, replicas := serviceMode(s)
	out := dockerengine.Row{
		"id":        s.ID,
		"name":      s.Spec.Name,
		"mode":      mode,
		"replicas":  replicas,
		"image":     cleanImage(specImage(s.Spec.TaskTemplate)),
		"ports":     servicePorts(s.Endpoint.Ports),
		"stack":     s.Spec.Labels[stackNamespaceLabel],
		"createdAt": s.CreatedAt.UTC().Format(time.RFC3339),
		"updatedAt": s.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if s.UpdateStatus != nil {
		out["updateState"] = string(s.UpdateStatus.State)
		out["updateMessage"] = s.UpdateStatus.Message
	}
	return out, nil
}

func inspectService(rc *plugin.RequestContext) (any, error) {
	cli, err := client(rc)
	if err != nil {
		return nil, err
	}
	res, err := cli.ServiceInspect(rc.Ctx, rc.Param("id"), dockerclient.ServiceInspectOptions{})
	if err != nil {
		return nil, dockerengine.DockerErr(err)
	}
	return dockerengine.RawOrValue(res.Raw, res.Service)
}

func serviceTasks(rc *plugin.RequestContext) (any, error) {
	cli, err := client(rc)
	if err != nil {
		return nil, err
	}
	res, err := cli.TaskList(rc.Ctx, dockerclient.TaskListOptions{Filters: make(dockerclient.Filters).Add("service", rc.Param("id"))})
	if err != nil {
		return nil, dockerengine.DockerErr(err)
	}
	return dockerengine.PageRows(rc, taskRows(res.Items, serviceNames(rc)))
}

func nodeOverview(rc *plugin.RequestContext) (any, error) {
	cli, err := client(rc)
	if err != nil {
		return nil, err
	}
	res, err := cli.NodeInspect(rc.Ctx, rc.Param("id"), dockerclient.NodeInspectOptions{})
	if err != nil {
		return nil, dockerengine.DockerErr(err)
	}
	n := res.Node
	leader := false
	reachability := ""
	if n.ManagerStatus != nil {
		leader = n.ManagerStatus.Leader
		reachability = string(n.ManagerStatus.Reachability)
	}
	return dockerengine.Row{
		"id":           n.ID,
		"name":         nodeName(n),
		"role":         string(n.Spec.Role),
		"availability": string(n.Spec.Availability),
		"state":        string(n.Status.State),
		"leader":       leader,
		"reachability": reachability,
		"address":      n.Status.Addr,
		"engine":       n.Description.Engine.EngineVersion,
		"os":           n.Description.Platform.OS,
		"arch":         n.Description.Platform.Architecture,
		"cpus":         n.Description.Resources.NanoCPUs / 1e9,
		"memoryBytes":  n.Description.Resources.MemoryBytes,
		"createdAt":    n.CreatedAt.UTC().Format(time.RFC3339),
	}, nil
}

func inspectNode(rc *plugin.RequestContext) (any, error) {
	cli, err := client(rc)
	if err != nil {
		return nil, err
	}
	res, err := cli.NodeInspect(rc.Ctx, rc.Param("id"), dockerclient.NodeInspectOptions{})
	if err != nil {
		return nil, dockerengine.DockerErr(err)
	}
	return dockerengine.RawOrValue(res.Raw, res.Node)
}

func nodeTasks(rc *plugin.RequestContext) (any, error) {
	cli, err := client(rc)
	if err != nil {
		return nil, err
	}
	res, err := cli.TaskList(rc.Ctx, dockerclient.TaskListOptions{Filters: make(dockerclient.Filters).Add("node", rc.Param("id"))})
	if err != nil {
		return nil, dockerengine.DockerErr(err)
	}
	return dockerengine.PageRows(rc, taskRows(res.Items, serviceNames(rc)))
}

func taskOverview(rc *plugin.RequestContext) (any, error) {
	cli, err := client(rc)
	if err != nil {
		return nil, err
	}
	res, err := cli.TaskInspect(rc.Ctx, rc.Param("id"), dockerclient.TaskInspectOptions{})
	if err != nil {
		return nil, dockerengine.DockerErr(err)
	}
	t := res.Task
	return dockerengine.Row{
		"id":           t.ID,
		"service":      t.ServiceID,
		"node":         t.NodeID,
		"slot":         t.Slot,
		"desiredState": string(t.DesiredState),
		"state":        string(t.Status.State),
		"message":      t.Status.Message,
		"error":        t.Status.Err,
		"image":        cleanImage(specImage(t.Spec)),
		"createdAt":    t.CreatedAt.UTC().Format(time.RFC3339),
	}, nil
}

func inspectTask(rc *plugin.RequestContext) (any, error) {
	cli, err := client(rc)
	if err != nil {
		return nil, err
	}
	res, err := cli.TaskInspect(rc.Ctx, rc.Param("id"), dockerclient.TaskInspectOptions{})
	if err != nil {
		return nil, dockerengine.DockerErr(err)
	}
	return dockerengine.RawOrValue(res.Raw, res.Task)
}

func stackOverview(rc *plugin.RequestContext) (any, error) {
	rows, err := stackRows(rc)
	if err != nil {
		return nil, err
	}
	stack := rc.Param("stack")
	for _, r := range rows {
		if r["name"] == stack {
			return r, nil
		}
	}
	return nil, plugin.ErrNotFound
}

func stackServices(rc *plugin.RequestContext) (any, error) {
	cli, err := client(rc)
	if err != nil {
		return nil, err
	}
	res, err := cli.ServiceList(rc.Ctx, dockerclient.ServiceListOptions{
		Status:  true,
		Filters: make(dockerclient.Filters).Add("label", stackNamespaceLabel+"="+rc.Param("stack")),
	})
	if err != nil {
		return nil, dockerengine.DockerErr(err)
	}
	return dockerengine.PageRows(rc, serviceRows(res.Items))
}

func removeService(rc *plugin.RequestContext) (any, error) {
	cli, err := client(rc)
	if err != nil {
		return nil, err
	}
	_, err = cli.ServiceRemove(rc.Ctx, rc.Param("id"), dockerclient.ServiceRemoveOptions{})
	return dockerengine.ActionResult{OK: err == nil}, dockerengine.DockerErr(err)
}

func removeStack(rc *plugin.RequestContext) (any, error) {
	cli, err := client(rc)
	if err != nil {
		return nil, err
	}
	stack := strings.TrimSpace(rc.Param("stack"))
	if stack == "" {
		return nil, fmt.Errorf("%w: stack name is required", plugin.ErrInvalidInput)
	}
	res, err := cli.ServiceList(rc.Ctx, dockerclient.ServiceListOptions{
		Filters: make(dockerclient.Filters).Add("label", stackNamespaceLabel+"="+stack),
	})
	if err != nil {
		return nil, dockerengine.DockerErr(err)
	}
	if len(res.Items) == 0 {
		return nil, plugin.ErrNotFound
	}
	var removed int
	for _, svc := range res.Items {
		if _, err := cli.ServiceRemove(rc.Ctx, svc.ID, dockerclient.ServiceRemoveOptions{}); err != nil {
			return nil, dockerengine.DockerErr(err)
		}
		removed++
	}
	return map[string]any{"ok": true, "removed": removed}, nil
}

func scaleSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Scale", Fields: []plugin.Field{
		{Key: "replicas", Label: "Replicas", Type: plugin.FieldStepper, Required: true, Default: 1, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 0}}},
	}}}}
}

// scaleService sets the replica count on a replicated service via an in-place
// ServiceUpdate against the current spec version.
func scaleService(rc *plugin.RequestContext) (any, error) {
	cli, err := client(rc)
	if err != nil {
		return nil, err
	}
	var req struct {
		Replicas uint64 `json:"replicas"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	res, err := cli.ServiceInspect(rc.Ctx, rc.Param("id"), dockerclient.ServiceInspectOptions{})
	if err != nil {
		return nil, dockerengine.DockerErr(err)
	}
	svc := res.Service
	if svc.Spec.Mode.Replicated == nil {
		return nil, fmt.Errorf("%w: only replicated services can be scaled", plugin.ErrInvalidInput)
	}
	replicas := req.Replicas
	svc.Spec.Mode.Replicated.Replicas = &replicas
	if _, err := cli.ServiceUpdate(rc.Ctx, svc.ID, dockerclient.ServiceUpdateOptions{Version: svc.Version, Spec: svc.Spec}); err != nil {
		return nil, dockerengine.DockerErr(err)
	}
	return dockerengine.ActionResult{OK: true}, nil
}

func serviceLogsStream(rc *plugin.RequestContext, stream plugin.ClientStream) error {
	cli, err := client(rc)
	if err != nil {
		return err
	}
	id := rc.Param("id")
	// A TTY service emits a raw (un-multiplexed) log stream; a non-TTY service
	// multiplexes stdout/stderr with stdcopy framing. Demuxing a raw stream
	// would corrupt it, so branch on the service's TTY setting.
	insp, err := cli.ServiceInspect(rc.Ctx, id, dockerclient.ServiceInspectOptions{})
	if err != nil {
		return dockerengine.DockerErr(err)
	}
	tty := insp.Service.Spec.TaskTemplate.ContainerSpec != nil && insp.Service.Spec.TaskTemplate.ContainerSpec.TTY
	logs, err := cli.ServiceLogs(rc.Ctx, id, dockerclient.ServiceLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     boolParam(rc, "follow", true),
		Timestamps: boolParam(rc, "timestamps", true),
		Tail:       stringParam(rc, "tail", "200"),
	})
	if err != nil {
		return dockerengine.DockerErr(err)
	}
	defer func() { _ = logs.Close() }()
	done := make(chan error, 1)
	go func() {
		var copyErr error
		if tty {
			_, copyErr = io.Copy(stream, logs)
		} else {
			_, copyErr = stdcopy.StdCopy(stream, stream, logs)
		}
		done <- copyErr
	}()
	select {
	case <-stream.Context().Done():
		return nil
	case err := <-done:
		if err == io.EOF {
			return nil
		}
		return err
	}
}

func watchServiceEvents(rc *plugin.RequestContext, stream plugin.ClientStream) error {
	cli, err := client(rc)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(rc.Ctx)
	defer cancel()
	result := cli.Events(ctx, dockerclient.EventsListOptions{
		Filters: make(dockerclient.Filters).Add("type", string(events.ServiceEventType)),
	})
	enc := json.NewEncoder(stream)
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case err, ok := <-result.Err:
			if !ok || err == nil || err == io.EOF || err == context.Canceled {
				return nil
			}
			return dockerengine.DockerErr(err)
		case msg, ok := <-result.Messages:
			if !ok {
				return nil
			}
			if ev := serviceEvent(msg); ev != nil {
				if err := enc.Encode(ev); err != nil {
					return err
				}
			}
		}
	}
}

func serviceEvent(msg events.Message) *plugin.ResourceEvent {
	id := msg.Actor.ID
	if id == "" {
		return nil
	}
	var evType string
	switch msg.Action {
	case events.ActionCreate:
		evType = "added"
	case events.ActionRemove:
		evType = "deleted"
	case events.ActionUpdate:
		evType = "updated"
	default:
		return nil
	}
	name := msg.Actor.Attributes["name"]
	if name == "" {
		name = dockerengine.ShortID(id)
	}
	ref := plugin.ResourceIdentity{Kind: "service", Name: name, UID: id}
	return &plugin.ResourceEvent{Type: evType, Ref: ref, Resource: dockerengine.Row{"id": id, "name": name, "ref": ref}}
}

func serviceRows(items []swarm.Service) []dockerengine.Row {
	rows := make([]dockerengine.Row, 0, len(items))
	for _, s := range items {
		mode, replicas := serviceMode(s)
		rows = append(rows, dockerengine.Row{
			"id":        s.ID,
			"name":      s.Spec.Name,
			"mode":      mode,
			"replicas":  replicas,
			"image":     cleanImage(specImage(s.Spec.TaskTemplate)),
			"ports":     servicePorts(s.Endpoint.Ports),
			"stack":     s.Spec.Labels[stackNamespaceLabel],
			"createdAt": s.CreatedAt.UTC().Format(time.RFC3339),
			"ref":       plugin.ResourceIdentity{Kind: "service", Name: s.Spec.Name, UID: s.ID},
		})
	}
	return rows
}

func nodeRows(items []swarm.Node) []dockerengine.Row {
	rows := make([]dockerengine.Row, 0, len(items))
	for _, n := range items {
		leader := false
		if n.ManagerStatus != nil {
			leader = n.ManagerStatus.Leader
		}
		name := nodeName(n)
		rows = append(rows, dockerengine.Row{
			"id":           n.ID,
			"name":         name,
			"role":         string(n.Spec.Role),
			"availability": string(n.Spec.Availability),
			"state":        string(n.Status.State),
			"leader":       leader,
			"engine":       n.Description.Engine.EngineVersion,
			"address":      n.Status.Addr,
			"ref":          plugin.ResourceIdentity{Kind: "node", Name: name, UID: n.ID},
		})
	}
	return rows
}

func taskRows(items []swarm.Task, svcNames map[string]string) []dockerengine.Row {
	rows := make([]dockerengine.Row, 0, len(items))
	for _, t := range items {
		name := taskName(t, svcNames)
		rows = append(rows, dockerengine.Row{
			"id":           t.ID,
			"name":         name,
			"service":      svcLabel(t.ServiceID, svcNames),
			"node":         dockerengine.ShortID(t.NodeID),
			"slot":         t.Slot,
			"desiredState": string(t.DesiredState),
			"state":        string(t.Status.State),
			"image":        cleanImage(specImage(t.Spec)),
			"error":        t.Status.Err,
			"createdAt":    t.CreatedAt.UTC().Format(time.RFC3339),
			"ref":          plugin.ResourceIdentity{Kind: "task", Name: name, UID: t.ID},
		})
	}
	return rows
}

func stackRows(rc *plugin.RequestContext) ([]dockerengine.Row, error) {
	cli, err := client(rc)
	if err != nil {
		return nil, err
	}
	res, err := cli.ServiceList(rc.Ctx, dockerclient.ServiceListOptions{})
	if err != nil {
		return nil, dockerengine.DockerErr(err)
	}
	stacks := map[string]dockerengine.Row{}
	for _, s := range res.Items {
		ns := s.Spec.Labels[stackNamespaceLabel]
		if ns == "" {
			continue
		}
		r, ok := stacks[ns]
		if !ok {
			r = dockerengine.Row{"name": ns, "services": 0, "ref": plugin.ResourceIdentity{Kind: "stack", Name: ns, UID: ns}}
			stacks[ns] = r
		}
		r["services"] = r["services"].(int) + 1
	}
	rows := make([]dockerengine.Row, 0, len(stacks))
	for _, r := range stacks {
		rows = append(rows, r)
	}
	return rows, nil
}

// serviceNames maps service ID to its name for task display. Failures degrade to
// an empty map (tasks then show short service IDs).
func serviceNames(rc *plugin.RequestContext) map[string]string {
	cli, err := client(rc)
	if err != nil {
		return nil
	}
	res, err := cli.ServiceList(rc.Ctx, dockerclient.ServiceListOptions{})
	if err != nil {
		return nil
	}
	names := make(map[string]string, len(res.Items))
	for _, s := range res.Items {
		names[s.ID] = s.Spec.Name
	}
	return names
}

func buildTree(rc *plugin.RequestContext, kind string, ic plugin.Icon, list func(*plugin.RequestContext) (any, error)) (any, error) {
	res, err := list(rc)
	if err != nil {
		return nil, err
	}
	page, ok := res.(plugin.Page[dockerengine.Row])
	if !ok {
		return nil, fmt.Errorf("%w: unexpected tree data", plugin.ErrUnavailable)
	}
	nodes := make([]plugin.TreeNode, 0, len(page.Items))
	for _, r := range page.Items {
		ref, ok := r["ref"].(plugin.ResourceIdentity)
		if !ok {
			continue
		}
		refCopy := ref
		nodes = append(nodes, plugin.TreeNode{Key: kind + ":" + ref.UID, Label: ref.Name, Icon: ic, Ref: &refCopy, Leaf: true})
	}
	return plugin.Page[plugin.TreeNode]{Items: nodes, NextCursor: page.NextCursor, Total: page.Total}, nil
}

func serviceMode(s swarm.Service) (mode, replicas string) {
	switch {
	case s.Spec.Mode.Global != nil:
		mode = "global"
	case s.Spec.Mode.ReplicatedJob != nil:
		mode = "replicated-job"
	case s.Spec.Mode.GlobalJob != nil:
		mode = "global-job"
	case s.Spec.Mode.Replicated != nil:
		mode = "replicated"
	}
	if s.ServiceStatus != nil {
		return mode, fmt.Sprintf("%d/%d", s.ServiceStatus.RunningTasks, s.ServiceStatus.DesiredTasks)
	}
	if s.Spec.Mode.Replicated != nil && s.Spec.Mode.Replicated.Replicas != nil {
		return mode, fmt.Sprintf("0/%d", *s.Spec.Mode.Replicated.Replicas)
	}
	return mode, ""
}

// serviceProxyURL returns the gateway "open in browser" URL for a service,
// proxying to its routing-mesh published port (reachable on the daemon host).
func serviceProxyURL(rc *plugin.RequestContext) (any, error) {
	s, err := dockerengine.Unwrap(rc.Session)
	if err != nil {
		return nil, err
	}
	id := rc.Param("id")
	portSeg := rc.Param("port")
	if portSeg == "" {
		res, err := s.Client().ServiceInspect(rc.Ctx, id, dockerclient.ServiceInspectOptions{})
		if err != nil {
			return nil, dockerengine.DockerErr(err)
		}
		portSeg, err = pickServicePort(res.Service.Endpoint.Ports)
		if err != nil {
			return nil, err
		}
	}
	return map[string]any{"url": rc.ProxyURL("service", id, portSeg)}, nil
}

func serviceOpenSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{
		Name: "Open",
		Fields: []plugin.Field{{
			Key:         "port",
			Label:       "Port",
			Type:        plugin.FieldSelect,
			Placeholder: "Select a port",
			OptionsSource: &plugin.DataSource{
				RouteID: "swarm.service.open.ports",
				Params:  map[string]string{"id": "${resource.uid}"},
			},
		}},
	}}}
}

func serviceOpenPorts(rc *plugin.RequestContext) (any, error) {
	s, err := dockerengine.Unwrap(rc.Session)
	if err != nil {
		return nil, err
	}
	res, err := s.Client().ServiceInspect(rc.Ctx, rc.Param("id"), dockerclient.ServiceInspectOptions{})
	if err != nil {
		return nil, dockerengine.DockerErr(err)
	}
	return plugin.Page[plugin.Option]{Items: servicePortOptions(res.Service.Endpoint.Ports)}, nil
}

// pickServicePort picks a service's published TCP port to open, preferring ingress
// (routing-mesh) ports — host-mode ones live only on the task's node, not the
// manager we dial — and a port that names a web protocol, else the lowest. The
// segment is the published port; an https-named/443/8443 port proxies over TLS.
func pickServicePort(ports []swarm.PortConfig) (string, error) {
	cands := reachableServicePorts(ports)
	if len(cands) == 0 {
		return "", fmt.Errorf("%w: service publishes no reachable TCP ports", plugin.ErrInvalidInput)
	}
	pick := cands[0]
	for _, p := range cands {
		if _, named := webproxy.WebSchemeFromName(p.Name); named {
			pick = p
			break
		}
	}
	return servicePortSegment(pick), nil
}

func reachableServicePorts(ports []swarm.PortConfig) []swarm.PortConfig {
	var ingress, hostMode []swarm.PortConfig
	for _, p := range ports {
		if p.PublishedPort == 0 || strings.ToLower(string(p.Protocol)) != "tcp" {
			continue
		}
		if p.PublishMode == swarm.PortConfigPublishModeHost {
			hostMode = append(hostMode, p)
		} else {
			ingress = append(ingress, p)
		}
	}
	cands := ingress
	if len(cands) == 0 {
		cands = hostMode
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].PublishedPort < cands[j].PublishedPort })
	return cands
}

func servicePortSegment(p swarm.PortConfig) string {
	seg := strconv.Itoa(int(p.PublishedPort))
	if scheme, _ := webproxy.WebSchemeFromName(p.Name); scheme == "https" ||
		webproxy.IsTLSPort(int(p.TargetPort)) ||
		webproxy.IsTLSPort(int(p.PublishedPort)) {
		return "https:" + seg
	}
	return seg
}

func servicePortOptions(ports []swarm.PortConfig) []plugin.Option {
	cands := reachableServicePorts(ports)
	items := make([]plugin.Option, 0, len(cands))
	seen := map[string]bool{}
	for _, p := range cands {
		value := servicePortSegment(p)
		if seen[value] {
			continue
		}
		seen[value] = true
		scheme := "HTTP"
		if strings.HasPrefix(value, "https:") {
			scheme = "HTTPS"
		}
		label := fmt.Sprintf("%s %d->%d/%s", scheme, p.PublishedPort, p.TargetPort, p.Protocol)
		if p.Name != "" {
			label = p.Name + " - " + label
		}
		items = append(items, plugin.Option{Label: label, Value: value})
	}
	return items
}

func servicePorts(ports []swarm.PortConfig) string {
	out := make([]string, 0, len(ports))
	for _, p := range ports {
		if p.PublishedPort == 0 {
			out = append(out, fmt.Sprintf("%d/%s", p.TargetPort, p.Protocol))
			continue
		}
		out = append(out, fmt.Sprintf("%d->%d/%s", p.PublishedPort, p.TargetPort, p.Protocol))
	}
	return strings.Join(out, ", ")
}

func specImage(spec swarm.TaskSpec) string {
	if spec.ContainerSpec == nil {
		return ""
	}
	return spec.ContainerSpec.Image
}

func cleanImage(image string) string {
	if i := strings.Index(image, "@"); i >= 0 {
		return image[:i]
	}
	return image
}

func nodeName(n swarm.Node) string {
	if n.Description.Hostname != "" {
		return n.Description.Hostname
	}
	if n.Spec.Name != "" {
		return n.Spec.Name
	}
	return dockerengine.ShortID(n.ID)
}

func taskName(t swarm.Task, svcNames map[string]string) string {
	base := svcLabel(t.ServiceID, svcNames)
	if t.Slot > 0 {
		return fmt.Sprintf("%s.%d", base, t.Slot)
	}
	return fmt.Sprintf("%s.%s", base, dockerengine.ShortID(t.NodeID))
}

func svcLabel(serviceID string, svcNames map[string]string) string {
	if name := svcNames[serviceID]; name != "" {
		return name
	}
	return dockerengine.ShortID(serviceID)
}

func boolParam(rc *plugin.RequestContext, key string, fallback bool) bool {
	raw := paramOrQuery(rc, key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return v
}

func stringParam(rc *plugin.RequestContext, key, fallback string) string {
	if raw := paramOrQuery(rc, key); raw != "" {
		return raw
	}
	return fallback
}

func paramOrQuery(rc *plugin.RequestContext, key string) string {
	if v := rc.Param(key); v != "" {
		return v
	}
	return rc.Query().Get(key)
}
