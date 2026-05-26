package plugin

type Category string

const (
	CategoryShell          Category = "shell"
	CategoryFiles          Category = "files"
	CategoryContainers     Category = "containers"
	CategoryVirtualization Category = "virtualization"
	CategoryRemoteDesktop  Category = "remote_desktop"
	CategoryDatabases      Category = "databases"
	CategoryOrchestration  Category = "orchestration"
	CategoryCloud          Category = "cloud"
	CategoryNetwork        Category = "network"
	CategorySecurity       Category = "security"
	CategoryDevOps         Category = "devops"
	CategoryObservability  Category = "observability"
	CategoryMessaging      Category = "messaging"
	CategoryOther          Category = "other"
)

type CategoryInfo struct {
	Key   Category `json:"key"`
	Label string   `json:"label"`
	Icon  Icon     `json:"icon"`
	Order int      `json:"order"`
}

var builtInPluginCategories = []CategoryInfo{
	{Key: CategoryShell, Label: "Shell & terminal", Icon: Icon{Type: IconLucide, Value: "terminal"}, Order: 10},
	{Key: CategoryFiles, Label: "Files & storage", Icon: Icon{Type: IconLucide, Value: "folder-open"}, Order: 20},
	{Key: CategoryContainers, Label: "Containers", Icon: Icon{Type: IconLucide, Value: "boxes"}, Order: 30},
	{Key: CategoryVirtualization, Label: "Virtualization", Icon: Icon{Type: IconLucide, Value: "server"}, Order: 40},
	{Key: CategoryRemoteDesktop, Label: "Remote desktop", Icon: Icon{Type: IconLucide, Value: "monitor"}, Order: 50},
	{Key: CategoryDatabases, Label: "Databases", Icon: Icon{Type: IconLucide, Value: "database"}, Order: 60},
	{Key: CategoryOrchestration, Label: "Orchestration", Icon: Icon{Type: IconLucide, Value: "ship-wheel"}, Order: 70},
	{Key: CategoryCloud, Label: "Cloud", Icon: Icon{Type: IconLucide, Value: "cloud"}, Order: 80},
	{Key: CategoryNetwork, Label: "Network", Icon: Icon{Type: IconLucide, Value: "network"}, Order: 90},
	{Key: CategorySecurity, Label: "Security", Icon: Icon{Type: IconLucide, Value: "shield"}, Order: 100},
	{Key: CategoryDevOps, Label: "DevOps & CI", Icon: Icon{Type: IconLucide, Value: "git-branch"}, Order: 110},
	{Key: CategoryObservability, Label: "Observability", Icon: Icon{Type: IconLucide, Value: "activity"}, Order: 120},
	{Key: CategoryMessaging, Label: "Messaging", Icon: Icon{Type: IconLucide, Value: "messages-square"}, Order: 130},
	{Key: CategoryOther, Label: "Other", Icon: Icon{Type: IconLucide, Value: "plug"}, Order: 1000},
}

var builtInCategoryByKey = func() map[Category]CategoryInfo {
	out := make(map[Category]CategoryInfo, len(builtInPluginCategories))
	for _, info := range builtInPluginCategories {
		out[info.Key] = info
	}
	return out
}()

func CategoryLookup(category Category) (CategoryInfo, bool) {
	info, ok := builtInCategoryByKey[category]
	return info, ok
}

func pluginCategoryInfo(category Category) CategoryInfo {
	if info, ok := CategoryLookup(category); ok {
		return info
	}
	return builtInCategoryByKey[CategoryOther]
}
