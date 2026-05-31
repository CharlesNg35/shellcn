package kubernetes

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
)

// These integration tests provision a real single-node k3s cluster in Docker and
// drive the plugin's ops handlers against it. Env-gated so the default test run
// (and CI without Docker) skips them.
//
//	SHELLCN_KUBERNETES_INTEGRATION=1 go test ./plugins/kubernetes/... -run Integration -count=1 -timeout 300s
const (
	integrationEnv = "SHELLCN_KUBERNETES_INTEGRATION"
	k3sImage       = "rancher/k3s:v1.31.5-k3s1"
)

func requireIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv(integrationEnv) != "1" {
		t.Skipf("set %s=1 to run the Kubernetes integration tests (needs Docker)", integrationEnv)
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not found in PATH")
	}
}

// startK3s launches a single-node k3s container and returns a kubeconfig pointed
// at the mapped API port. The container is removed in t.Cleanup.
func startK3s(t *testing.T) string {
	t.Helper()
	name := "shellcn-k3s-" + strings.ReplaceAll(t.Name(), "/", "-")
	_ = exec.Command("docker", "rm", "-f", name).Run()

	// k3s needs a privileged container; disable bundled components we don't use to
	// speed up readiness. Publish the API server on an ephemeral host port.
	runArgs := []string{
		"run", "-d", "--privileged", "--name", name,
		"-p", "127.0.0.1:0:6443",
		"-e", "K3S_KUBECONFIG_OUTPUT=/output/kubeconfig.yaml",
		"-e", "K3S_KUBECONFIG_MODE=666",
		k3sImage,
		"server",
		"--disable", "traefik,servicelb,metrics-server,local-storage",
		"--disable-network-policy",
		"--tls-san", "127.0.0.1",
	}
	out, err := exec.Command("docker", runArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("docker run k3s: %v: %s", err, out)
	}
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", name).Run()
	})

	hostPort := mappedPort(t, name, "6443/tcp")

	kubeconfig := waitForKubeconfig(t, name)
	// Rewrite the in-container server URL to the host-mapped port.
	kubeconfig = rewriteServer(kubeconfig, fmt.Sprintf("https://127.0.0.1:%s", hostPort))
	return kubeconfig
}

