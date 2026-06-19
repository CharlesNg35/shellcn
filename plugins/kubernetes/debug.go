package kubernetes

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

const defaultDebugImage = "busybox:1.36"

// debugAction creates an ephemeral debug container, then opens an exec terminal
// into it via the open_panel effect (passing the name through ${response.container}).
func debugAction() plugin.Action {
	return plugin.Action{
		ID: "kubernetes.pod.debug", Label: "Debug", Icon: lucide("bug"),
		RouteID: "kubernetes.pod.debug.create",
		Params:  map[string]string{"namespace": "${resource.namespace}", "name": "${resource.name}"},
		Confirm: true,
		ConfirmText: "Add an ephemeral debug container to this pod and open a shell? " +
			"The container persists until the pod is recreated.",
		EnabledWhen: &plugin.Condition{AllOf: []plugin.Rule{
			{Field: "status", Op: plugin.OpEq, Value: "Running"},
			{Field: "can.patch", Op: plugin.OpEq, Value: true},
		}},
		OnSuccess: &plugin.ActionSuccess{Effects: []plugin.ActionEffect{{
			Type: plugin.ActionEffectOpenPanel,
			OpenPanel: &plugin.OpenPanelEffect{
				Open: plugin.OpenDock, Panel: plugin.PanelTerminal, Title: "Debug", Icon: lucide("bug"),
				Source: &plugin.DataSource{
					RouteID: "kubernetes.pod.exec", Method: plugin.MethodWS,
					Params: map[string]string{
						"namespace": "${response.namespace}",
						"name":      "${response.name}",
						"container": "${response.container}",
						"tty":       "true", "cols": "80", "rows": "24",
					},
				},
				Config: plugin.TerminalConfig{Zoom: true, Search: true},
			},
		}}},
	}
}

// DebugCreate adds an ephemeral debug container to a running pod and returns once
// it is running; the action's onSuccess then execs into it. Keeping create and
// attach separate means the container is only added on an explicit Debug click.
func DebugCreate(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	ns, pod := rc.Param("namespace"), rc.Param("name")
	if err := validateNamespace(ns); err != nil {
		return nil, err
	}
	if err := validateName(pod); err != nil {
		return nil, err
	}
	image := param(rc, "image")
	if image == "" {
		image = defaultDebugImage
	}
	name, err := s.addEphemeralContainer(rc.Ctx, ns, pod, image, param(rc, "target"))
	if err != nil {
		return nil, err
	}
	if err := s.waitEphemeralRunning(rc.Ctx, ns, pod, name); err != nil {
		return nil, err
	}
	return map[string]any{"container": name, "namespace": ns, "name": pod}, nil
}

// addEphemeralContainer appends a debug container to the pod's ephemeralcontainers
// subresource and returns its generated name.
func (s *Session) addEphemeralContainer(ctx context.Context, ns, pod, image, target string) (string, error) {
	p, err := s.clientset.CoreV1().Pods(ns).Get(ctx, pod, metav1.GetOptions{})
	if err != nil {
		return "", apiErr(err)
	}
	name := "debugger-" + utilrand.String(5)
	p.Spec.EphemeralContainers = append(p.Spec.EphemeralContainers, corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:                     name,
			Image:                    image,
			ImagePullPolicy:          corev1.PullIfNotPresent,
			Stdin:                    true,
			TTY:                      true,
			TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		},
		TargetContainerName: target,
	})
	if _, err := s.clientset.CoreV1().Pods(ns).UpdateEphemeralContainers(ctx, pod, p, metav1.UpdateOptions{}); err != nil {
		return "", apiErr(err)
	}
	return name, nil
}

// waitEphemeralRunning blocks until the debug container is running, surfacing a
// terminated state (e.g. an image-pull failure) instead of hanging.
func (s *Session) waitEphemeralRunning(ctx context.Context, ns, pod, name string) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	for {
		p, err := s.clientset.CoreV1().Pods(ns).Get(ctx, pod, metav1.GetOptions{})
		if err != nil {
			return apiErr(err)
		}
		for _, st := range p.Status.EphemeralContainerStatuses {
			if st.Name != name {
				continue
			}
			if st.State.Running != nil {
				return nil
			}
			if t := st.State.Terminated; t != nil {
				return fmt.Errorf("%w: debug container exited (%s)", plugin.ErrUnavailable, t.Reason)
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: debug container did not start in time", plugin.ErrUnavailable)
		case <-time.After(500 * time.Millisecond):
		}
	}
}
