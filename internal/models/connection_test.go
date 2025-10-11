package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err, "failed to open test database")

	err = db.AutoMigrate(&Connection{}, &ConnectionTarget{}, &ResourcePermission{})
	require.NoError(t, err, "failed to auto-migrate")

	return db
}

func TestConnection_BeforeDelete(t *testing.T) {
	db := setupTestDB(t)

	// Create a connection with owner
	connection := Connection{
		Name:        "Test Connection",
		Description: "Test description",
		ProtocolID:  "ssh",
		OwnerUserID: "user-123",
	}
	require.NoError(t, db.Create(&connection).Error, "failed to create connection")

	// Create associated connection targets
	targets := []ConnectionTarget{
		{
			ConnectionID: connection.ID,
			Host:         "192.168.1.1",
			Port:         22,
			Ordering:     1,
		},
		{
			ConnectionID: connection.ID,
			Host:         "192.168.1.2",
			Port:         22,
			Ordering:     2,
		},
	}
	for i := range targets {
		require.NoError(t, db.Create(&targets[i]).Error, "failed to create target %d", i+1)
	}

	// Create associated resource permissions (user and team grants)
	userGrant := ResourcePermission{
		ResourceID:    connection.ID,
		ResourceType:  "connection",
		PrincipalType: "user",
		PrincipalID:   "user-456",
		PermissionID:  "connection.view",
	}
	teamGrant := ResourcePermission{
		ResourceID:    connection.ID,
		ResourceType:  "connection",
		PrincipalType: "team",
		PrincipalID:   "team-789",
		PermissionID:  "connection.launch",
	}
	require.NoError(t, db.Create(&userGrant).Error, "failed to create user grant")
	require.NoError(t, db.Create(&teamGrant).Error, "failed to create team grant")

	// Verify records exist before deletion
	var targetCountBefore int64
	require.NoError(t, db.Model(&ConnectionTarget{}).
		Where("connection_id = ?", connection.ID).
		Count(&targetCountBefore).Error)
	assert.Equal(t, int64(2), targetCountBefore, "expected 2 targets before deletion")

	var grantCountBefore int64
	require.NoError(t, db.Model(&ResourcePermission{}).
		Where("resource_type = ? AND resource_id = ?", "connection", connection.ID).
		Count(&grantCountBefore).Error)
	assert.Equal(t, int64(2), grantCountBefore, "expected 2 grants before deletion")

	// Delete the connection (should trigger BeforeDelete hook)
	require.NoError(t, db.Delete(&connection).Error, "failed to delete connection")

	// Verify connection targets were cascade deleted
	var targetCountAfter int64
	require.NoError(t, db.Model(&ConnectionTarget{}).
		Where("connection_id = ?", connection.ID).
		Count(&targetCountAfter).Error)
	assert.Equal(t, int64(0), targetCountAfter, "expected 0 targets after cascade delete")

	// Verify resource permissions were cascade deleted
	var grantCountAfter int64
	require.NoError(t, db.Model(&ResourcePermission{}).
		Where("resource_type = ? AND resource_id = ?", "connection", connection.ID).
		Count(&grantCountAfter).Error)
	assert.Equal(t, int64(0), grantCountAfter, "expected 0 grants after cascade delete")
}

func TestConnection_BeforeDeleteWithMultipleResourceTypes(t *testing.T) {
	db := setupTestDB(t)

	// Create multiple connections
	conn1 := Connection{
		Name:        "Connection 1",
		ProtocolID:  "ssh",
		OwnerUserID: "user-1",
	}
	conn2 := Connection{
		Name:        "Connection 2",
		ProtocolID:  "rdp",
		OwnerUserID: "user-1",
	}
	require.NoError(t, db.Create(&conn1).Error)
	require.NoError(t, db.Create(&conn2).Error)

	// Create resource permissions for both connections
	grants := []ResourcePermission{
		{
			ResourceID:    conn1.ID,
			ResourceType:  "connection",
			PrincipalType: "user",
			PrincipalID:   "user-shared",
			PermissionID:  "connection.view",
		},
		{
			ResourceID:    conn2.ID,
			ResourceType:  "connection",
			PrincipalType: "user",
			PrincipalID:   "user-shared",
			PermissionID:  "connection.view",
		},
		{
			ResourceID:    conn1.ID,
			ResourceType:  "connection",
			PrincipalType: "team",
			PrincipalID:   "team-1",
			PermissionID:  "protocol:ssh.connect",
		},
	}
	for i := range grants {
		require.NoError(t, db.Create(&grants[i]).Error)
	}

	// Verify 3 grants exist before deletion
	var totalGrants int64
	require.NoError(t, db.Model(&ResourcePermission{}).Count(&totalGrants).Error)
	assert.Equal(t, int64(3), totalGrants, "expected 3 total grants before deletion")

	// Delete conn1 (should only delete conn1's grants)
	require.NoError(t, db.Delete(&conn1).Error)

	// Verify conn1's grants were deleted
	var conn1Grants int64
	require.NoError(t, db.Model(&ResourcePermission{}).
		Where("resource_type = ? AND resource_id = ?", "connection", conn1.ID).
		Count(&conn1Grants).Error)
	assert.Equal(t, int64(0), conn1Grants, "expected 0 grants for conn1 after deletion")

	// Verify conn2's grants still exist
	var conn2Grants int64
	require.NoError(t, db.Model(&ResourcePermission{}).
		Where("resource_type = ? AND resource_id = ?", "connection", conn2.ID).
		Count(&conn2Grants).Error)
	assert.Equal(t, int64(1), conn2Grants, "expected 1 grant for conn2 to remain")

	// Verify total grants count
	require.NoError(t, db.Model(&ResourcePermission{}).Count(&totalGrants).Error)
	assert.Equal(t, int64(1), totalGrants, "expected 1 total grant remaining")
}

func TestConnection_BeforeDeleteWithNoAssociations(t *testing.T) {
	db := setupTestDB(t)

	// Create a connection without any targets or grants
	connection := Connection{
		Name:        "Standalone Connection",
		ProtocolID:  "kubernetes",
		OwnerUserID: "user-solo",
	}
	require.NoError(t, db.Create(&connection).Error)

	// Verify no targets or grants exist
	var targetCount, grantCount int64
	require.NoError(t, db.Model(&ConnectionTarget{}).
		Where("connection_id = ?", connection.ID).
		Count(&targetCount).Error)
	require.NoError(t, db.Model(&ResourcePermission{}).
		Where("resource_type = ? AND resource_id = ?", "connection", connection.ID).
		Count(&grantCount).Error)
	assert.Equal(t, int64(0), targetCount, "expected 0 targets")
	assert.Equal(t, int64(0), grantCount, "expected 0 grants")

	// Delete should succeed without errors even with no associations
	require.NoError(t, db.Delete(&connection).Error, "deletion should succeed with no associations")

	// Verify connection was deleted
	var exists int64
	require.NoError(t, db.Model(&Connection{}).
		Where("id = ?", connection.ID).
		Count(&exists).Error)
	assert.Equal(t, int64(0), exists, "connection should be deleted")
}
