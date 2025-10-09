package services

import "context"

type mockPermissionChecker struct {
	grants map[string]bool
	err    error
}

func (m *mockPermissionChecker) Check(_ context.Context, _ string, permissionID string) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	if m.grants == nil {
		return false, nil
	}
	return m.grants[permissionID], nil
}
