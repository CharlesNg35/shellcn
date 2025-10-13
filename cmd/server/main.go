package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/charlesng35/shellcn/internal/app"
	"github.com/charlesng35/shellcn/internal/database"
	"github.com/charlesng35/shellcn/pkg/logger"
)

const shutdownTimeout = 15 * time.Second

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	if err := run(ctx, os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("shellcn-server", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	var configPath string
	fs.StringVar(&configPath, "config", "", "Path to configuration directory or file")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Load configuration from disk/environment.
	cfg, err := loadApplicationConfig(configPath)
	if err != nil {
		return err
	}

	if strings.TrimSpace(cfg.Vault.EncryptionKey) == "" {
		if stored, err := loadVaultKeyFromSystemSettings(ctx, cfg); err == nil && strings.TrimSpace(stored) != "" {
			cfg.Vault.EncryptionKey = stored
		}
	}

	// Fill in runtime defaults (JWT secrets, vault key, etc.).
	generated, err := app.ApplyRuntimeDefaults(cfg)
	if err != nil {
		return err
	}

	// Configure structured logging.
	if err := app.ConfigureLogging(cfg.Server.LogLevel); err != nil {
		return fmt.Errorf("configure logging: %w", err)
	}
	defer logger.Sync() // best effort

	log := logger.WithModule("bootstrap")
	for key := range generated {
		log.Info("generated runtime secret", zap.String("key", key))
	}

	// Ensure required secrets are available before bootstrapping services.
	if err := ensureSecretsPresent(cfg); err != nil {
		return err
	}

	// Spin up databases, caches, background jobs, and the HTTP router.
	stack, err := bootstrapRuntime(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer func() {
		if stack != nil {
			stack.Shutdown(context.Background(), log)
		}
	}()

	router := stack.Router

	// Launch the HTTP server and block until shutdown.
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: router,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Info("server listening", zap.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case err := <-serverErr:
		if err != nil {
			return fmt.Errorf("server error: %w", err)
		}
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	stack.Shutdown(shutdownCtx, log)
	stack = nil

	if err, ok := <-serverErr; ok && err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	log.Info("server stopped gracefully")
	return nil
}

func loadApplicationConfig(path string) (*app.Config, error) {
	switch {
	case strings.TrimSpace(path) == "":
		return app.LoadConfig()
	default:
		info, err := os.Stat(path)
		if err == nil {
			if info.IsDir() {
				return app.LoadConfig(path)
			}
			return app.LoadConfig(filepath.Dir(path))
		}
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("config path %q does not exist", path)
		}
		return nil, fmt.Errorf("stat config path: %w", err)
	}
}

func ensureSecretsPresent(cfg *app.Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}

	cfg.Auth.JWT.Secret = strings.TrimSpace(cfg.Auth.JWT.Secret)
	if cfg.Auth.JWT.Secret == "" {
		return errors.New("auth.jwt.secret must be configured")
	}

	jwtBytes, err := app.KeyByteLength(cfg.Auth.JWT.Secret)
	if err != nil {
		return fmt.Errorf("auth.jwt.secret: %w", err)
	}
	if jwtBytes < 32 {
		return fmt.Errorf("auth.jwt.secret must decode to at least 32 bytes (current: %d)", jwtBytes)
	}

	cfg.Vault.EncryptionKey = strings.TrimSpace(cfg.Vault.EncryptionKey)
	if cfg.Vault.EncryptionKey == "" {
		return errors.New("vault.encryption_key must be configured")
	}
	length, err := app.KeyByteLength(cfg.Vault.EncryptionKey)
	if err != nil {
		return fmt.Errorf("vault.encryption_key: %w", err)
	}
	if length != 32 {
		return fmt.Errorf("vault.encryption_key must decode to exactly 32 bytes (current: %d)", length)
	}

	return nil
}

func loadVaultKeyFromSystemSettings(ctx context.Context, cfg *app.Config) (string, error) {
	preDB, err := database.Open(convertDatabaseConfig(cfg))
	if err != nil {
		return "", err
	}

	sqlDB, err := preDB.DB()
	if err == nil {
		defer sqlDB.Close()
	}

	return database.GetSystemSetting(ctx, preDB, database.VaultEncryptionKeySetting)
}
