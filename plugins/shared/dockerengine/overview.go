package dockerengine

import (
	"context"
	"encoding/json"
	"time"

	"github.com/charlesng35/shellcn/internal/plugin"
	dockerclient "github.com/moby/moby/client"
)

// OverviewKind is the singleton resource kind backing an engine overview
// dashboard. Each engine plugin declares it in its own manifest.
const OverviewKind = "overview"

const overviewInterval = 5 * time.Second

// OverviewRef opens the overview dashboard from a tree group.
func OverviewRef() *plugin.ResourceRef {
	return &plugin.ResourceRef{Kind: OverviewKind, Name: "Overview", UID: OverviewKind}
}

// OverviewList is the single row backing the overview ResourceType.
func OverviewList(_ *plugin.RequestContext) (any, error) {
	row := Row{"name": "Overview", "uid": OverviewKind, "ref": *OverviewRef()}
	return plugin.Page[Row]{Items: []Row{row}, Total: ptr(1)}, nil
}

// MetricsLoop streams frames on a fixed cadence until the client or request
// context closes. A frame builder that can't reach its data returns what it has.
func MetricsLoop(rc *plugin.RequestContext, stream plugin.ClientStream, frame func(context.Context) map[string]any) error {
	enc := json.NewEncoder(stream)
	ticker := time.NewTicker(overviewInterval)
	defer ticker.Stop()
	for {
		if err := enc.Encode(frame(rc.Ctx)); err != nil {
			return nil
		}
		select {
		case <-stream.Context().Done():
			return nil
		case <-rc.Ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// OverviewMetricsConfig declares the environment count tiles.
func OverviewMetricsConfig() plugin.MetricsConfig {
	return plugin.MetricsConfig{Stats: []plugin.MetricStat{
		{Key: "containers", Label: "Containers"},
		{Key: "running", Label: "Running"},
		{Key: "stopped", Label: "Stopped"},
		{Key: "images", Label: "Images"},
		{Key: "volumes", Label: "Volumes"},
		{Key: "networks", Label: "Networks"},
	}}
}

// OverviewMetrics streams live counts of the engine's containers, images,
// volumes, and networks.
func OverviewMetrics(rc *plugin.RequestContext, stream plugin.ClientStream) error {
	return MetricsLoop(rc, stream, func(ctx context.Context) map[string]any {
		s, err := sess(rc)
		if err != nil {
			return map[string]any{}
		}
		return engineFrame(ctx, s)
	})
}

func engineFrame(ctx context.Context, s *Session) map[string]any {
	frame := map[string]any{}
	if res, err := s.cli.ContainerList(ctx, dockerclient.ContainerListOptions{All: true}); err == nil {
		var running, stopped int
		for _, c := range res.Items {
			if c.State == "running" {
				running++
			} else {
				stopped++
			}
		}
		frame["containers"] = len(res.Items)
		frame["running"] = running
		frame["stopped"] = stopped
	}
	if res, err := s.cli.ImageList(ctx, dockerclient.ImageListOptions{All: true}); err == nil {
		frame["images"] = len(res.Items)
	}
	if res, err := s.cli.VolumeList(ctx, dockerclient.VolumeListOptions{}); err == nil {
		frame["volumes"] = len(res.Items)
	}
	if res, err := s.cli.NetworkList(ctx, dockerclient.NetworkListOptions{}); err == nil {
		frame["networks"] = len(res.Items)
	}
	return frame
}
