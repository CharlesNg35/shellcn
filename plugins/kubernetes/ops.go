package kubernetes

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

// revisionAnnotation is the stable annotation the Deployment controller stamps on
// each owned ReplicaSet to track its rollout revision.
const revisionAnnotation = "deployment.kubernetes.io/revision"

// maxGracePeriodSeconds bounds a drain/eviction grace period to keep a single
// request from holding the node indefinitely.
const maxGracePeriodSeconds = int64(3600)

type DrainRequest struct {
	GracePeriodSeconds int64 `json:"gracePeriodSeconds"`
	Force              bool  `json:"force"` // evict bare (unmanaged) pods too
}

// DrainNode cordons a node (Unschedulable=true) and evicts its pods through the
// Eviction API, honoring PodDisruptionBudgets. Mirror/static and DaemonSet pods
// are skipped (as kubectl drain does); bare pods are skipped unless force.
func DrainNode(rc *plugin.RequestContext) (any, error) {
	var req DrainRequest
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	if req.GracePeriodSeconds < 0 || req.GracePeriodSeconds > maxGracePeriodSeconds {
		return nil, fmt.Errorf("%w: gracePeriodSeconds must be between 0 and %d", plugin.ErrInvalidInput, maxGracePeriodSeconds)
	}
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	name := rc.Param("name")
	if err := validateName(name); err != nil {
		return nil, err
	}

	cordon := []byte(`{"spec":{"unschedulable":true}}`)
	if _, err := s.clientset.CoreV1().Nodes().Patch(rc.Ctx, name, types.MergePatchType, cordon, metav1.PatchOptions{}); err != nil {
		return nil, apiErr(err)
	}

	pods, err := s.clientset.CoreV1().Pods("").List(rc.Ctx, metav1.ListOptions{FieldSelector: "spec.nodeName=" + name})
	if err != nil {
		return nil, apiErr(err)
	}

	var grace *int64
	if req.GracePeriodSeconds > 0 {
		grace = &req.GracePeriodSeconds
	}
	evicted, skipped := 0, 0
	for i := range pods.Items {
		p := &pods.Items[i]
		if !evictablePod(p, req.Force) {
			skipped++
			continue
		}
		ev := &policyv1.Eviction{
			ObjectMeta:    metav1.ObjectMeta{Name: p.Name, Namespace: p.Namespace},
			DeleteOptions: &metav1.DeleteOptions{GracePeriodSeconds: grace},
		}
		err := s.clientset.PolicyV1().Evictions(p.Namespace).Evict(rc.Ctx, ev)
		switch {
		case err == nil:
			evicted++
		case apierrors.IsNotFound(err):
			// Pod already gone; nothing to evict.
		default:
			return nil, apiErr(err)
		}
	}
	return map[string]any{"ok": true, "evicted": evicted, "skipped": skipped}, nil
}

// evictablePod reports whether drain should evict pod p: skip mirror/static pods
// and DaemonSet-managed pods always; skip bare (unowned) pods unless force.
func evictablePod(p *corev1.Pod, force bool) bool {
	if p == nil {
		return false
	}
	if _, mirror := p.Annotations[corev1.MirrorPodAnnotationKey]; mirror {
		return false
	}
	owner := metav1.GetControllerOf(&p.ObjectMeta)
	if owner != nil && owner.Kind == "DaemonSet" {
		return false
	}
	if owner == nil && !force {
		return false
	}
	return true
}

// RolloutUndo rolls a Deployment back to its previous revision: it finds the
// owned ReplicaSet with the highest revision below the Deployment's current one
// and patches the Deployment's pod template back to that ReplicaSet's template.
func RolloutUndo(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	name := rc.Param("name")
	if err := validateName(name); err != nil {
		return nil, err
	}
	ns := rc.Param("namespace")
	if err := validateNamespace(ns); err != nil {
		return nil, err
	}

	dep, err := s.clientset.AppsV1().Deployments(ns).Get(rc.Ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, apiErr(err)
	}
	sel, err := metav1.LabelSelectorAsSelector(dep.Spec.Selector)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", plugin.ErrInvalidInput, err)
	}
	rsList, err := s.clientset.AppsV1().ReplicaSets(ns).List(rc.Ctx, metav1.ListOptions{LabelSelector: sel.String()})
	if err != nil {
		return nil, apiErr(err)
	}

	prev, err := previousReplicaSet(dep, rsList.Items)
	if err != nil {
		return nil, err
	}

	template := rolloutUndoTemplate(prev.Spec.Template)
	patch, err := json.Marshal(map[string]any{"spec": map[string]any{"template": template}})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", plugin.ErrInvalidInput, err)
	}
	if _, err := s.clientset.AppsV1().Deployments(ns).Patch(rc.Ctx, name, types.StrategicMergePatchType, patch, metav1.PatchOptions{}); err != nil {
		return nil, apiErr(err)
	}
	return map[string]any{"ok": true, "revision": revisionOf(prev.Annotations)}, nil
}

