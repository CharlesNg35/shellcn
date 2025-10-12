package permissions

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
)

func TestRegisterPreventsDuplicates(t *testing.T) {
	id := "test.unique.permission"
	err := Register(&Permission{
		ID:     id,
		Module: "test",
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		removePermission(id)
	})

	err = Register(&Permission{
		ID:     id,
		Module: "test",
	})
	require.Error(t, err)
}

func TestResolveDependenciesReturnsTransitiveClosure(t *testing.T) {
	ids := []string{"perm.base", "perm.mid", "perm.top"}
	require.NoError(t, Register(&Permission{ID: ids[0], Module: "test"}))
	require.NoError(t, Register(&Permission{ID: ids[1], Module: "test", DependsOn: []string{ids[0]}}))
	require.NoError(t, Register(&Permission{ID: ids[2], Module: "test", DependsOn: []string{ids[1]}}))
	t.Cleanup(func() {
		for _, id := range ids {
			removePermission(id)
		}
	})

	deps, err := ResolveDependencies(ids[2])
	require.NoError(t, err)
	require.ElementsMatch(t, []string{ids[0], ids[1]}, deps)
}

func TestResolveDependenciesDetectsCycles(t *testing.T) {
	const (
		first  = "perm.cycle.first"
		second = "perm.cycle.second"
	)
	require.NoError(t, Register(&Permission{ID: first, Module: "test", DependsOn: []string{second}}))
	require.NoError(t, Register(&Permission{ID: second, Module: "test", DependsOn: []string{first}}))
	t.Cleanup(func() {
		removePermission(first)
		removePermission(second)
	})

	_, err := ResolveDependencies(first)
	require.Error(t, err)
	require.ErrorContains(t, err, ErrCircularDependency.Error())
}

func TestCheckerRootBypassesAllChecks(t *testing.T) {
	db := setupPermissionTestDB(t)

	rootUser := &models.User{
		Username: "root",
		Email:    "root@example.com",
		Password: "hashed",
		IsRoot:   true,
	}
	require.NoError(t, db.Create(rootUser).Error)

	checker, err := NewChecker(db)
	require.NoError(t, err)

	ok, err := checker.Check(context.Background(), rootUser.ID, "non.existent.permission")
	require.NoError(t, err)
	require.True(t, ok)
}

func TestCheckerDependencyEnforcement(t *testing.T) {
	db := setupPermissionTestDB(t)

	role := &models.Role{
		BaseModel: models.BaseModel{ID: "role.tester"},
		Name:      "Tester",
	}
	require.NoError(t, db.Create(role).Error)

	user := &models.User{
		Username: "tester",
		Email:    "tester@example.com",
		Password: "secret",
	}
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Model(user).Association("Roles").Append(role))

	var deletePerm models.Permission
	require.NoError(t, db.First(&deletePerm, "id = ?", "user.delete").Error)
	require.NoError(t, db.Model(role).Association("Permissions").Replace(&deletePerm))

	checker, err := NewChecker(db)
	require.NoError(t, err)

	ok, err := checker.Check(context.Background(), user.ID, "user.delete")
	require.NoError(t, err)
	require.False(t, ok)

	var viewPerm, editPerm models.Permission
	require.NoError(t, db.First(&viewPerm, "id = ?", "user.view").Error)
	require.NoError(t, db.First(&editPerm, "id = ?", "user.edit").Error)
	require.NoError(t, db.Model(role).Association("Permissions").Replace(&viewPerm, &editPerm, &deletePerm))

	ok, err = checker.Check(context.Background(), user.ID, "user.delete")
	require.NoError(t, err)
	require.True(t, ok)
}