func mappedPort(t *testing.T, name, containerPort string) string {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		out, err := exec.Command("docker", "port", name, containerPort).CombinedOutput()
		if err == nil {
			line := strings.TrimSpace(string(out))
			if i := strings.LastIndex(line, ":"); i >= 0 {
				return strings.TrimSpace(line[i+1:])
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("docker port %s %s: %v: %s", name, containerPort, err, out)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func waitForKubeconfig(t *testing.T, name string) string {
	t.Helper()
	deadline := time.Now().Add(120 * time.Second)
	for {
		out, err := exec.Command("docker", "exec", name, "cat", "/etc/rancher/k3s/k3s.yaml").CombinedOutput()
		if err == nil && strings.Contains(string(out), "client-certificate-data") {
			return string(out)
		}
		if time.Now().After(deadline) {
			t.Fatalf("k3s kubeconfig not ready: %v: %s", err, out)
		}
		time.Sleep(time.Second)
	}
}

func rewriteServer(kubeconfig, server string) string {
	lines := strings.Split(kubeconfig, "\n")
	for i, ln := range lines {
		if strings.Contains(ln, "server:") {
			indent := ln[:len(ln)-len(strings.TrimLeft(ln, " "))]
			lines[i] = indent + "server: " + server
		}
	}
	return strings.Join(lines, "\n")
}

// connectK3s connects the plugin's session to the cluster and waits for the API
// to be reachable and a node to register.
func connectK3s(t *testing.T, kubeconfig string) *Session {
	t.Helper()
	var s *Session
	deadline := time.Now().Add(120 * time.Second)
	for {
		sess, err := Connect(context.Background(), plugin.ConnectConfig{
			ConnectionID: "it",
			Transport:    plugin.TransportDirect,
			Config:       map[string]any{"kubeconfig": kubeconfig},
			Net:          fakeNet{},
		})
		if err == nil {
			s = sess.(*Session)
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("connect to k3s: %v", err)
		}
		time.Sleep(2 * time.Second)
	}
	t.Cleanup(func() { _ = s.Close() })

	// Wait for the single node to register and become Ready.
	deadline = time.Now().Add(120 * time.Second)
	for {
		nodes, err := s.clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		if err == nil && len(nodes.Items) > 0 && nodeReady(&nodes.Items[0]) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("k3s node did not become Ready in time")
		}
		time.Sleep(2 * time.Second)
	}
	return s
}

func nodeReady(n *corev1.Node) bool {
	for _, c := range n.Status.Conditions {
		if c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func rcBody(s *Session, params map[string]string, body string) *plugin.RequestContext {
	var b []byte
	if body != "" {
		b = []byte(body)
	}
	return plugin.NewRequestContext(context.Background(), models.User{ID: "u1"}, s, params, url.Values{}, b)
}

func TestIntegrationKubernetesOps(t *testing.T) {
	requireIntegration(t)
	kubeconfig := startK3s(t)
	s := connectK3s(t, kubeconfig)
	ctx := context.Background()

	nodes, err := s.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil || len(nodes.Items) == 0 {
		t.Fatalf("list nodes: %v", err)
	}
	nodeName := nodes.Items[0].Name

	t.Run("cordon and uncordon", func(t *testing.T) {
		if _, err := CordonNode(rcBody(s, map[string]string{"kind": "node", "name": nodeName}, "")); err != nil {
			t.Fatalf("cordon: %v", err)
		}
		n, err := s.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil || !n.Spec.Unschedulable {
			t.Fatalf("node not cordoned: unschedulable=%v err=%v", n.Spec.Unschedulable, err)
		}
		if _, err := UncordonNode(rcBody(s, map[string]string{"kind": "node", "name": nodeName}, "")); err != nil {
			t.Fatalf("uncordon: %v", err)
		}
		n, err = s.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil || n.Spec.Unschedulable {
			t.Fatalf("node not uncordoned: unschedulable=%v err=%v", n.Spec.Unschedulable, err)
		}
	})

	t.Run("rollout undo reverts the template", func(t *testing.T) {
		dep := newDeployment("web", "nginx:1.25")
		if _, err := s.clientset.AppsV1().Deployments("default").Create(ctx, dep, metav1.CreateOptions{}); err != nil {
			t.Fatalf("create deployment: %v", err)
		}
		t.Cleanup(func() {
			_ = s.clientset.AppsV1().Deployments("default").Delete(ctx, "web", metav1.DeleteOptions{})
		})
		waitDeploymentRevision(t, s, "web", "1")

		// New revision: change the image.
		patch := []byte(`{"spec":{"template":{"spec":{"containers":[{"name":"app","image":"nginx:1.26"}]}}}}`)
		if _, err := s.clientset.AppsV1().Deployments("default").Patch(ctx, "web", "application/strategic-merge-patch+json", patch, metav1.PatchOptions{}); err != nil {
			t.Fatalf("patch deployment image: %v", err)
		}
		waitDeploymentRevision(t, s, "web", "2")

		if _, err := RolloutUndo(rcBody(s, map[string]string{"kind": "deployment", "namespace": "default", "name": "web"}, "")); err != nil {
			t.Fatalf("rollout undo: %v", err)
		}

		// After undo the live template image must be back to the original.
		waitDeploymentImage(t, s, "web", "nginx:1.25")
	})

	t.Run("cronjob trigger creates a job", func(t *testing.T) {
		cj := newCronJob("report")
		if _, err := s.clientset.BatchV1().CronJobs("default").Create(ctx, cj, metav1.CreateOptions{}); err != nil {
			t.Fatalf("create cronjob: %v", err)
		}
		t.Cleanup(func() {
			pol := metav1.DeletePropagationBackground
			_ = s.clientset.BatchV1().CronJobs("default").Delete(ctx, "report", metav1.DeleteOptions{PropagationPolicy: &pol})
		})

		out, err := TriggerCronJob(rcBody(s, map[string]string{"kind": "cronjob", "namespace": "default", "name": "report"}, ""))
		if err != nil {
			t.Fatalf("trigger cronjob: %v", err)
		}
		res := out.(map[string]any)
		jobName, _ := res["job"].(string)
		if jobName == "" {
			t.Fatalf("trigger result missing job name: %v", res)
		}
		job, err := s.clientset.BatchV1().Jobs("default").Get(ctx, jobName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("get triggered job %q: %v", jobName, err)
		}
		if len(job.OwnerReferences) != 1 || job.OwnerReferences[0].Kind != "CronJob" {
			t.Fatalf("triggered job not owned by the cronjob: %+v", job.OwnerReferences)
		}
	})

	t.Run("drain cordons and evicts an evictable pod", func(t *testing.T) {
		// A standalone Deployment pod is evictable (ReplicaSet-managed). Drain must
		// cordon the node and evict it; we assert the eviction by waiting for the
		// pod's UID to change (the ReplicaSet reschedules a fresh pod).
		dep := newDeployment("drainme", "nginx:1.25")
		if _, err := s.clientset.AppsV1().Deployments("default").Create(ctx, dep, metav1.CreateOptions{}); err != nil {
			t.Fatalf("create deployment: %v", err)
		}
		t.Cleanup(func() {
			_ = s.clientset.AppsV1().Deployments("default").Delete(ctx, "drainme", metav1.DeleteOptions{})
		})
		origUID := waitRunningPodUID(t, s, "app=drainme")

		out, err := DrainNode(rcBody(s, map[string]string{"kind": "node", "name": nodeName}, `{"gracePeriodSeconds":0}`))
		if err != nil {
			t.Fatalf("drain: %v", err)
		}
		res := out.(map[string]any)
		if ev, _ := res["evicted"].(int); ev < 1 {
			t.Fatalf("drain evicted %v pods, want >= 1 (result=%v)", res["evicted"], res)
		}

		n, err := s.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil || !n.Spec.Unschedulable {
			t.Fatalf("drain did not cordon node: unschedulable=%v err=%v", n.Spec.Unschedulable, err)
		}

		// The evicted pod must be replaced by a different one.
		deadline := time.Now().Add(60 * time.Second)
		for {
			newUID := runningOrPendingPodUID(s, "app=drainme")
			if newUID != "" && newUID != origUID {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("evicted pod was not replaced (orig=%s)", origUID)
			}
			time.Sleep(2 * time.Second)
		}

		// Restore schedulability so cleanup deletes settle cleanly.
		_, _ = UncordonNode(rcBody(s, map[string]string{"kind": "node", "name": nodeName}, ""))
	})
}

func newDeployment(name, image string) *appsv1.Deployment {
	replicas := int32(1)
	labels := map[string]string{"app": name}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: ptr(int64(0)),
					Containers:                    []corev1.Container{{Name: "app", Image: image}},
				},
			},
		},
	}
}

