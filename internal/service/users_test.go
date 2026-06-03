package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/charlesng35/shellcn/internal/service"
	"github.com/charlesng35/shellcn/internal/store"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestUserServiceCreateValidatesPassword(t *testing.T) {
	svc := service.NewUserService(store.NewMemory().Users)
	if _, err := svc.Create(context.Background(), service.NewUserInput{
		Username: "alice",
		Password: "short",
	}); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("weak password: want invalid input, got %v", err)
	}
}
