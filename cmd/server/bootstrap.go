package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/api"
	"github.com/charlesng35/shellcn/internal/app"
	"github.com/charlesng35/shellcn/internal/app/maintenance"
	iauth "github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/cache"
	"github.com/charlesng35/shellcn/internal/database"
	"github.com/charlesng35/shellcn/internal/drivers"
	_ "github.com/charlesng35/shellcn/internal/drivers/ssh"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/monitoring"
	"github.com/charlesng35/shellcn/internal/monitoring/checks"
	"github.com/charlesng35/shellcn/internal/services"
	"github.com/charlesng35/shellcn/internal/vault"
	"github.com/charlesng35/shellcn/pkg/logger"
)

// runtimeStack bundles long-lived services used by the HTTP server.
type runtimeStack struct {
	DB             *gorm.DB
	Redis          cache.Store
	RedisClient    *cache.RedisClient
	SessionSvc     *iauth.SessionService
	AuditSvc       *services.AuditService
	VaultSvc       *services.VaultService
	RecorderSvc    *services.RecorderService
	Cleaner        *maintenance.Cleaner
	RateStore      middleware.RateStore
	DriverRegistry *drivers.Registry
	Router         *gin.Engine
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

	// ---------------------------------------------------------------------------
	// Monitoring / Telemetry
	// ---------------------------------------------------------------------------
	mon, err := monitoring.NewModule(monitoring.Options{})
	if err != nil {
		return nil, fmt.Errorf("initialise monitoring module: %w", err)
	}
	monitoring.SetModule(mon)

	// enable gin debug mod
	if debug, _ := os.LookupEnv("GIN_DEBUG"); debug != "true" {
		fmt.Print("GIN is in RELEASE MODE; export GIN_DEBUG=true to enable Gin debug.")
		gin.SetMode(gin.ReleaseMode)
	}

	// ---------------------------------------------------------------------------
	// Database (primary persistence layer)
	// ---------------------------------------------------------------------------
	stack.DB, err = initialiseDatabase(cfg)
	if err != nil {
		return nil, err
	}
	if health := mon.Health(); health != nil {
		health.RegisterReadiness(checks.Database(stack.DB, 2*time.Second))
		health.RegisterLiveness(monitoring.NewCheck("uptime", func(ctx context.Context) monitoring.ProbeResult {
			return monitoring.ProbeResult{Status: monitoring.StatusUp}
		}))
	}

	if err := database.EnsureVaultEncryptionKey(ctx, stack.DB, cfg.Vault.EncryptionKey); err != nil {
		return nil, err
	}

	// ---------------------------------------------------------------------------
	// System Settings Seeding
	// ---------------------------------------------------------------------------
	if err := seedSSHProtocolDefaults(ctx, stack.DB, cfg); err != nil {
		return nil, fmt.Errorf("seed ssh defaults: %w", err)
	}
	if err := seedRecordingDefaults(ctx, stack.DB, cfg); err != nil {
		return nil, fmt.Errorf("seed recording defaults: %w", err)
	}
	if err := seedSessionSharingDefaults(ctx, stack.DB, cfg); err != nil {
		return nil, fmt.Errorf("seed session sharing defaults: %w", err)
	}

	// ---------------------------------------------------------------------------
	// Protocol Catalogue
	// ---------------------------------------------------------------------------
	stack.DriverRegistry = drivers.DefaultRegistry()
	catalogSvc, err := services.NewProtocolCatalogService(stack.DB)
	if err != nil {
		return nil, fmt.Errorf("initialise protocol catalog service: %w", err)
	}
	if err := catalogSvc.Sync(ctx, stack.DriverRegistry, cfg); err != nil {
		return nil, fmt.Errorf("sync protocol catalog: %w", err)
	}

	// ---------------------------------------------------------------------------
	// Cache & Rate Limiting Stores
	// ---------------------------------------------------------------------------
	dbStore := cache.NewDatabaseStore(stack.DB)

	if cfg.Cache.Redis.Enabled {
		redisClient, redisErr := cache.NewRedisClient(cfg.Cache.RedisClientConfig())
		if redisErr != nil {
			log.Warn("redis unavailable; falling back to database-backed operations", zap.Error(redisErr))
		} else {
			stack.Redis = redisClient
			stack.RedisClient = redisClient
			log.Info("redis connected", zap.String("addr", cfg.Cache.Redis.Address))
			if health := mon.Health(); health != nil {
				health.RegisterReadiness(checks.Redis(redisClient, true, 2*time.Second))
			}
		}
	} else {
		if health := mon.Health(); health != nil {
			health.RegisterReadiness(checks.Redis(nil, false, 0))
		}
	}

	// ---------------------------------------------------------------------------
	// Authentication / Sessions
	// ---------------------------------------------------------------------------
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

	// ---------------------------------------------------------------------------
	// Auditing
	// ---------------------------------------------------------------------------
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

	// ---------------------------------------------------------------------------
	// Vault Service
	// ---------------------------------------------------------------------------
	stack.VaultSvc, err = services.NewVaultService(stack.DB, stack.AuditSvc, nil, vaultCrypto)
	if err != nil {
		return nil, fmt.Errorf("initialise vault service: %w", err)
	}

	// ---------------------------------------------------------------------------
	// Recorder (Session Recording)
	// ---------------------------------------------------------------------------
	recordingRoot := filepath.Join("data", "records")
	recorderStore, err := services.NewFilesystemRecorderStore(recordingRoot)
	if err != nil {
		return nil, fmt.Errorf("initialise recorder store: %w", err)
	}

	recorderPolicy := services.LoadRecorderPolicy(ctx, stack.DB)
	stack.RecorderSvc, err = services.NewRecorderService(stack.DB, recorderStore, services.WithRecorderPolicy(recorderPolicy))
	if err != nil {
		return nil, fmt.Errorf("initialise recorder service: %w", err)
	}

	// ---------------------------------------------------------------------------
	// Background Maintenance Jobs
	// ---------------------------------------------------------------------------
	stack.Cleaner = maintenance.NewCleaner(stack.DB, stack.SessionSvc, stack.AuditSvc,
		maintenance.WithVaultService(stack.VaultSvc),
		maintenance.WithRecorderService(stack.RecorderSvc),
	)
	if err := stack.Cleaner.Start(); err != nil {
		return nil, fmt.Errorf("start maintenance jobs: %w", err)
	}

	if health := mon.Health(); health != nil {
		health.RegisterReadiness(checks.Maintenance(0))
	}

	// ---------------------------------------------------------------------------
	// Rate Limiter Store Selection
	// ---------------------------------------------------------------------------
	switch {
	case stack.Redis != nil:
		stack.RateStore = middleware.NewRedisRateStore(stack.Redis)
	case dbStore != nil:
		stack.RateStore = middleware.NewDatabaseRateStore(dbStore)
	}

	// ---------------------------------------------------------------------------
	// HTTP Router
	// ---------------------------------------------------------------------------
	stack.Router, err = api.NewRouter(stack.DB, jwtSvc, cfg, stack.DriverRegistry, stack.SessionSvc, stack.RateStore, mon, stack.RecorderSvc)
	if err != nil {
		return nil, fmt.Errorf("build api router: %w", err)
	}

	success = true
	return stack, nil
}

