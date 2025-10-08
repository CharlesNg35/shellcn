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
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/api"
	"github.com/charlesng35/shellcn/internal/app"
	"github.com/charlesng35/shellcn/internal/app/maintenance"
	iauth "github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/cache"
	"github.com/charlesng35/shellcn/internal/database"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/services"
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

	cfg, err := loadApplicationConfig(configPath)
	if err != nil {
		return err
	}

	generated, err := app.ApplyRuntimeDefaults(cfg)
	if err != nil {
		return err
	}

	if err := app.ConfigureLogging(cfg.Server.LogLevel); err != nil {
		return fmt.Errorf("configure logging: %w", err)
	}
	defer logger.Sync() // best effort

	log := logger.WithModule("bootstrap")
	for key := range generated {
		log.Info("generated runtime secret", zap.String("key", key))
	}

	if err := ensureSecretsPresent(cfg); err != nil {
		return err
	}

	db, err := initialiseDatabase(cfg)
	if err != nil {
		return err
	}
	defer closeDatabase(db, log)

	dbStore := cache.NewDatabaseStore(db)

	var redisClient cache.Store
	if cfg.Cache.Redis.Enabled {
		client, redisErr := cache.NewRedisClient(cfg.Cache.RedisClientConfig())
		if redisErr != nil {
			log.Warn("redis unavailable; falling back to database-backed operations", zap.Error(redisErr))
		} else {
			redisClient = client
			log.Info("redis connected", zap.String("addr", cfg.Cache.Redis.Address))
		}
	}

	defer func() {
		if rc, ok := redisClient.(*cache.RedisClient); ok && rc != nil {
			_ = rc.Close()
		}
	}()

	jwtService, err := iauth.NewJWTService(cfg.Auth.JWTServiceConfig())
	if err != nil {
		return fmt.Errorf("initialise jwt service: %w", err)
	}

	sessionCfg := cfg.Auth.SessionServiceConfig()
	switch {
	case redisClient != nil:
		sessionCfg.Cache = iauth.NewRedisSessionCache(redisClient)
	case dbStore != nil:
		sessionCfg.Cache = iauth.NewDatabaseSessionCache(dbStore)
	}

	sessionSvc, err := iauth.NewSessionService(db, jwtService, sessionCfg)
	if err != nil {
		return fmt.Errorf("initialise session service: %w", err)
	}

	auditSvc, err := services.NewAuditService(db)
	if err != nil {
		return fmt.Errorf("initialise audit service: %w", err)
	}

	cleaner := maintenance.NewCleaner(db, sessionSvc, auditSvc)
	if err := cleaner.Start(); err != nil {
		return fmt.Errorf("start maintenance jobs: %w", err)
	}
	defer func() {
		stopCtx := cleaner.Stop()
		if err := cleaner.RunOnce(stopCtx); err != nil {
			log.Warn("maintenance shutdown cleanup failed", zap.Error(err))
		}
	}()

	var rateStore middleware.RateStore
	switch {
	case redisClient != nil:
		rateStore = middleware.NewRedisRateStore(redisClient)
	case dbStore != nil:
		rateStore = middleware.NewDatabaseRateStore(dbStore)
	}

	router, err := api.NewRouter(db, jwtService, cfg, sessionSvc, rateStore)
	if err != nil {
		return fmt.Errorf("build api router: %w", err)
	}

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

	cfg.Vault.EncryptionKey = strings.TrimSpace(cfg.Vault.EncryptionKey)
	keyLen := len(cfg.Vault.EncryptionKey)
	if keyLen != 16 && keyLen != 24 && keyLen != 32 {
		return fmt.Errorf("vault.encryption_key must be 16, 24, or 32 characters (current: %d)", keyLen)
	}

	return nil
}

func initialiseDatabase(cfg *app.Config) (*gorm.DB, error) {
	dbCfg := convertDatabaseConfig(cfg)
	db, err := database.Open(dbCfg)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := database.AutoMigrateAndSeed(db); err != nil {
		return nil, fmt.Errorf("auto-migrate database: %w", err)
	}

	log := logger.WithModule("database")
	log.Info("database connected", zap.String("driver", strings.ToLower(strings.TrimSpace(dbCfg.Driver))))

	return db, nil
}

func convertDatabaseConfig(cfg *app.Config) database.Config {
	dbCfg := database.Config{
		Driver: strings.ToLower(strings.TrimSpace(cfg.Database.Driver)),
		Path:   strings.TrimSpace(cfg.Database.Path),
		DSN:    strings.TrimSpace(cfg.Database.DSN),
	}

	switch dbCfg.Driver {
	case "", "sqlite":
		dbCfg.Driver = "sqlite"
	case "postgres", "postgresql":
		dbCfg.Driver = "postgres"
		dbCfg.Host = strings.TrimSpace(cfg.Database.Postgres.Host)
		dbCfg.Port = cfg.Database.Postgres.Port
		dbCfg.Name = strings.TrimSpace(cfg.Database.Postgres.Database)
		dbCfg.User = strings.TrimSpace(cfg.Database.Postgres.Username)
		dbCfg.Password = strings.TrimSpace(cfg.Database.Postgres.Password)
	case "mysql":
		dbCfg.Host = strings.TrimSpace(cfg.Database.MySQL.Host)
		dbCfg.Port = cfg.Database.MySQL.Port
		dbCfg.Name = strings.TrimSpace(cfg.Database.MySQL.Database)
		dbCfg.User = strings.TrimSpace(cfg.Database.MySQL.Username)
		dbCfg.Password = strings.TrimSpace(cfg.Database.MySQL.Password)
	default:
		// Leave driver as-is to surface unsupported driver error during open.
	}

	return dbCfg
}

func closeDatabase(db *gorm.DB, log *zap.Logger) {
	if db == nil {
		return
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Warn("failed to obtain underlying sql DB for closing", zap.Error(err))
		return
	}

	if err := sqlDB.Close(); err != nil {
		log.Warn("failed to close database", zap.Error(err))
	}
}
