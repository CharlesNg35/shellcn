package docker

import (
	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/plugins/shared/dockerengine"
)

// Routes wires the shared Docker-engine handlers to docker-namespaced route IDs,
// permissions, and audit events. All behaviour lives in dockerengine.
func Routes() []plugin.Route {
	return []plugin.Route{
		{ID: "docker.containers.tree", Method: plugin.MethodGet, Path: "/tree/containers", Permission: "docker.containers.read", Risk: plugin.RiskSafe, AuditEvent: "docker.containers.tree", Handle: dockerengine.TreeContainers},
		{ID: "docker.images.tree", Method: plugin.MethodGet, Path: "/tree/images", Permission: "docker.images.read", Risk: plugin.RiskSafe, AuditEvent: "docker.images.tree", Handle: dockerengine.TreeImages},
		{ID: "docker.volumes.tree", Method: plugin.MethodGet, Path: "/tree/volumes", Permission: "docker.volumes.read", Risk: plugin.RiskSafe, AuditEvent: "docker.volumes.tree", Handle: dockerengine.TreeVolumes},
		{ID: "docker.networks.tree", Method: plugin.MethodGet, Path: "/tree/networks", Permission: "docker.networks.read", Risk: plugin.RiskSafe, AuditEvent: "docker.networks.tree", Handle: dockerengine.TreeNetworks},
		{ID: "docker.compose.tree", Method: plugin.MethodGet, Path: "/tree/compose", Permission: "docker.compose.read", Risk: plugin.RiskSafe, AuditEvent: "docker.compose.tree", Handle: dockerengine.TreeCompose},
		{ID: "docker.containers.list", Method: plugin.MethodGet, Path: "/containers", Permission: "docker.containers.read", Risk: plugin.RiskSafe, AuditEvent: "docker.containers.list", Handle: dockerengine.ListContainers},
		{ID: "docker.images.list", Method: plugin.MethodGet, Path: "/images", Permission: "docker.images.read", Risk: plugin.RiskSafe, AuditEvent: "docker.images.list", Handle: dockerengine.ListImages},
		{ID: "docker.volumes.list", Method: plugin.MethodGet, Path: "/volumes", Permission: "docker.volumes.read", Risk: plugin.RiskSafe, AuditEvent: "docker.volumes.list", Handle: dockerengine.ListVolumes},
		{ID: "docker.networks.list", Method: plugin.MethodGet, Path: "/networks", Permission: "docker.networks.read", Risk: plugin.RiskSafe, AuditEvent: "docker.networks.list", Handle: dockerengine.ListNetworks},
		{ID: "docker.compose.list", Method: plugin.MethodGet, Path: "/compose", Permission: "docker.compose.read", Risk: plugin.RiskSafe, AuditEvent: "docker.compose.list", Handle: dockerengine.ListCompose},
		{ID: "docker.container.overview", Method: plugin.MethodGet, Path: "/containers/{id}/overview", Permission: "docker.containers.read", Risk: plugin.RiskSafe, AuditEvent: "docker.container.overview", Handle: dockerengine.ContainerOverview},
		{ID: "docker.image.overview", Method: plugin.MethodGet, Path: "/images/{id}/overview", Permission: "docker.images.read", Risk: plugin.RiskSafe, AuditEvent: "docker.image.overview", Handle: dockerengine.ImageOverview},
		{ID: "docker.volume.overview", Method: plugin.MethodGet, Path: "/volumes/{id}/overview", Permission: "docker.volumes.read", Risk: plugin.RiskSafe, AuditEvent: "docker.volume.overview", Handle: dockerengine.VolumeOverview},
		{ID: "docker.network.overview", Method: plugin.MethodGet, Path: "/networks/{id}/overview", Permission: "docker.networks.read", Risk: plugin.RiskSafe, AuditEvent: "docker.network.overview", Handle: dockerengine.NetworkOverview},
		{ID: "docker.compose.overview", Method: plugin.MethodGet, Path: "/compose/{project}/overview", Permission: "docker.compose.read", Risk: plugin.RiskSafe, AuditEvent: "docker.compose.overview", Handle: dockerengine.ComposeOverview},
		{ID: "docker.compose.containers", Method: plugin.MethodGet, Path: "/compose/{project}/containers", Permission: "docker.compose.read", Risk: plugin.RiskSafe, AuditEvent: "docker.compose.containers", Handle: dockerengine.ComposeContainers},
		{ID: "docker.compose.services", Method: plugin.MethodGet, Path: "/compose/{project}/services", Permission: "docker.compose.read", Risk: plugin.RiskSafe, AuditEvent: "docker.compose.services", Handle: dockerengine.ComposeServices},
		{ID: "docker.container.inspect", Method: plugin.MethodGet, Path: "/containers/{id}/inspect", Permission: "docker.containers.read", Risk: plugin.RiskSafe, AuditEvent: "docker.container.inspect", Handle: dockerengine.InspectContainer},
		{ID: "docker.image.inspect", Method: plugin.MethodGet, Path: "/images/{id}/inspect", Permission: "docker.images.read", Risk: plugin.RiskSafe, AuditEvent: "docker.image.inspect", Handle: dockerengine.InspectImage},
		{ID: "docker.volume.inspect", Method: plugin.MethodGet, Path: "/volumes/{id}/inspect", Permission: "docker.volumes.read", Risk: plugin.RiskSafe, AuditEvent: "docker.volume.inspect", Handle: dockerengine.InspectVolume},
		{ID: "docker.network.inspect", Method: plugin.MethodGet, Path: "/networks/{id}/inspect", Permission: "docker.networks.read", Risk: plugin.RiskSafe, AuditEvent: "docker.network.inspect", Handle: dockerengine.InspectNetwork},
		{ID: "docker.container.env", Method: plugin.MethodGet, Path: "/containers/{id}/env", Permission: "docker.containers.read", Risk: plugin.RiskSafe, AuditEvent: "docker.container.env", Handle: dockerengine.ContainerEnv},
		{ID: "docker.container.open", Method: plugin.MethodGet, Path: "/containers/{id}/open", Permission: "docker.containers.read", Risk: plugin.RiskSafe, AuditEvent: "docker.container.open", Handle: dockerengine.ContainerProxyURL},
		{ID: "docker.container.create", Method: plugin.MethodPost, Path: "/containers", Permission: "docker.containers.write", Risk: plugin.RiskWrite, AuditEvent: "docker.container.create", Input: dockerengine.CreateContainerSchema(), Handle: dockerengine.CreateContainer},
		{ID: "docker.container.start", Method: plugin.MethodPost, Path: "/containers/{id}/start", Permission: "docker.containers.write", Risk: plugin.RiskWrite, AuditEvent: "docker.container.start", Handle: dockerengine.StartContainer},
		{ID: "docker.container.stop", Method: plugin.MethodPost, Path: "/containers/{id}/stop", Permission: "docker.containers.write", Risk: plugin.RiskWrite, AuditEvent: "docker.container.stop", Handle: dockerengine.StopContainer},
		{ID: "docker.container.restart", Method: plugin.MethodPost, Path: "/containers/{id}/restart", Permission: "docker.containers.write", Risk: plugin.RiskWrite, AuditEvent: "docker.container.restart", Handle: dockerengine.RestartContainer},
		{ID: "docker.container.remove", Method: plugin.MethodDelete, Path: "/containers/{id}", Permission: "docker.containers.delete", Risk: plugin.RiskDestructive, AuditEvent: "docker.container.remove", Handle: dockerengine.RemoveContainer},
		{ID: "docker.image.remove", Method: plugin.MethodDelete, Path: "/images/{id}", Permission: "docker.images.delete", Risk: plugin.RiskDestructive, AuditEvent: "docker.image.remove", Handle: dockerengine.RemoveImage},
		{ID: "docker.volume.remove", Method: plugin.MethodDelete, Path: "/volumes/{id}", Permission: "docker.volumes.delete", Risk: plugin.RiskDestructive, AuditEvent: "docker.volume.remove", Handle: dockerengine.RemoveVolume},
		{ID: "docker.network.remove", Method: plugin.MethodDelete, Path: "/networks/{id}", Permission: "docker.networks.delete", Risk: plugin.RiskDestructive, AuditEvent: "docker.network.remove", Handle: dockerengine.RemoveNetwork},
		{ID: "docker.image.pull", Method: plugin.MethodPost, Path: "/images/pull", Permission: "docker.images.write", Risk: plugin.RiskWrite, AuditEvent: "docker.image.pull", Input: dockerengine.ImagePullSchema(), Handle: dockerengine.PullImage},
		{ID: "docker.volume.create", Method: plugin.MethodPost, Path: "/volumes/create", Permission: "docker.volumes.write", Risk: plugin.RiskWrite, AuditEvent: "docker.volume.create", Input: dockerengine.VolumeCreateSchema(), Handle: dockerengine.CreateVolume},
		{ID: "docker.network.create", Method: plugin.MethodPost, Path: "/networks/create", Permission: "docker.networks.write", Risk: plugin.RiskWrite, AuditEvent: "docker.network.create", Input: dockerengine.NetworkCreateSchema(), Handle: dockerengine.CreateNetwork},
		{ID: "docker.containers.prune", Method: plugin.MethodPost, Path: "/containers/prune", Permission: "docker.containers.delete", Risk: plugin.RiskDestructive, AuditEvent: "docker.containers.prune", Handle: dockerengine.PruneContainers},
		{ID: "docker.images.prune", Method: plugin.MethodPost, Path: "/images/prune", Permission: "docker.images.delete", Risk: plugin.RiskDestructive, AuditEvent: "docker.images.prune", Handle: dockerengine.PruneImages},
		{ID: "docker.volumes.prune", Method: plugin.MethodPost, Path: "/volumes/prune", Permission: "docker.volumes.delete", Risk: plugin.RiskDestructive, AuditEvent: "docker.volumes.prune", Handle: dockerengine.PruneVolumes},
		{ID: "docker.networks.prune", Method: plugin.MethodPost, Path: "/networks/prune", Permission: "docker.networks.delete", Risk: plugin.RiskDestructive, AuditEvent: "docker.networks.prune", Handle: dockerengine.PruneNetworks},
		{ID: "docker.container.logs", Method: plugin.MethodWS, Path: "/containers/{id}/logs/{tail}/{follow}/{timestamps}", Permission: "docker.containers.logs", Risk: plugin.RiskSafe, AuditEvent: "docker.container.logs", Input: dockerengine.LogsSchema(), Stream: dockerengine.LogsStream},
		{ID: "docker.container.exec", Method: plugin.MethodWS, Path: "/containers/{id}/exec/ws/{cols}/{rows}/{command}", Permission: "docker.containers.exec", Risk: plugin.RiskPrivileged, AuditEvent: "docker.container.exec", Input: dockerengine.ExecSchema(), Stream: dockerengine.ExecStream},
		{ID: "docker.events.watch", Method: plugin.MethodWS, Path: "/events", Permission: "docker.events.read", Risk: plugin.RiskSafe, AuditEvent: "docker.events.watch", Stream: dockerengine.WatchEvents},
	}
}
