package main

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/config"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/store"
)

func TestBootstrapAdminUsesConfig(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemory()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	if err := bootstrapAdmin(ctx, logger, st, config.BootstrapConfig{
		AdminUsername: "root",
		AdminPassword: "initial-secret",
	}); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	user, err := st.Users.GetByUsername(ctx, "root")
	if err != nil {
		t.Fatalf("admin user: %v", err)
	}
	if !user.Protected || !user.HasRole(models.RoleAdmin) {
		t.Fatalf("admin flags: %+v", user)
	}
	if _, err := auth.NewLocalAuthenticator(st.Users).Authenticate(ctx, "root", "initial-secret"); err != nil {
		t.Fatalf("configured admin password should authenticate: %v", err)
	}
}

func TestBootstrapAdminRejectsWeakConfiguredPassword(t *testing.T) {
	err := bootstrapAdmin(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), store.NewMemory(), config.BootstrapConfig{
		AdminUsername: "admin",
		AdminPassword: "short",
	})
	if err == nil {
		t.Fatal("weak configured bootstrap password should fail")
	}
}
