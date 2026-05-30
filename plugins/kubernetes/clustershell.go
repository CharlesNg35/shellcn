package kubernetes

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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
	shellSAName       = "shellcn-shell"
	shellNamespace    = "default"
	shellPodLabel     = "shellcn.io/cluster-shell"
	shellStartTimeout = 90 * time.Second
	// shellKeepalive idles the container so an interactive exec has something to
	// attach to; it exits cleanly on the pod's termination signal.
	shellKeepalive = "trap : TERM INT; sleep 2147483647 & wait"
)

// shellLaunch prefers an interactive bash (most kubectl images ship it) and falls
// back to POSIX sh. It sets a sane TERM and, since these minimal images carry no
// terminfo, aliases clear/reset to raw ANSI so screen-clearing works regardless.
const shellLaunch = `export TERM="${TERM:-xterm-256color}"
if command -v bash >/dev/null 2>&1; then
rc="$(mktemp 2>/dev/null || echo /tmp/.shellcn_bashrc)"
cat >"$rc" <<'SHRC'
alias clear='printf "\033[H\033[2J\033[3J"'
alias reset='printf "\033c"'
SHRC
exec bash --rcfile "$rc"
fi
exec sh`

// ClusterShellStream attaches an interactive shell to a long-lived kubectl pod,
// giving the operator cluster-scoped kubectl from inside the cluster. A single
// fixed-name pod is reused across sessions, so reconnects are instant and never
// pile up duplicates.
func ClusterShellStream(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, err := sess(rc)
	if err != nil {
		return err
	}
	if err := ensureShellPod(rc.Ctx, client.Context(), s); err != nil {
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
	return []string{"/bin/sh", "-c", shellLaunch}
}

// ensureShellPod reuses a healthy shell pod, recreating it only when missing or
// dead, then blocks until it is ready to exec into.
func ensureShellPod(ctx, waitCtx context.Context, s *Session) error {
	pods := s.clientset.CoreV1().Pods(shellNamespace)
	useSA := ensureShellRBAC(ctx, s)
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
	if _, err := pods.Create(ctx, shellPod(useSA), metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return apiErr(err)
	}
	return waitPodRunning(waitCtx, pods, shellPodName)
}

// ensureShellRBAC idempotently provisions a cluster-admin service account for the
// shell, so its kubectl can actually reach the cluster (matching how the agent
// install binds cluster-admin). It reports whether the dedicated SA is usable;
// if it can't be created the caller falls back to the namespace default SA.
func ensureShellRBAC(ctx context.Context, s *Session) bool {
	sa := s.clientset.CoreV1().ServiceAccounts(shellNamespace)
	if _, err := sa.Create(ctx, shellServiceAccount(), metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return false
	}
	// A missing binding only narrows what the shell can do; the SA is still usable.
	crb := s.clientset.RbacV1().ClusterRoleBindings()
	_, _ = crb.Create(ctx, shellClusterRoleBinding(), metav1.CreateOptions{})
	return true
}

func shellLabels() map[string]string {
	return map[string]string{
		"app.kubernetes.io/managed-by": "shellcn",
		shellPodLabel:                  "true",
	}
}

func shellServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: shellSAName, Namespace: shellNamespace, Labels: shellLabels()},
	}
}

func shellClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: shellSAName, Labels: shellLabels()},
		RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "cluster-admin"},
		Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: shellSAName, Namespace: shellNamespace}},
	}
}

func shellPod(useSA bool) *corev1.Pod {
	automount := true
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: shellPodName, Labels: shellLabels()},
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
	if useSA {
		pod.Spec.ServiceAccountName = shellSAName
	}
	return pod
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
