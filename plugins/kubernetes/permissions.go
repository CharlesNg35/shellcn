package kubernetes

import (
	"context"
	"sync"

	authzv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// permissionVerbs surface under a nested "can" map that EnabledWhen rules read by
// dotted path (can.patch …); an absent value fails open.
var permissionVerbs = []string{"update", "patch", "delete"}

// accessReview probes the user's RBAC for one object concurrently.
func (s *Session) accessReview(ctx context.Context, k kind, namespace, name string) map[string]any {
	can := make(map[string]any, len(permissionVerbs))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, verb := range permissionVerbs {
		wg.Add(1)
		go func(verb string) {
			defer wg.Done()
			allowed := s.canI(ctx, k, namespace, name, verb)
			mu.Lock()
			can[verb] = allowed
			mu.Unlock()
		}(verb)
	}
	wg.Wait()
	return can
}

func (s *Session) canI(ctx context.Context, k kind, namespace, name, verb string) bool {
	review := &authzv1.SelfSubjectAccessReview{
		Spec: authzv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authzv1.ResourceAttributes{
				Group:     k.gvr.Group,
				Resource:  k.gvr.Resource,
				Namespace: namespace,
				Name:      name,
				Verb:      verb,
			},
		},
	}
	res, err := s.clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, review, metav1.CreateOptions{})
	if err != nil {
		return true // fail open: never hide an action on an SSAR error
	}
	return res.Status.Allowed
}