func TestCheckerIncludesImpliedPermissions(t *testing.T) {
	const (
		child  = "perm.child"
		parent = "perm.parent"
	)

	require.NoError(t, Register(&Permission{ID: child, Module: "test"}))
	require.NoError(t, Register(&Permission{
		ID:        parent,
		Module:    "test",
		Implies:   []string{child},
		DependsOn: nil,
	}))
	t.Cleanup(func() {
		removePermission(child)
		removePermission(parent)
	})

	db := setupPermissionTestDB(t)
	require.NoError(t, Sync(context.Background(), db))

	role := &models.Role{
		BaseModel: models.BaseModel{ID: "role.implied"},
		Name:      "Implied Tester",
	}
	require.NoError(t, db.Create(role).Error)

	user := &models.User{
		Username: "implied",
		Email:    "implied@example.com",
		Password: "secret",
	}
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Model(user).Association("Roles").Append(role))

	var parentPerm models.Permission
	require.NoError(t, db.First(&parentPerm, "id = ?", parent).Error)
	require.NoError(t, db.Model(role).Association("Permissions").Append(&parentPerm))

	checker, err := NewChecker(db)
	require.NoError(t, err)

	ok, err := checker.Check(context.Background(), user.ID, child)
	require.NoError(t, err)
	require.True(t, ok)

	perms, err := checker.GetUserPermissions(context.Background(), user.ID)
	require.NoError(t, err)
	require.Contains(t, perms, child)
	require.Contains(t, perms, parent)
}

func TestCheckerRequiresEmailVerification(t *testing.T) {
	db := setupPermissionTestDB(t)

	require.NoError(t, db.Create(&models.AuthProvider{
		Type:                     "local",
		Name:                     "Local",
		Enabled:                  true,
		RequireEmailVerification: true,
	}).Error)

	role := &models.Role{
		BaseModel: models.BaseModel{ID: "role.verification"},
		Name:      "Needs Verification",
	}
	require.NoError(t, db.Create(role).Error)

	var viewPerm models.Permission
	require.NoError(t, db.First(&viewPerm, "id = ?", "user.view").Error)
	require.NoError(t, db.Model(role).Association("Permissions").Append(&viewPerm))

	user := &models.User{
		Username: "verify-me",
		Email:    "verify@example.com",
		Password: "secret",
	}
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Model(user).Association("Roles").Append(role))

	require.NoError(t, db.Create(&models.EmailVerification{
		UserID:    user.ID,
		TokenHash: "pending",
		ExpiresAt: time.Now().Add(time.Hour),
	}).Error)

	checker, err := NewChecker(db)
	require.NoError(t, err)

	perms, err := checker.GetUserPermissions(context.Background(), user.ID)
	require.NoError(t, err)
	require.Empty(t, perms)

	now := time.Now()
	require.NoError(t, db.Model(&models.EmailVerification{}).
		Where("user_id = ?", user.ID).
		Update("verified_at", now).Error)

	perms, err = checker.GetUserPermissions(context.Background(), user.ID)
	require.NoError(t, err)
	require.Contains(t, perms, "user.view")
}

func TestCheckerCheckResourceHonoursGrants(t *testing.T) {
	db := setupPermissionTestDB(t)

	user := &models.User{
		Username: "resource-user",
		Email:    "resource@example.com",
		Password: "secret",
	}
	require.NoError(t, db.Create(user).Error)

	resourceID := "conn-resource-1"

	checker, err := NewChecker(db)
	require.NoError(t, err)

	ok, err := checker.CheckResource(context.Background(), user.ID, "connection", resourceID, "connection.view")
	require.NoError(t, err)
	require.False(t, ok)

	grant := models.ResourcePermission{
		ResourceID:    resourceID,
		ResourceType:  "connection",
		PrincipalType: principalTypeUser,
		PrincipalID:   user.ID,
		PermissionID:  "connection.view",
	}
	require.NoError(t, db.Create(&grant).Error)

	ok, err = checker.CheckResource(context.Background(), user.ID, "connection", resourceID, "connection.view")
	require.NoError(t, err)
	require.True(t, ok)
}

