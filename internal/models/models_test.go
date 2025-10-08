package models

import "testing"

func TestUserBeforeCreateGeneratesID(t *testing.T) {
	var user User
	if err := user.BeforeCreate(nil); err != nil {
		t.Fatalf("before create: %v", err)
	}
	if user.ID == "" {
		t.Fatal("expected user ID to be generated")
	}
}

func TestOrganizationBeforeCreateGeneratesID(t *testing.T) {
	var org Organization
	if err := org.BeforeCreate(nil); err != nil {
		t.Fatalf("before create: %v", err)
	}
	if org.ID == "" {
		t.Fatal("expected organization ID to be generated")
	}
}

func TestTeamBeforeCreateGeneratesID(t *testing.T) {
	var team Team
	if err := team.BeforeCreate(nil); err != nil {
		t.Fatalf("before create: %v", err)
	}
	if team.ID == "" {
		t.Fatal("expected team ID to be generated")
	}
}

func TestRoleBeforeCreateGeneratesIDWhenEmpty(t *testing.T) {
	var role Role
	if err := role.BeforeCreate(nil); err != nil {
		t.Fatalf("before create: %v", err)
	}
	if role.ID == "" {
		t.Fatal("expected role ID to be generated")
	}
}

func TestSessionBeforeCreateGeneratesID(t *testing.T) {
	var session Session
	if err := session.BeforeCreate(nil); err != nil {
		t.Fatalf("before create: %v", err)
	}
	if session.ID == "" {
		t.Fatal("expected session ID to be generated")
	}
}

func TestAuditLogBeforeCreateGeneratesID(t *testing.T) {
	var entry AuditLog
	if err := entry.BeforeCreate(nil); err != nil {
		t.Fatalf("before create: %v", err)
	}
	if entry.ID == "" {
		t.Fatal("expected audit log ID to be generated")
	}
}

func TestMFASecretBeforeCreateGeneratesID(t *testing.T) {
	var secret MFASecret
	if err := secret.BeforeCreate(nil); err != nil {
		t.Fatalf("before create: %v", err)
	}
	if secret.ID == "" {
		t.Fatal("expected MFA secret ID to be generated")
	}
}

func TestPasswordResetTokenBeforeCreateGeneratesID(t *testing.T) {
	var token PasswordResetToken
	if err := token.BeforeCreate(nil); err != nil {
		t.Fatalf("before create: %v", err)
	}
	if token.ID == "" {
		t.Fatal("expected password reset token ID to be generated")
	}
}

func TestAuthProviderBeforeCreateGeneratesID(t *testing.T) {
	var provider AuthProvider
	if err := provider.BeforeCreate(nil); err != nil {
		t.Fatalf("before create: %v", err)
	}
	if provider.ID == "" {
		t.Fatal("expected auth provider ID to be generated")
	}
}
