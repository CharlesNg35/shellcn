package kubernetes

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/charlesng35/shellcn/internal/plugin"
)

const (
	shellImage        = "docker.io/bitnami/kubectl:latest"
	shellContainer    = "shell"
	shellPodName      = "shellcn-shell"
	shellNamespace    = "default"
	shellPodLabel     = "shellcn.io/cluster-shell"
	shellStartTimeout = 90 * time.Second
	// shellKeepalive idles the container so an interactive exec has something to
	// attach to; it exits cleanly on the pod's termination signal.
	shellKeepalive = "trap : TERM INT; sleep 2147483647 & wait"
)

// ClusterShellStream attaches an interactive shell to a long-lived kubectl pod,
// giving the operator cluster-scoped kubectl from inside the cluster. A single
// fixed-name pod is reused across sessions, so reconnects are instant and never
// pile up duplicates.
func ClusterShellStream(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, err := sess(rc)
	if err != nil {
		return err
	}
	pods := s.clientset.CoreV1().Pods(shellNamespace)
	if err := ensureShellPod(rc.Ctx, client.Context(), pods); err != nil {
		return err
	}

	exec, err := s.podExecutor(shellNamespace, shellPodName, &corev1.PodExecOptions{
		Container: shellContainer,
		Command:   shellExecCommand(rc),
		Stdin:     true,
		Stdout:    true,
		TTY:       true,
	})
	if err != nil {
		return err
	}
	return streamExec(client, exec, true, intParam(rc, "cols"), intParam(rc, "rows"))
}

func shellExecCommand(rc *plugin.RequestContext) []string {
	if c := param(rc, "command"); c != "" {
		return strings.Fields(c)
	}
	return []string{"/bin/sh"}
}

// ensureShellPod reuses a healthy shell pod, recreating it only when missing or
// dead, then blocks until it is ready to exec into.
func ensureShellPod(ctx, waitCtx context.Context, pods corev1client.PodInterface) error {
	if p, err := pods.Get(ctx, shellPodName, metav1.GetOptions{}); err == nil && p.DeletionTimestamp == nil {
		switch {
		case podReady(p):
			return nil
		case p.Status.Phase == corev1.PodFailed || p.Status.Phase == corev1.PodSucceeded:
			grace := int64(0)
			_ = pods.Delete(ctx, shellPodName, metav1.DeleteOptions{GracePeriodSeconds: &grace})
		default:
			return waitPodRunning(waitCtx, pods, shellPodName)
		}
	}
	if _, err := pods.Create(ctx, shellPod(), metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return apiErr(err)
	}
	return waitPodRunning(waitCtx, pods, shellPodName)
}

func shellPod() *corev1.Pod {
	automount := true
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: shellPodName,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "shellcn",
				shellPodLabel:                  "true",
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy:                corev1.RestartPolicyAlways,
			AutomountServiceAccountToken: &automount,
			Containers: []corev1.Container{{
				Name:    shellContainer,
				Image:   shellImage,
				Command: []string{"/bin/sh", "-c", shellKeepalive},
				Stdin:   true,
				TTY:     true,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("25m"),
						corev1.ResourceMemory: resource.MustParse("32Mi"),
					},
				},
			}},
		},
	}
}

func podReady(p *corev1.Pod) bool {
	if p.Status.Phase != corev1.PodRunning {
		return false
	}
	for _, cs := range p.Status.ContainerStatuses {
		if cs.Ready {
			return true
		}
	}
	return false
}

// waitPodRunning blocks until the shell pod is ready, failing fast on an
// unrecoverable container state (e.g. an image that can't be pulled).
func waitPodRunning(ctx context.Context, pods corev1client.PodInterface, name string) error {
	return wait.PollUntilContextTimeout(ctx, 500*time.Millisecond, shellStartTimeout, true, func(ctx context.Context) (bool, error) {
		p, err := pods.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		for _, cs := range p.Status.ContainerStatuses {
			if w := cs.State.Waiting; w != nil {
				switch w.Reason {
				case "ErrImagePull", "ImagePullBackOff", "InvalidImageName",
					"CreateContainerError", "CreateContainerConfigError":
					return false, fmt.Errorf("%w: shell pod: %s", plugin.ErrUnavailable, w.Message)
				}
			}
		}
		if p.Status.Phase == corev1.PodFailed {
			return false, fmt.Errorf("%w: shell pod failed to start", plugin.ErrUnavailable)
		}
		return podReady(p), nil
	})
}
