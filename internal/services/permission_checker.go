package services

import "context"

// PermissionChecker abstracts permission evaluation for services.
type PermissionChecker interface {
	Check(ctx context.Context, userID, permissionID string) (bool, error)
}
