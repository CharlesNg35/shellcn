package podman

import (
	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/plugins/shared/dockerengine"
)

// Routes wires podman-namespaced route IDs. Containers, images, volumes,
// networks, logs, exec, and events reuse the shared dockerengine handlers over
// Podman's Docker-compatible socket; pods are Podman-native.
func Routes() []plugin.Route {
	return []plugin.Route{
		{ID: "podman.containers.tree", Method: plugin.MethodGet, Path: "/tree/containers", Permission: "podman.containers.read", Risk: plugin.RiskSafe, AuditEvent: "podman.containers.tree", Handle: dockerengine.TreeContainers},
		{ID: "podman.pods.tree", Method: plugin.MethodGet, Path: "/tree/pods", Permission: "podman.pods.read", Risk: plugin.RiskSafe, AuditEvent: "podman.pods.tree", Handle: treePods},
		{ID: "podman.images.tree", Method: plugin.MethodGet, Path: "/tree/images", Permission: "podman.images.read", Risk: plugin.RiskSafe, AuditEvent: "podman.images.tree", Handle: dockerengine.TreeImages},
		{ID: "podman.volumes.tree", Method: plugin.MethodGet, Path: "/tree/volumes", Permission: "podman.volumes.read", Risk: plugin.RiskSafe, AuditEvent: "podman.volumes.tree", Handle: dockerengine.TreeVolumes},
		{ID: "podman.networks.tree", Method: plugin.MethodGet, Path: "/tree/networks", Permission: "podman.networks.read", Risk: plugin.RiskSafe, AuditEvent: "podman.networks.tree", Handle: dockerengine.TreeNetworks},
		{ID: "podman.containers.list", Method: plugin.MethodGet, Path: "/containers", Permission: "podman.containers.read", Risk: plugin.RiskSafe, AuditEvent: "podman.containers.list", Handle: dockerengine.ListContainers},
		{ID: "podman.pods.list", Method: plugin.MethodGet, Path: "/pods", Permission: "podman.pods.read", Risk: plugin.RiskSafe, AuditEvent: "podman.pods.list", Handle: listPods},
		{ID: "podman.images.list", Method: plugin.MethodGet, Path: "/images", Permission: "podman.images.read", Risk: plugin.RiskSafe, AuditEvent: "podman.images.list", Handle: dockerengine.ListImages},
		{ID: "podman.volumes.list", Method: plugin.MethodGet, Path: "/volumes", Permission: "podman.volumes.read", Risk: plugin.RiskSafe, AuditEvent: "podman.volumes.list", Handle: dockerengine.ListVolumes},
		{ID: "podman.networks.list", Method: plugin.MethodGet, Path: "/networks", Permission: "podman.networks.read", Risk: plugin.RiskSafe, AuditEvent: "podman.networks.list", Handle: dockerengine.ListNetworks},
		{ID: "podman.container.overview", Method: plugin.MethodGet, Path: "/containers/{id}/overview", Permission: "podman.containers.read", Risk: plugin.RiskSafe, AuditEvent: "podman.container.overview", Handle: dockerengine.ContainerOverview},
		{ID: "podman.image.overview", Method: plugin.MethodGet, Path: "/images/{id}/overview", Permission: "podman.images.read", Risk: plugin.RiskSafe, AuditEvent: "podman.image.overview", Handle: dockerengine.ImageOverview},
		{ID: "podman.volume.overview", Method: plugin.MethodGet, Path: "/volumes/{id}/overview", Permission: "podman.volumes.read", Risk: plugin.RiskSafe, AuditEvent: "podman.volume.overview", Handle: dockerengine.VolumeOverview},
		{ID: "podman.network.overview", Method: plugin.MethodGet, Path: "/networks/{id}/overview", Permission: "podman.networks.read", Risk: plugin.RiskSafe, AuditEvent: "podman.network.overview", Handle: dockerengine.NetworkOverview},
		{ID: "podman.pod.overview", Method: plugin.MethodGet, Path: "/pods/{id}/overview", Permission: "podman.pods.read", Risk: plugin.RiskSafe, AuditEvent: "podman.pod.overview", Handle: podOverview},
		{ID: "podman.pod.containers", Method: plugin.MethodGet, Path: "/pods/{id}/containers", Permission: "podman.pods.read", Risk: plugin.RiskSafe, AuditEvent: "podman.pod.containers", Handle: podContainers},
		{ID: "podman.container.inspect", Method: plugin.MethodGet, Path: "/containers/{id}/inspect", Permission: "podman.containers.read", Risk: plugin.RiskSafe, AuditEvent: "podman.container.inspect", Handle: dockerengine.InspectContainer},
		{ID: "podman.image.inspect", Method: plugin.MethodGet, Path: "/images/{id}/inspect", Permission: "podman.images.read", Risk: plugin.RiskSafe, AuditEvent: "podman.image.inspect", Handle: dockerengine.InspectImage},
		{ID: "podman.volume.inspect", Method: plugin.MethodGet, Path: "/volumes/{id}/inspect", Permission: "podman.volumes.read", Risk: plugin.RiskSafe, AuditEvent: "podman.volume.inspect", Handle: dockerengine.InspectVolume},
		{ID: "podman.network.inspect", Method: plugin.MethodGet, Path: "/networks/{id}/inspect", Permission: "podman.networks.read", Risk: plugin.RiskSafe, AuditEvent: "podman.network.inspect", Handle: dockerengine.InspectNetwork},
		{ID: "podman.pod.inspect", Method: plugin.MethodGet, Path: "/pods/{id}/inspect", Permission: "podman.pods.read", Risk: plugin.RiskSafe, AuditEvent: "podman.pod.inspect", Handle: podInspect},
		{ID: "podman.pod.start", Method: plugin.MethodPost, Path: "/pods/{id}/start", Permission: "podman.pods.write", Risk: plugin.RiskWrite, AuditEvent: "podman.pod.start", Handle: startPod},
		{ID: "podman.pod.stop", Method: plugin.MethodPost, Path: "/pods/{id}/stop", Permission: "podman.pods.write", Risk: plugin.RiskWrite, AuditEvent: "podman.pod.stop", Handle: stopPod},
		{ID: "podman.pod.restart", Method: plugin.MethodPost, Path: "/pods/{id}/restart", Permission: "podman.pods.write", Risk: plugin.RiskWrite, AuditEvent: "podman.pod.restart", Handle: restartPod},
		{ID: "podman.pod.remove", Method: plugin.MethodDelete, Path: "/pods/{id}", Permission: "podman.pods.delete", Risk: plugin.RiskDestructive, AuditEvent: "podman.pod.remove", Handle: removePod},
		{ID: "podman.container.env", Method: plugin.MethodGet, Path: "/containers/{id}/env", Permission: "podman.containers.read", Risk: plugin.RiskSafe, AuditEvent: "podman.container.env", Handle: dockerengine.ContainerEnv},
		{ID: "podman.container.create", Method: plugin.MethodPost, Path: "/containers", Permission: "podman.containers.write", Risk: plugin.RiskWrite, AuditEvent: "podman.container.create", Input: dockerengine.CreateContainerSchema(), Handle: dockerengine.CreateContainer},
		{ID: "podman.container.start", Method: plugin.MethodPost, Path: "/containers/{id}/start", Permission: "podman.containers.write", Risk: plugin.RiskWrite, AuditEvent: "podman.container.start", Handle: dockerengine.StartContainer},
		{ID: "podman.container.stop", Method: plugin.MethodPost, Path: "/containers/{id}/stop", Permission: "podman.containers.write", Risk: plugin.RiskWrite, AuditEvent: "podman.container.stop", Handle: dockerengine.StopContainer},
		{ID: "podman.container.restart", Method: plugin.MethodPost, Path: "/containers/{id}/restart", Permission: "podman.containers.write", Risk: plugin.RiskWrite, AuditEvent: "podman.container.restart", Handle: dockerengine.RestartContainer},
		{ID: "podman.container.remove", Method: plugin.MethodDelete, Path: "/containers/{id}", Permission: "podman.containers.delete", Risk: plugin.RiskDestructive, AuditEvent: "podman.container.remove", Handle: dockerengine.RemoveContainer},
		{ID: "podman.image.remove", Method: plugin.MethodDelete, Path: "/images/{id}", Permission: "podman.images.delete", Risk: plugin.RiskDestructive, AuditEvent: "podman.image.remove", Handle: dockerengine.RemoveImage},
		{ID: "podman.volume.remove", Method: plugin.MethodDelete, Path: "/volumes/{id}", Permission: "podman.volumes.delete", Risk: plugin.RiskDestructive, AuditEvent: "podman.volume.remove", Handle: dockerengine.RemoveVolume},
		{ID: "podman.network.remove", Method: plugin.MethodDelete, Path: "/networks/{id}", Permission: "podman.networks.delete", Risk: plugin.RiskDestructive, AuditEvent: "podman.network.remove", Handle: dockerengine.RemoveNetwork},
		{ID: "podman.image.pull", Method: plugin.MethodPost, Path: "/images/pull", Permission: "podman.images.write", Risk: plugin.RiskWrite, AuditEvent: "podman.image.pull", Input: dockerengine.ImagePullSchema(), Handle: dockerengine.PullImage},
		{ID: "podman.volume.create", Method: plugin.MethodPost, Path: "/volumes/create", Permission: "podman.volumes.write", Risk: plugin.RiskWrite, AuditEvent: "podman.volume.create", Input: dockerengine.VolumeCreateSchema(), Handle: dockerengine.CreateVolume},
		{ID: "podman.network.create", Method: plugin.MethodPost, Path: "/networks/create", Permission: "podman.networks.write", Risk: plugin.RiskWrite, AuditEvent: "podman.network.create", Input: dockerengine.NetworkCreateSchema(), Handle: dockerengine.CreateNetwork},
		{ID: "podman.containers.prune", Method: plugin.MethodPost, Path: "/containers/prune", Permission: "podman.containers.delete", Risk: plugin.RiskDestructive, AuditEvent: "podman.containers.prune", Handle: dockerengine.PruneContainers},
		{ID: "podman.images.prune", Method: plugin.MethodPost, Path: "/images/prune", Permission: "podman.images.delete", Risk: plugin.RiskDestructive, AuditEvent: "podman.images.prune", Handle: dockerengine.PruneImages},
		{ID: "podman.volumes.prune", Method: plugin.MethodPost, Path: "/volumes/prune", Permission: "podman.volumes.delete", Risk: plugin.RiskDestructive, AuditEvent: "podman.volumes.prune", Handle: dockerengine.PruneVolumes},
		{ID: "podman.networks.prune", Method: plugin.MethodPost, Path: "/networks/prune", Permission: "podman.networks.delete", Risk: plugin.RiskDestructive, AuditEvent: "podman.networks.prune", Handle: dockerengine.PruneNetworks},
		{ID: "podman.container.logs", Method: plugin.MethodWS, Path: "/containers/{id}/logs/{tail}/{follow}/{timestamps}", Permission: "podman.containers.logs", Risk: plugin.RiskSafe, AuditEvent: "podman.container.logs", Input: dockerengine.LogsSchema(), Stream: dockerengine.LogsStream},
		{ID: "podman.container.exec", Method: plugin.MethodWS, Path: "/containers/{id}/exec/ws/{cols}/{rows}/{command}", Permission: "podman.containers.exec", Risk: plugin.RiskPrivileged, AuditEvent: "podman.container.exec", Input: dockerengine.ExecSchema(), Stream: dockerengine.ExecStream},
		{ID: "podman.events.watch", Method: plugin.MethodWS, Path: "/events", Permission: "podman.events.read", Risk: plugin.RiskSafe, AuditEvent: "podman.events.watch", Stream: dockerengine.WatchEvents},
	}
}
