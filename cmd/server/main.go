// Command server is the ShellCN gateway entrypoint. It wires the core runtime
// (store, secrets, auth, policy, sessions, transport, audit, telemetry), the
// chi HTTP/WS server, and the compiled-in plugins.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"

	aiconfig "github.com/charlesng35/shellcn/internal/ai/config"
	"github.com/charlesng35/shellcn/internal/ai/modelreg"
	"github.com/charlesng35/shellcn/internal/app"
	"github.com/charlesng35/shellcn/internal/audit"
	"github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/cluster"
	"github.com/charlesng35/shellcn/internal/config"
	"github.com/charlesng35/shellcn/internal/email"
	"github.com/charlesng35/shellcn/internal/extplugin"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/pluginmarket"
	"github.com/charlesng35/shellcn/internal/pluginregistry"
	"github.com/charlesng35/shellcn/internal/policy"
	"github.com/charlesng35/shellcn/internal/recording"
	"github.com/charlesng35/shellcn/internal/secrets"
	"github.com/charlesng35/shellcn/internal/server"
	"github.com/charlesng35/shellcn/internal/service"
	"github.com/charlesng35/shellcn/internal/session"
	"github.com/charlesng35/shellcn/internal/store"
	"github.com/charlesng35/shellcn/internal/telemetry"
	"github.com/charlesng35/shellcn/internal/transport"
	"github.com/charlesng35/shellcn/plugins"
	"github.com/charlesng35/shellcn/sdk/plugin"
	"github.com/charlesng35/shellcn/web"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	var (
		showVersion bool
		dev         bool
		addr        string
		dbPath      string
		configPath  string
	)
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.BoolVar(&dev, "dev", false, "dev mode: serve the API only; Vite serves the UI")
	flag.StringVar(&configPath, "config", "", "extra directory to search for config.yaml (besides . and ./config)")
	flag.StringVar(&addr, "addr", "", "address to listen on (overrides config)")
	flag.StringVar(&dbPath, "db", "", "database DSN / SQLite path (overrides config)")
	flag.Parse()

	if showVersion {
		fmt.Printf("%s %s\n", app.ServerBinary, version)
		return
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Default().Error("load config", "err", err)
		os.Exit(1)
	}
	// CLI flags take precedence over file/env config.
	if addr != "" {
		cfg.Server.Addr = addr
	}
	if dbPath != "" {
		cfg.Database.DSN = dbPath
	}

	// Logs go to stdout, or to a size-rotated file when configured.
	logOut := io.Writer(os.Stdout)
	if path := cfg.Server.LogFile; path != "" {
		rotator := &lumberjack.Logger{Filename: path, MaxSize: 100, MaxBackups: 7, MaxAge: 28, Compress: true}
		defer func() { _ = rotator.Close() }()
		logOut = rotator
	}
	format := telemetry.LogFormatJSON
	if dev && cfg.Server.LogFile == "" {
		format = telemetry.LogFormatConsole
	}
	logger := telemetry.NewLogger(telemetry.LogConfig{Level: cfg.SlogLevel(), Format: format, Output: logOut})
	slog.SetDefault(logger)

	if err := run(logger, cfg, dev); err != nil {
		logger.Error("server exited", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger, cfg *config.Config, dev bool) error {
	// Secrets and store.
	// Master key: required in prod; generated (ephemeral) with a loud warning in dev.
	masterKey, err := secrets.ResolveMasterKey(cfg.Secrets.MasterKey, cfg.Secrets.MasterKeyFile)
	if err != nil {
		if !dev {
			return fmt.Errorf("load master key: %w", err)
		}
		masterKey, _ = secrets.GenerateMasterKey()
		logger.Warn("dev: generated an EPHEMERAL master key — set SHELLCN_MASTER_KEY to persist secrets",
			"key", secrets.EncodeMasterKey(masterKey))
	}
	vault, err := secrets.NewVault(masterKey)
	if err != nil {
		return err
	}

	st, err := store.Open(store.Config{Driver: store.Driver(cfg.Database.Driver), DSN: cfg.Database.DSN})
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = st.Close() }()

	if err := bootstrapAdmin(context.Background(), logger, st, cfg.Bootstrap); err != nil {
		return err
	}

	// Core registries and policy.
	reg := pluginregistry.New()
	plugins.Register(reg)

	pol, err := policy.New()
	if err != nil {
		return err
	}
	if err := pol.LoadStorePolicies(context.Background(), st.Policies); err != nil {
		return fmt.Errorf("load policies: %w", err)
	}

	// Live-state ownership and transports.
	instance := cluster.NewInstanceRef("", cluster.DiscoverInternalURL(cluster.PortFromListenAddress(cfg.Server.Addr), false))
	owners := cluster.NewStoreOwnerRegistry(st.ClusterOwners)
	leaseTTL := cfg.Cluster.LeaseTTLDuration()
	renewInterval := cfg.Cluster.RenewIntervalDuration()
	sessions := session.New(session.Options{OwnerRegistry: owners, Instance: instance, LeaseTTL: leaseTTL, RenewInterval: renewInterval})
	defer sessions.Shutdown()

	metrics := telemetry.NewMetrics()
	tunnels := transport.NewRegistry(
		transport.WithOwnerRegistry(owners, instance),
		transport.WithLeaseTTL(leaseTTL),
		transport.WithRenewInterval(renewInterval),
	)

	// Connection services.
	creds := service.NewCredentialService(st.Credentials, st.CredentialGrants, vault, service.WithCredentialKindCatalog(reg))
	creds.SetSecretAccessHook(metrics.IncSecretAccess)

	connector := service.NewConnector(reg, creds, vault, tunnels)
	connector.SetSecretAccessHook(metrics.IncSecretAccess)

	connections := service.NewConnectionService(st.Connections, reg, creds, vault)
	enrollments := service.NewEnrollmentService(st.Enrollments, st.Connections, reg)
	protocols := service.NewProtocolService(st.ProtocolSettings)

	var auditWriter audit.Sink = audit.NewWriter(st.Audit)
	if !cfg.Audit.Enabled {
		auditWriter = audit.Noop{}
		logger.Warn("audit is disabled by configuration")
	}

	// Out-of-tree plugins: register subprocesses from plugins.dir into the same
	// registry as the built-ins, forwarding their audit to the core writer.
	var extPlugins *extplugin.Manager
	var market *pluginmarket.Service
	if cfg.Plugins.Dir != "" {
		extPlugins = extplugin.NewManager(cfg.Plugins.Dir,
			extplugin.WithLogger(logger),
			extplugin.WithAudit(func(result plugin.AuditResult, params map[string]string, errMsg string) {
				var auditErr error
				if errMsg != "" {
					auditErr = errors.New(errMsg)
				}
				auditWriter.Record(context.Background(), audit.Event{
					Event: "plugin.stream", RouteID: "plugin.stream", Risk: string(plugin.RiskPrivileged),
					Result: models.AuditResult(result), Params: params, Err: auditErr,
				})
			}))
		if err := extPlugins.LoadAll(context.Background(), reg); err != nil {
			logger.Warn("load external plugins", "dir", cfg.Plugins.Dir, "err", err)
		}
		defer extPlugins.Close()
	}
	if cfg.Plugins.Market.Enabled && len(cfg.Plugins.Market.Indexes) > 0 && extPlugins != nil {
		market = pluginmarket.New(cfg.Plugins.Market.Indexes)
	}

	// Recording, identity, and AI services.
	recBlobs, err := recording.NewLocalBlobStore(cfg.Recordings.Dir)
	if err != nil {
		return fmt.Errorf("recording storage: %w", err)
	}

	recEngine := recording.NewEngine(recording.Options{
		Store: st.Recordings, Blobs: recBlobs, Audit: auditWriter,
		Metrics: metrics, DefaultRetentionDays: cfg.Recordings.RetentionDays,
	})
	recEngine.Register(plugin.FormatAsciicastV2, recording.NewAsciicastRecorder)

	recordings := service.NewRecordingService(st.Recordings, recBlobs)
	users := service.NewUserService(st.Users)
	twoFactor := service.NewTwoFactorService(st.Users, vault, app.DisplayName)

	mailer := email.New(email.SMTP{
		Enabled:  cfg.Email.Enabled,
		Host:     cfg.Email.Host,
		Port:     cfg.Email.Port,
		From:     cfg.Email.From,
		Username: cfg.Email.Username,
		Password: cfg.Email.Password,
		UseTLS:   cfg.Email.UseTLS,
	})
	invitations := service.NewInvitationService(st.Invitations, users, mailer)

	modelRegistry := modelreg.New(modelreg.WithLogger(logger))
	aiConfig := aiconfig.New(st.AIProviders, vault, cfg.AI).WithModels(modelRegistry)

	// Health and background jobs.
	health := telemetry.NewHealth()
	health.Register("store", func(ctx context.Context) error {
		_, err := st.Users.Count(ctx)
		return err
	})

	// Background maintenance: always reap abandoned chunked (browser-capture)
	// recordings so partial blobs from vanished sessions don't leak; additionally
	// sweep expired recordings when an admin has opted into retention.
	stopCleanup := make(chan struct{})
	defer close(stopCleanup)
	go func() {
		t := time.NewTicker(cfg.Recordings.CleanupEvery())
		defer t.Stop()
		for {
			select {
			case <-stopCleanup:
				return
			case <-t.C:
				// 24h is a safe backstop: it frees genuinely abandoned captures
				// without cutting off legitimately long-running sessions.
				recEngine.ReapStaleChunked(context.Background(), 24*time.Hour)
				if cfg.Recordings.RetentionEnabled() {
					if n, err := recordings.Cleanup(context.Background(), time.Now()); err != nil {
						logger.Warn("recording cleanup failed", "err", err)
					} else if n > 0 {
						logger.Info("recording cleanup removed expired recordings", "count", n)
					}
				}
			}
		}
	}()

	if cfg.Audit.Enabled && cfg.Audit.RetentionEnabled() {
		stopAuditCleanup := make(chan struct{})
		defer close(stopAuditCleanup)

		go func() {
			t := time.NewTicker(cfg.Audit.CleanupEvery())
			defer t.Stop()

			for {
				select {
				case <-stopAuditCleanup:
					return

				case <-t.C:
					before := time.Now().AddDate(0, 0, -cfg.Audit.RetentionDays)

					if n, err := st.Audit.DeleteBefore(context.Background(), before); err != nil {
						logger.Warn("audit cleanup failed", "err", err)
					} else if n > 0 {
						logger.Info("audit cleanup removed expired entries", "count", n)
					}
				}
			}
		}()
	}

	// Reflect live session/channel counts into the gauges.
	stopMetrics := make(chan struct{})
	defer close(stopMetrics)
	go func() {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-stopMetrics:
				return
			case <-t.C:
				s := sessions.Stats()
				metrics.SetSessions(s.Sessions)
				metrics.SetChannels(s.Channels)
			}
		}
	}()

	var staticFS fs.FS
	if !dev {
		staticFS, err = web.FS()
		if err != nil {
			return fmt.Errorf("load embedded frontend: %w", err)
		}
	}

	// HTTP server.
	authKey := cfg.Auth.JWTSigningKey(masterKey)
	srv := server.New(server.Deps{
		Plugins:    reg,
		Store:      st,
		Sessions:   sessions,
		Auth:       auth.NewLocalAuthenticator(st.Users),
		SessionMgr: auth.NewSessionManagerWithKey(cfg.Auth.SessionTTLDuration(), authKey),
		Tickets: auth.NewTicketStore(auth.TicketStoreOptions{
			SigningKey: authKey,
			Owners:     owners,
			Instance:   instance,
		}),
		ArtifactTickets: auth.NewTicketStore(auth.TicketStoreOptions{
			TTL:        service.DefaultEnrollmentTTL,
			SigningKey: authKey,
			Owners:     owners,
			Instance:   instance,
		}),
		Policy:            pol,
		Connector:         connector,
		Connections:       connections,
		Credentials:       creds,
		Enrollments:       enrollments,
		Protocols:         protocols,
		ExtPlugins:        extPlugins,
		Market:            market,
		PluginsDir:        cfg.Plugins.Dir,
		Users:             users,
		TwoFactor:         twoFactor,
		Invitations:       invitations,
		Tunnels:           tunnels,
		Owners:            owners,
		Instance:          instance,
		Recording:         recEngine,
		Recordings:        recordings,
		RecordingMaxChunk: cfg.Recordings.MaxChunkBytes,
		AI:                aiConfig,
		AIGlobal:          cfg.AI,
		ModelRegistry:     modelRegistry,
		Audit:             auditWriter,
		Metrics:           metrics,
		Health:            health,
		Logger:            logger,
		StaticFS:          staticFS,
		Dev:               dev,
		AccessLog:         cfg.Server.AccessLog,
	})

	httpServer := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		mode := "embedded UI"
		if dev {
			mode = "dev (API only; Vite serves the UI)"
		}
		logger.Info("starting", "addr", cfg.Server.Addr, "version", version, "mode", mode)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("listen", "err", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpServer.Shutdown(ctx)
}

// bootstrapAdmin creates a default admin on first run and logs generated credentials.
func bootstrapAdmin(ctx context.Context, logger *slog.Logger, st *store.Store, cfg config.BootstrapConfig) error {
	n, err := st.Users.Count(ctx)
	if err != nil {
		return fmt.Errorf("count users: %w", err)
	}
	if n > 0 {
		return nil
	}

	password := cfg.AdminPassword
	generated := password == ""
	if generated {
		password = uuid.NewString()
	}
	admin, err := service.NewUserService(st.Users).Create(ctx, service.NewUserInput{
		Username:    cfg.AdminUsername,
		DisplayName: "Administrator",
		Roles:       []models.Role{models.RoleAdmin},
		Password:    password,
		Protected:   true,
	})
	if err != nil {
		return fmt.Errorf("create admin: %w", err)
	}
	if generated {
		logger.Warn("created initial admin account", "username", admin.Username, "password", password)
	} else {
		logger.Info("created initial admin account", "username", admin.Username)
	}
	return nil
}
