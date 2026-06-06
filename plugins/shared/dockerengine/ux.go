package dockerengine

import "github.com/charlesng35/shellcn/sdk/plugin"

func ContainerOverviewDetailConfig() plugin.ObjectDetailConfig {
	return plugin.ObjectDetailConfig{
		RawToggle: true,
		Sections: []plugin.ObjectDetailSection{
			{Title: "Summary", Fields: []plugin.ObjectDetailField{
				{Key: "id", Label: "ID", Copy: true},
				{Key: "name", Label: "Name", Copy: true},
				{Key: "image", Label: "Image", Copy: true},
				{Key: "state", Label: "State", Type: plugin.ColumnBadge, Severities: StateSeverities()},
				{Key: "health", Label: "Health", Type: plugin.ColumnBadge, Severities: healthSeverities()},
				{Key: "running", Label: "Running", Type: plugin.ColumnBool},
				{Key: "restartCount", Label: "Restarts", Type: plugin.ColumnNumber},
				{Key: "exitCode", Label: "Exit code", Type: plugin.ColumnNumber},
			}},
			{Title: "Runtime", Fields: []plugin.ObjectDetailField{
				{Key: "command", Label: "Command", Copy: true},
				{Key: "driver", Label: "Driver"},
				{Key: "platform", Label: "Platform"},
				{Key: "created", Label: "Created", Type: plugin.ColumnDateTime},
				{Key: "startedAt", Label: "Started", Type: plugin.ColumnDateTime},
				{Key: "finishedAt", Label: "Finished", Type: plugin.ColumnDateTime},
			}},
			{Title: "Placement", Fields: []plugin.ObjectDetailField{
				{Key: "composeProject", Label: "Compose project"},
				{Key: "composeService", Label: "Compose service"},
				{Key: "networks", Label: "Networks", Type: plugin.ColumnJSON},
				{Key: "mounts", Label: "Mounts", Type: plugin.ColumnNumber},
			}},
		},
	}
}

func ImageOverviewDetailConfig() plugin.ObjectDetailConfig {
	return plugin.ObjectDetailConfig{
		RawToggle: true,
		Sections: []plugin.ObjectDetailSection{
			{Title: "Image", Fields: []plugin.ObjectDetailField{
				{Key: "Id", Label: "ID", Copy: true},
				{Key: "RepoTags", Label: "Tags", Type: plugin.ColumnJSON},
				{Key: "RepoDigests", Label: "Digests", Type: plugin.ColumnJSON},
				{Key: "Created", Label: "Created", Type: plugin.ColumnDateTime},
				{Key: "Size", Label: "Size", Type: plugin.ColumnBytes},
				{Key: "VirtualSize", Label: "Virtual size", Type: plugin.ColumnBytes},
			}},
			{Title: "Platform", Fields: []plugin.ObjectDetailField{
				{Key: "Architecture", Label: "Architecture"},
				{Key: "Os", Label: "OS"},
				{Key: "DockerVersion", Label: "Docker version"},
				{Key: "Author", Label: "Author"},
				{Key: "Comment", Label: "Comment"},
			}},
		},
	}
}

func VolumeOverviewDetailConfig() plugin.ObjectDetailConfig {
	return plugin.ObjectDetailConfig{
		RawToggle: true,
		Sections: []plugin.ObjectDetailSection{
			{Title: "Volume", Fields: []plugin.ObjectDetailField{
				{Key: "name", Label: "Name", Copy: true},
				{Key: "driver", Label: "Driver"},
				{Key: "scope", Label: "Scope"},
				{Key: "mountpoint", Label: "Mountpoint", Copy: true},
				{Key: "createdAt", Label: "Created", Type: plugin.ColumnDateTime},
				{Key: "size", Label: "Size", Type: plugin.ColumnBytes},
				{Key: "refs", Label: "Refs", Type: plugin.ColumnNumber},
			}},
			{Title: "Metadata", Fields: []plugin.ObjectDetailField{
				{Key: "labels", Label: "Labels", Type: plugin.ColumnJSON},
				{Key: "options", Label: "Options", Type: plugin.ColumnJSON},
			}},
		},
	}
}

func NetworkOverviewDetailConfig() plugin.ObjectDetailConfig {
	return plugin.ObjectDetailConfig{
		RawToggle: true,
		Sections: []plugin.ObjectDetailSection{
			{Title: "Network", Fields: []plugin.ObjectDetailField{
				{Key: "id", Label: "ID", Copy: true},
				{Key: "name", Label: "Name", Copy: true},
				{Key: "driver", Label: "Driver"},
				{Key: "scope", Label: "Scope"},
				{Key: "created", Label: "Created", Type: plugin.ColumnDateTime},
			}},
			{Title: "Behavior", Fields: []plugin.ObjectDetailField{
				{Key: "internal", Label: "Internal", Type: plugin.ColumnBool},
				{Key: "attachable", Label: "Attachable", Type: plugin.ColumnBool},
				{Key: "ingress", Label: "Ingress", Type: plugin.ColumnBool},
				{Key: "containers", Label: "Containers", Type: plugin.ColumnNumber},
				{Key: "services", Label: "Services", Type: plugin.ColumnNumber},
				{Key: "labels", Label: "Labels", Type: plugin.ColumnJSON},
			}},
		},
	}
}

func ComposeOverviewDetailConfig() plugin.ObjectDetailConfig {
	return plugin.ObjectDetailConfig{
		RawToggle: true,
		Sections: []plugin.ObjectDetailSection{
			{Title: "Project", Fields: []plugin.ObjectDetailField{
				{Key: "name", Label: "Name", Copy: true},
				{Key: "containers", Label: "Containers", Type: plugin.ColumnNumber},
				{Key: "running", Label: "Running", Type: plugin.ColumnNumber},
				{Key: "workingDir", Label: "Working directory", Copy: true},
				{Key: "config", Label: "Config", Copy: true},
			}},
		},
	}
}

func healthSeverities() map[string]plugin.Severity {
	return map[string]plugin.Severity{
		"healthy":   plugin.SeveritySuccess,
		"starting":  plugin.SeverityWarn,
		"unhealthy": plugin.SeverityDanger,
	}
}