// previousReplicaSet selects the ReplicaSet to roll back to: the one owned by dep
// with the highest revision strictly below dep's current revision.
func previousReplicaSet(dep *appsv1.Deployment, all []appsv1.ReplicaSet) (*appsv1.ReplicaSet, error) {
	current := revisionOf(dep.Annotations)
	owned := make([]appsv1.ReplicaSet, 0, len(all))
	for i := range all {
		rs := all[i]
		if metav1.IsControlledBy(&rs, dep) {
			owned = append(owned, rs)
		}
	}
	sort.Slice(owned, func(i, j int) bool {
		return revisionOf(owned[i].Annotations) > revisionOf(owned[j].Annotations)
	})
	for i := range owned {
		rev := revisionOf(owned[i].Annotations)
		if rev > 0 && rev < current {
			return &owned[i], nil
		}
	}
	return nil, fmt.Errorf("%w: no previous revision to roll back to", plugin.ErrNotFound)
}

// revisionOf reads the rollout revision from an object's annotations (0 if unset
// or unparseable).
func revisionOf(annotations map[string]string) int64 {
	v, ok := annotations[revisionAnnotation]
	if !ok {
		return 0
	}
	n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// rolloutUndoTemplate normalizes a ReplicaSet's pod template for patching back
// onto a Deployment: the controller-managed pod-template-hash label must be
// dropped so the Deployment recomputes it.
func rolloutUndoTemplate(tmpl corev1.PodTemplateSpec) corev1.PodTemplateSpec {
	out := *tmpl.DeepCopy()
	delete(out.Labels, appsv1.DefaultDeploymentUniqueLabelKey)
	return out
}

// TriggerCronJob manually creates a Job from a CronJob's jobTemplate, mirroring
// `kubectl create job --from=cronjob/<name>`.
func TriggerCronJob(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	name := rc.Param("name")
	if err := validateName(name); err != nil {
		return nil, err
	}
	ns := rc.Param("namespace")
	if err := validateNamespace(ns); err != nil {
		return nil, err
	}

	cj, err := s.clientset.BatchV1().CronJobs(ns).Get(rc.Ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, apiErr(err)
	}
	job := jobFromCronJob(cj)
	created, err := s.clientset.BatchV1().Jobs(ns).Create(rc.Ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, apiErr(err)
	}
	return map[string]any{"ok": true, "job": created.Name}, nil
}

// manualCronJobTimestampAnnotation is the annotation kubectl stamps on a Job
// created manually from a CronJob.
const manualCronJobTimestampAnnotation = "cronjob.kubernetes.io/instantiate"

// jobFromCronJob builds a Job from a CronJob's jobTemplate, carrying the
// template's labels/annotations and an owner reference back to the CronJob — the
// same shape kubectl produces for `create job --from=cronjob`.
func jobFromCronJob(cj *batchv1.CronJob) *batchv1.Job {
	annotations := map[string]string{manualCronJobTimestampAnnotation: "manual"}
	for k, v := range cj.Spec.JobTemplate.Annotations {
		annotations[k] = v
	}
	labels := map[string]string{}
	for k, v := range cj.Spec.JobTemplate.Labels {
		labels[k] = v
	}
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        manualJobName(cj.Name),
			Namespace:   cj.Namespace,
			Labels:      labels,
			Annotations: annotations,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cj, batchv1.SchemeGroupVersion.WithKind("CronJob")),
			},
		},
		Spec: *cj.Spec.JobTemplate.Spec.DeepCopy(),
	}
}

// manualJobName derives a unique-enough Job name from a CronJob name, matching
// kubectl's "<cronjob>-manual-<suffix>" convention and the 63-char limit.
func manualJobName(cronJob string) string {
	suffix := "-manual-" + strconv.FormatInt(time.Now().Unix(), 10)
	const maxNameLen = 63
	if budget := maxNameLen - len(suffix); len(cronJob) > budget {
		cronJob = cronJob[:budget]
	}
	return cronJob + suffix
}

// rfc1123Subdomain/rfc1123Label bound resource names and namespaces to the formats
// the apiserver accepts, which also rejects the metacharacters (`=`, `,`, whitespace)
// that would otherwise let a crafted name broaden a field selector.
var (
	rfc1123Subdomain = regexp.MustCompile(`^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$`)
	rfc1123Label     = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
)

func validateName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("%w: name is required", plugin.ErrInvalidInput)
	}
	if len(name) > 253 || !rfc1123Subdomain.MatchString(name) {
		return fmt.Errorf("%w: invalid resource name %q", plugin.ErrInvalidInput, name)
	}
	return nil
}

func validateNamespace(ns string) error {
	ns = strings.TrimSpace(ns)
	if ns == "" {
		return fmt.Errorf("%w: namespace is required", plugin.ErrInvalidInput)
	}
	if len(ns) > 63 || !rfc1123Label.MatchString(ns) {
		return fmt.Errorf("%w: invalid namespace %q", plugin.ErrInvalidInput, ns)
	}
	return nil
}