func TestCheckerCheckResourceEnforcesDependencies(t *testing.T) {
	db := setupPermissionTestDB(t)

	user := &models.User{
		Username: "resource-deps",
		Email:    "resource-deps@example.com",
		Password: "secret",
	}
	require.NoError(t, db.Create(user).Error)

	resourceID := "conn-resource-2"

	checker, err := NewChecker(db)
	require.NoError(t, err)

	launchGrant := models.ResourcePermission{
		ResourceID:    resourceID,
		ResourceType:  "connection",
		PrincipalType: principalTypeUser,
		PrincipalID:   user.ID,
		PermissionID:  "connection.launch",
	}
	require.NoError(t, db.Create(&launchGrant).Error)

	ok, err := checker.CheckResource(context.Background(), user.ID, "connection", resourceID, "connection.launch")
	require.NoError(t, err)
	require.False(t, ok, "should require connection.view dependency")

	viewGrant := models.ResourcePermission{
		ResourceID:    resourceID,
		ResourceType:  "connection",
		PrincipalType: principalTypeUser,
		PrincipalID:   user.ID,
		PermissionID:  "connection.view",
	}
	require.NoError(t, db.Create(&viewGrant).Error)

	ok, err = checker.CheckResource(context.Background(), user.ID, "connection", resourceID, "connection.launch")
	require.NoError(t, err)
	require.True(t, ok)
}

func TestCheckerCheckResourceHonoursExpiry(t *testing.T) {
	db := setupPermissionTestDB(t)

	user := &models.User{
		Username: "resource-expiry",
		Email:    "resource-expiry@example.com",
		Password: "secret",
	}
	require.NoError(t, db.Create(user).Error)

	resourceID := "conn-resource-3"

	past := time.Now().Add(-1 * time.Hour)
	grant := models.ResourcePermission{
		ResourceID:    resourceID,
		ResourceType:  "connection",
		PrincipalType: principalTypeUser,
		PrincipalID:   user.ID,
		PermissionID:  "connection.view",
		ExpiresAt:     &past,
	}
	require.NoError(t, db.Create(&grant).Error)

	checker, err := NewChecker(db)
	require.NoError(t, err)

	ok, err := checker.CheckResource(context.Background(), user.ID, "connection", resourceID, "connection.view")
	require.NoError(t, err)
	require.False(t, ok)
}

func setupPermissionTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Role{},
		&models.Team{},
		&models.Permission{},
		&models.ResourcePermission{},
		&models.AuthProvider{},
		&models.EmailVerification{},
	))
	require.NoError(t, Sync(context.Background(), db))

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	return db
}

func removePermission(id string) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	delete(globalRegistry.permissions, id)
}

func TestCheckerIncludesTeamRoles(t *testing.T) {
	db := setupPermissionTestDB(t)

	role := &models.Role{
		BaseModel: models.BaseModel{ID: "role.team"},
		Name:      "Team Role",
	}
	require.NoError(t, db.Create(role).Error)

	var viewPerm models.Permission
	require.NoError(t, db.First(&viewPerm, "id = ?", "user.view").Error)
	require.NoError(t, db.Model(role).Association("Permissions").Append(&viewPerm))

	team := &models.Team{
		Name: "Infra",
	}
	require.NoError(t, db.Create(team).Error)
	require.NoError(t, db.Model(team).Association("Roles").Append(role))

	user := &models.User{
		Username: "team-member",
		Email:    "team@example.com",
		Password: "secret",
	}
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Model(team).Association("Users").Append(user))

	checker, err := NewChecker(db)
	require.NoError(t, err)

	ok, err := checker.Check(context.Background(), user.ID, "user.view")
	require.NoError(t, err)
	require.True(t, ok)

	perms, err := checker.GetUserPermissions(context.Background(), user.ID)
	require.NoError(t, err)
	require.Contains(t, perms, "user.view")
}
