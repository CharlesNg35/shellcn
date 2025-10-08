package permissions

import (
	"fmt"
)

var (
	// ErrUnknownPermission indicates a permission lookup failed because it has not been registered.
	ErrUnknownPermission = fmt.Errorf("permission: unknown permission")
	// ErrCircularDependency signals that a dependency graph contains a cycle.
	ErrCircularDependency = fmt.Errorf("permission: circular dependency detected")
)

// ResolveDependencies returns the full dependency chain for the specified permission.
func ResolveDependencies(permissionID string) ([]string, error) {
	perms := GetAll()

	root, ok := perms[permissionID]
	if !ok {
		return nil, fmt.Errorf("%w %q", ErrUnknownPermission, permissionID)
	}

	visited := make(map[string]bool, len(perms))
	recStack := make(map[string]bool, len(perms))
	var resolved []string

	var walk func(string) error
	walk = func(current string) error {
		perm, ok := perms[current]
		if !ok {
			return fmt.Errorf("%w %q", ErrUnknownPermission, current)
		}
		if recStack[current] {
			return fmt.Errorf("%w at %s", ErrCircularDependency, current)
		}
		if visited[current] {
			return nil
		}

		recStack[current] = true
		for _, dep := range perm.DependsOn {
			if err := walk(dep); err != nil {
				return err
			}
		}
		recStack[current] = false
		visited[current] = true

		if current != permissionID {
			resolved = append(resolved, current)
		}

		return nil
	}

	for _, dep := range root.DependsOn {
		if err := walk(dep); err != nil {
			return nil, err
		}
	}

	return resolved, nil
}
