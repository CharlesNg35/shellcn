package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/api"
	"github.com/charlesng35/shellcn/internal/app"
	"github.com/charlesng35/shellcn/internal/app/maintenance"
	iauth "github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/cache"
	"github.com/charlesng35/shellcn/internal/database"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/services"
	"github.com/charlesng35/shellcn/internal/vault"
	"github.com/charlesng35/shellcn/pkg/logger"
)

// runtimeStack bundles long-lived services used by the HTTP server.
type runtimeStack struct {
	DB         *gorm.DB
	Redis      cache.Store
	SessionSvc *iauth.SessionService
	AuditSvc   *services.AuditService
	VaultSvc   *services.VaultService
	Cleaner    *maintenance.Cleaner
	RateStore  middleware.RateStore
	Router     *gin.Engine
}

// bootstrapRuntime initialises databases, caches, services, and the HTTP router.
func bootstrapRuntime(ctx context.Context, cfg *app.Config, log *zap.Logger) (*runtimeStack, error) {
	stack := &runtimeStack{}
	var err error
	success := false

	defer func() {
		if !success {
			stack.Shutdown(context.Background(), log)
		}
	}()

	// enable gin debug mod
	if debug, _ := os.LookupEnv("GIN_DEBUG"); debug != "true" {
		gin.SetMode(gin.ReleaseMode)
	}

	stack.DB, err = initialiseDatabase(cfg)
	if err != nil {
		return nil, err
	}

	if err := database.EnsureVaultEncryptionKey(ctx, stack.DB, cfg.Vault.EncryptionKey); err != nil {
		return nil, err
	}

	dbStore := cache.NewDatabaseStore(stack.DB)

	if cfg.Cache.Redis.Enabled {
		if stack.Redis, err = cache.NewRedisClient(cfg.Cache.RedisClientConfig()); err != nil {
			log.Warn("redis unavailable; falling back to database-backed operations", zap.Error(err))
		} else {
			log.Info("redis connected", zap.String("addr", cfg.Cache.Redis.Address))
		}
	}

	jwtSvc, err := iauth.NewJWTService(cfg.Auth.JWTServiceConfig())
	if err != nil {
		return nil, fmt.Errorf("initialise jwt service: %w", err)
	}

	sessionCfg := cfg.Auth.SessionServiceConfig()
	switch {
	case stack.Redis != nil:
		sessionCfg.Cache = iauth.NewRedisSessionCache(stack.Redis)
	case dbStore != nil:
		sessionCfg.Cache = iauth.NewDatabaseSessionCache(dbStore)
	}

	stack.SessionSvc, err = iauth.NewSessionService(stack.DB, jwtSvc, sessionCfg)
	if err != nil {
		return nil, fmt.Errorf("initialise session service: %w", err)
	}

	stack.AuditSvc, err = services.NewAuditService(stack.DB)
	if err != nil {
		return nil, fmt.Errorf("initialise audit service: %w", err)
	}

	encryptionKey, err := app.DecodeKey(cfg.Vault.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("decode vault encryption key: %w", err)
	}

	vaultCrypto, err := vault.NewCrypto(encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("initialise vault crypto: %w", err)
	}

	stack.VaultSvc, err = services.NewVaultService(stack.DB, stack.AuditSvc, nil, vaultCrypto)
	if err != nil {
		return nil, fmt.Errorf("initialise vault service: %w", err)
	}

	stack.Cleaner = maintenance.NewCleaner(stack.DB, stack.SessionSvc, stack.AuditSvc, maintenance.WithVaultService(stack.VaultSvc))
	if err := stack.Cleaner.Start(); err != nil {
		return nil, fmt.Errorf("start maintenance jobs: %w", err)
	}

	switch {
	case stack.Redis != nil:
		stack.RateStore = middleware.NewRedisRateStore(stack.Redis)
	case dbStore != nil:
		stack.RateStore = middleware.NewDatabaseRateStore(dbStore)
	}

	stack.Router, err = api.NewRouter(stack.DB, jwtSvc, cfg, stack.SessionSvc, stack.RateStore)
	if err != nil {
		return nil, fmt.Errorf("build api router: %w", err)
	}

	success = true
	return stack, nil
}

// Shutdown gracefully stops background jobs and releases resources.
func (s *runtimeStack) Shutdown(ctx context.Context, log *zap.Logger) {
	if s == nil {
		return
	}

	if s.Cleaner != nil {
		stopCtx := s.Cleaner.Stop()
		if stopCtx != nil {
			ctx = stopCtx
		}
		if err := s.Cleaner.RunOnce(ctx); err != nil {
			log.Warn("maintenance shutdown cleanup failed", zap.Error(err))
		}
	}

	if rc, ok := s.Redis.(*cache.RedisClient); ok && rc != nil {
		if err := rc.Close(); err != nil {
			log.Warn("redis shutdown", zap.Error(err))
		}
	}

	if s.DB != nil {
		closeDatabase(s.DB, log)
	}
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
