package kubernetes

import (
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func pod(name string, owner *metav1.OwnerReference, mirror bool) *corev1.Pod {
	p := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}}
	if owner != nil {
		p.OwnerReferences = []metav1.OwnerReference{*owner}
	}
	if mirror {
		p.Annotations = map[string]string{corev1.MirrorPodAnnotationKey: "x"}
	}
	return p
}

func controller(kind string) *metav1.OwnerReference {
	ctrl := true
	return &metav1.OwnerReference{Kind: kind, Name: "owner", Controller: &ctrl}
}

func TestEvictablePod(t *testing.T) {
	cases := []struct {
		name  string
		pod   *corev1.Pod
		force bool
		want  bool
	}{
		{"replicaset-managed", pod("a", controller("ReplicaSet"), false), false, true},
		{"daemonset-skipped", pod("b", controller("DaemonSet"), false), false, false},
		{"daemonset-skipped-even-with-force", pod("b", controller("DaemonSet"), false), true, false},
		{"mirror-skipped", pod("c", controller("Node"), true), true, false},
		{"bare-skipped-without-force", pod("d", nil, false), false, false},
		{"bare-evicted-with-force", pod("e", nil, false), true, true},
		{"nil-pod", nil, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := evictablePod(tc.pod, tc.force); got != tc.want {
				t.Fatalf("evictablePod(%s, force=%v) = %v, want %v", tc.name, tc.force, got, tc.want)
			}
		})
	}
}

func deploymentWithRevision(rev string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "web",
			Namespace:   "default",
			UID:         "dep-uid",
			Annotations: map[string]string{revisionAnnotation: rev},
		},
	}
}

func replicaSet(name, rev string, ownedBy *appsv1.Deployment) appsv1.ReplicaSet {
	rs := appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   "default",
			Annotations: map[string]string{revisionAnnotation: rev},
		},
	}
	if ownedBy != nil {
		ctrl := true
		rs.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: "apps/v1", Kind: "Deployment", Name: ownedBy.Name, UID: ownedBy.UID, Controller: &ctrl,
		}}
	}
	return rs
}

func TestPreviousReplicaSet(t *testing.T) {
	dep := deploymentWithRevision("3")
	other := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "other", UID: "other-uid"}}

	rsList := []appsv1.ReplicaSet{
		replicaSet("web-1", "1", dep),
		replicaSet("web-2", "2", dep),     // expected previous (highest < 3)
		replicaSet("web-3", "3", dep),     // current revision, excluded
		replicaSet("foreign", "9", other), // not owned, excluded
	}

	prev, err := previousReplicaSet(dep, rsList)
	if err != nil {
		t.Fatalf("previousReplicaSet: %v", err)
	}
	if prev.Name != "web-2" {
		t.Fatalf("previous = %q, want web-2", prev.Name)
	}
}

func TestPreviousReplicaSetNoPrior(t *testing.T) {
	dep := deploymentWithRevision("1")
	rsList := []appsv1.ReplicaSet{replicaSet("web-1", "1", dep)}
	if _, err := previousReplicaSet(dep, rsList); err == nil {
		t.Fatal("expected an error when there is no prior revision")
	}
}

func TestRolloutUndoTemplateStripsPodTemplateHash(t *testing.T) {
	tmpl := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
			"app":                                  "web",
			appsv1.DefaultDeploymentUniqueLabelKey: "abc123",
		}},
	}
	out := rolloutUndoTemplate(tmpl)
	if _, ok := out.Labels[appsv1.DefaultDeploymentUniqueLabelKey]; ok {
		t.Fatal("pod-template-hash label must be stripped")
	}
	if out.Labels["app"] != "web" {
		t.Fatal("other labels must be preserved")
	}
	// Original must be untouched (DeepCopy).
	if _, ok := tmpl.Labels[appsv1.DefaultDeploymentUniqueLabelKey]; !ok {
		t.Fatal("rolloutUndoTemplate must not mutate its input")
	}
}

func TestJobFromCronJob(t *testing.T) {
	cj := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: "report", Namespace: "ops", UID: "cj-uid"},
		Spec: batchv1.CronJobSpec{
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"team": "data"},
					Annotations: map[string]string{"note": "nightly"},
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: "busybox"}}},
					},
				},
			},
		},
	}

	job := jobFromCronJob(cj)
	if job.Namespace != "ops" {
		t.Fatalf("namespace = %q, want ops", job.Namespace)
	}
	if !strings.HasPrefix(job.Name, "report-manual-") {
		t.Fatalf("job name = %q, want report-manual-* prefix", job.Name)
	}
	if job.Labels["team"] != "data" {
		t.Fatal("job template labels must carry over")
	}
	if job.Annotations["note"] != "nightly" {
		t.Fatal("job template annotations must carry over")
	}
	if job.Annotations[manualCronJobTimestampAnnotation] == "" {
		t.Fatal("manual instantiate annotation must be set")
	}
	if len(job.OwnerReferences) != 1 || job.OwnerReferences[0].Kind != "CronJob" || job.OwnerReferences[0].UID != "cj-uid" {
		t.Fatalf("owner reference = %+v, want a controller ref to the CronJob", job.OwnerReferences)
	}
	if len(job.Spec.Template.Spec.Containers) != 1 || job.Spec.Template.Spec.Containers[0].Image != "busybox" {
		t.Fatal("job spec must be copied from the cronjob jobTemplate")
	}
}

func TestManualJobNameTruncates(t *testing.T) {
	long := strings.Repeat("a", 80)
	name := manualJobName(long)
	if len(name) > 63 {
		t.Fatalf("manual job name length = %d, want <= 63", len(name))
	}
	if !strings.Contains(name, "-manual-") {
		t.Fatalf("manual job name = %q, want a -manual- segment", name)
	}
}
