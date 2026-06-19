package kubernetes

import (
	"context"
	"sync"

	authzv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// permissionKeys maps the RBAC verbs probed for an object to the flat record keys
// the manifest's EnabledWhen rules read. An absent key fails open (action shown),
// so RBAC errors never hide an action the user might be able to perform.
var permissionKeys = map[string]string{
	"update": "canUpdate",
	"patch":  "canPatch",
	"delete": "canDelete",
}

// accessReview probes the user's RBAC for one object concurrently and returns the
// flat canUpdate/canPatch/canDelete booleans for the detail view to gate actions.
func (s *Session) accessReview(ctx context.Context, k kind, namespace, name string) map[string]any {
	out := make(map[string]any, len(permissionKeys))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for verb, key := range permissionKeys {
		wg.Add(1)
		go func(verb, key string) {
			defer wg.Done()
			allowed := s.canI(ctx, k, namespace, name, verb)
			mu.Lock()
			out[key] = allowed
			mu.Unlock()
		}(verb, key)
	}
	wg.Wait()
	return out
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
