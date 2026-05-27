package kubernetes

import (
	"context"
	"encoding/json"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/charlesng/shellcn/internal/plugin"
)

const metricsInterval = 10 * time.Second

// ClusterMetrics streams cluster-wide CPU/memory/pod usage vs capacity. Frames
// degrade gracefully (metricsAvailable=false) when metrics-server is absent.
func ClusterMetrics(rc *plugin.RequestContext, client plugin.ClientStream) error {
	return metricsLoop(rc, client, func(ctx context.Context) map[string]any {
		s, _ := sess(rc)
		return s.clusterFrame(ctx)
	})
}

// NodeMetrics streams one node's CPU/memory usage vs capacity.
func NodeMetrics(rc *plugin.RequestContext, client plugin.ClientStream) error {
	node := rc.Param("name")
	return metricsLoop(rc, client, func(ctx context.Context) map[string]any {
		s, _ := sess(rc)
		return s.nodeFrame(ctx, node)
	})
}

func metricsLoop(rc *plugin.RequestContext, client plugin.ClientStream, frame func(context.Context) map[string]any) error {
	enc := json.NewEncoder(client)
	ticker := time.NewTicker(metricsInterval)
	defer ticker.Stop()
	for {
		if err := enc.Encode(frame(rc.Ctx)); err != nil {
			return nil
		}
		select {
		case <-client.Context().Done():
			return nil
		case <-rc.Ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (s *Session) clusterFrame(ctx context.Context) map[string]any {
	frame := map[string]any{"metricsAvailable": false}

	nodes, err := s.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return frame
	}
	var cpuCap, memCap, podCap int64
	for i := range nodes.Items {
		alloc := nodes.Items[i].Status.Allocatable
		cpuCap += alloc.Cpu().MilliValue()
		memCap += alloc.Memory().Value()
		podCap += alloc.Pods().Value()
	}
	frame["nodes"] = len(nodes.Items)
	frame["cpuCapacity"] = milliToCores(cpuCap)
	frame["memCapacity"] = memCap
	frame["podCapacity"] = podCap

	if pods, err := s.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{}); err == nil {
		frame["pods"] = len(pods.Items)
	}

	nm, err := s.metrics.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return frame
	}
	var cpuUse, memUse int64
	for i := range nm.Items {
		cpuUse += nm.Items[i].Usage.Cpu().MilliValue()
		memUse += nm.Items[i].Usage.Memory().Value()
	}
	frame["metricsAvailable"] = true
	frame["cpu"] = milliToCores(cpuUse)
	frame["mem"] = memUse
	frame["cpuPct"] = pct(cpuUse, cpuCap)
	frame["memPct"] = pct(memUse, memCap)
	return frame
}

func (s *Session) nodeFrame(ctx context.Context, node string) map[string]any {
	frame := map[string]any{"metricsAvailable": false}
	n, err := s.clientset.CoreV1().Nodes().Get(ctx, node, metav1.GetOptions{})
	if err != nil {
		return frame
	}
	cpuCap := n.Status.Allocatable.Cpu().MilliValue()
	memCap := n.Status.Allocatable.Memory().Value()
	frame["cpuCapacity"] = milliToCores(cpuCap)
	frame["memCapacity"] = memCap

	nm, err := s.metrics.MetricsV1beta1().NodeMetricses().Get(ctx, node, metav1.GetOptions{})
	if err != nil {
		return frame
	}
	cpuUse := nm.Usage.Cpu().MilliValue()
	memUse := nm.Usage.Memory().Value()
	frame["metricsAvailable"] = true
	frame["cpu"] = milliToCores(cpuUse)
	frame["mem"] = memUse
	frame["cpuPct"] = pct(cpuUse, cpuCap)
	frame["memPct"] = pct(memUse, memCap)
	return frame
}

func milliToCores(m int64) float64 { return float64(m) / 1000 }

func pct(used, capacity int64) float64 {
	if capacity <= 0 {
		return 0
	}
	return float64(used) / float64(capacity) * 100
}

// nodePodSelector limits a pod list to one node.
func nodePodSelector(node string) string { return "spec.nodeName=" + node }