func seedSSHProtocolDefaults(ctx context.Context, db *gorm.DB, cfg *app.Config) error {
	if db == nil || cfg == nil {
		return nil
	}

	sshCfg := cfg.Protocols.SSH
	defaults := map[string]string{
		"protocol.ssh.enable_sftp_default":       strconv.FormatBool(sshCfg.EnableSFTPDefault),
		"protocol.ssh.terminal.theme_mode":       strings.ToLower(strings.TrimSpace(sshCfg.Terminal.ThemeMode)),
		"protocol.ssh.terminal.font_family":      strings.TrimSpace(sshCfg.Terminal.FontFamily),
		"protocol.ssh.terminal.font_size":        strconv.Itoa(max(sshCfg.Terminal.FontSize, 0)),
		"protocol.ssh.terminal.scrollback_limit": strconv.Itoa(max(sshCfg.Terminal.Scrollback, 0)),
	}

	for key, value := range defaults {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		current, err := database.GetSystemSetting(ctx, db, key)
		if err != nil {
			return err
		}
		if strings.TrimSpace(current) != "" {
			continue
		}
		if err := database.UpsertSystemSetting(ctx, db, key, value); err != nil {
			return err
		}
	}

	return nil
}

func seedRecordingDefaults(ctx context.Context, db *gorm.DB, cfg *app.Config) error {
	if db == nil || cfg == nil {
		return nil
	}

	recCfg := cfg.Features.Recording
	defaults := map[string]string{
		"recording.mode":            strings.ToLower(strings.TrimSpace(recCfg.Mode)),
		"recording.storage":         strings.ToLower(strings.TrimSpace(recCfg.Storage)),
		"recording.retention_days":  strconv.Itoa(max(recCfg.RetentionDays, 0)),
		"recording.require_consent": strconv.FormatBool(recCfg.RequireConsent),
	}

	for key, value := range defaults {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		current, err := database.GetSystemSetting(ctx, db, key)
		if err != nil {
			return err
		}
		if strings.TrimSpace(current) != "" {
			continue
		}
		if err := database.UpsertSystemSetting(ctx, db, key, value); err != nil {
			return err
		}
	}

	return nil
}

func seedSessionSharingDefaults(ctx context.Context, db *gorm.DB, cfg *app.Config) error {
	if db == nil || cfg == nil {
		return nil
	}

	shareCfg := cfg.Features.SessionSharing
	sessCfg := cfg.Features.Sessions
	defaults := map[string]string{
		"session_sharing.enabled":                  strconv.FormatBool(shareCfg.Enabled),
		"session_sharing.max_shared_users":         strconv.Itoa(max(shareCfg.MaxSharedUsers, 0)),
		"session_sharing.allow_default":            strconv.FormatBool(shareCfg.AllowDefault),
		"session_sharing.restrict_write_to_admins": strconv.FormatBool(shareCfg.RestrictWriteToAdmins),
		"sessions.concurrent_limit_default":        strconv.Itoa(max(sessCfg.ConcurrentLimitDefault, 0)),
		"sessions.idle_timeout_minutes":            formatIdleTimeoutMinutes(sessCfg.IdleTimeout),
	}

	for key, value := range defaults {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		current, err := database.GetSystemSetting(ctx, db, key)
		if err != nil {
			return err
		}
		if strings.TrimSpace(current) != "" {
			continue
		}
		if err := database.UpsertSystemSetting(ctx, db, key, value); err != nil {
			return err
		}
	}

	return nil
}

func formatIdleTimeoutMinutes(d time.Duration) string {
	if d <= 0 {
		return "0"
	}
	minutes := int(d / time.Minute)
	if minutes <= 0 {
		minutes = 0
	}
	return strconv.Itoa(minutes)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
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

	if s.RedisClient != nil {
		if err := s.RedisClient.Close(); err != nil {
			log.Warn("redis shutdown", zap.Error(err))
		}
	} else if rc, ok := s.Redis.(*cache.RedisClient); ok && rc != nil {
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