func newCronJob(name string) *batchv1.CronJob {
	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 0 31 2 *", // Feb 31: never fires on its own.
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyNever,
							Containers:    []corev1.Container{{Name: "c", Image: "busybox", Command: []string{"true"}}},
						},
					},
				},
			},
		},
	}
}

func waitDeploymentRevision(t *testing.T, s *Session, name, want string) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for {
		dep, err := s.clientset.AppsV1().Deployments("default").Get(context.Background(), name, metav1.GetOptions{})
		if err == nil && dep.Annotations[revisionAnnotation] == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("deployment %s did not reach revision %s", name, want)
		}
		time.Sleep(time.Second)
	}
}

func waitDeploymentImage(t *testing.T, s *Session, name, want string) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for {
		dep, err := s.clientset.AppsV1().Deployments("default").Get(context.Background(), name, metav1.GetOptions{})
		if err == nil && len(dep.Spec.Template.Spec.Containers) > 0 && dep.Spec.Template.Spec.Containers[0].Image == want {
			return
		}
		if time.Now().After(deadline) {
			got := "<none>"
			if dep, e := s.clientset.AppsV1().Deployments("default").Get(context.Background(), name, metav1.GetOptions{}); e == nil && len(dep.Spec.Template.Spec.Containers) > 0 {
				got = dep.Spec.Template.Spec.Containers[0].Image
			}
			t.Fatalf("deployment %s image = %s, want %s after rollout undo", name, got, want)
		}
		time.Sleep(time.Second)
	}
}

func waitRunningPodUID(t *testing.T, s *Session, selector string) string {
	t.Helper()
	deadline := time.Now().Add(120 * time.Second)
	for {
		if uid := runningOrPendingPodUID(s, selector); uid != "" {
			return uid
		}
		if time.Now().After(deadline) {
			t.Fatalf("no pod for selector %q became schedulable", selector)
		}
		time.Sleep(2 * time.Second)
	}
}

func runningOrPendingPodUID(s *Session, selector string) string {
	pods, err := s.clientset.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return ""
	}
	for i := range pods.Items {
		p := &pods.Items[i]
		if p.DeletionTimestamp != nil {
			continue
		}
		return string(p.UID)
	}
	return ""
}
