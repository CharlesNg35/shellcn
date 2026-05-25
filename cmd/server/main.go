// Command server is the ShellCN gateway entrypoint. It wires the core runtime
// (store, secrets, auth, policy, sessions, transport, audit, telemetry), the
// chi HTTP/WS server, and the compiled-in plugins.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/charlesng/shellcn/internal/audit"
	"github.com/charlesng/shellcn/internal/auth"
	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/policy"
	"github.com/charlesng/shellcn/internal/secrets"
	"github.com/charlesng/shellcn/internal/server"
	"github.com/charlesng/shellcn/internal/service"
	"github.com/charlesng/shellcn/internal/session"
	"github.com/charlesng/shellcn/internal/store"
	"github.com/charlesng/shellcn/internal/telemetry"
	"github.com/charlesng/shellcn/internal/transport"
	"github.com/charlesng/shellcn/plugins/noop"
	"github.com/charlesng/shellcn/web"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	var (
		showVersion bool
		dev         bool
		addr        string
		dbPath      string
	)
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.BoolVar(&dev, "dev", false, "dev mode: serve the API only; Vite serves the UI")
	flag.StringVar(&addr, "addr", ":8080", "address to listen on")
	flag.StringVar(&dbPath, "db", "shellcn.db", "SQLite database path")
	flag.Parse()

	if showVersion {
		fmt.Printf("shellcn %s\n", version)
		return
	}

	logger := telemetry.NewLogger(slog.LevelInfo, !dev)
	slog.SetDefault(logger)

	if err := run(logger, addr, dbPath, dev); err != nil {
		logger.Error("server exited", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger, addr, dbPath string, dev bool) error {
	// Master key: required in prod; generated (ephemeral) with a loud warning in dev.
	masterKey, err := secrets.LoadMasterKey()
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

	st, err := store.Open(store.Config{Driver: store.DriverSQLite, DSN: dbPath})
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = st.Close() }()

	if err := bootstrapAdmin(context.Background(), logger, st); err != nil {
		return err
	}

	reg := plugin.NewRegistry()
	reg.MustRegister(noop.New())

	pol, err := policy.New()
	if err != nil {
		return err
	}

	sessions := session.New(session.Options{})
	defer sessions.Shutdown()

	creds := service.NewCredentialService(st.Credentials, st.CredentialGrants, vault)
	connector := service.NewConnector(reg, creds, vault, transport.EmptyTunnelRegistry{})

	metrics := telemetry.NewMetrics()
	health := telemetry.NewHealth()
	health.Register("store", func(ctx context.Context) error {
		_, err := st.Users.Count(ctx)
		return err
	})

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

	srv := server.New(server.Deps{
		Plugins:     reg,
		Store:       st,
		Sessions:    sessions,
		Auth:        auth.NewLocalAuthenticator(st.Users),
		SessionMgr:  auth.NewSessionManager(0),
		Tickets:     auth.NewTicketStore(0),
		Policy:      pol,
		Connector:   connector,
		Credentials: creds,
		Audit:       audit.NewWriter(st.Audit),
		Metrics:     metrics,
		Health:      health,
		Logger:      logger,
		StaticFS:    staticFS,
		Dev:         dev,
	})

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		mode := "embedded UI"
		if dev {
			mode = "dev (API only; Vite serves the UI)"
		}
		logger.Info("starting", "addr", addr, "version", version, "mode", mode)
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

// bootstrapAdmin creates a default admin on first run and logs its credentials.
func bootstrapAdmin(ctx context.Context, logger *slog.Logger, st *store.Store) error {
	n, err := st.Users.Count(ctx)
	if err != nil {
		return fmt.Errorf("count users: %w", err)
	}
	if n > 0 {
		return nil
	}

	password := os.Getenv("SHELLCN_ADMIN_PASSWORD")
	generated := password == ""
	if generated {
		password = uuid.NewString()
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	admin := &models.User{
		ID:          uuid.NewString(),
		Username:    "admin",
		DisplayName: "Administrator",
		Roles:       []models.Role{models.RoleAdmin},
	}
	if err := st.Users.Create(ctx, admin, hash); err != nil {
		return fmt.Errorf("create admin: %w", err)
	}
	if generated {
		logger.Warn("created initial admin account", "username", "admin", "password", password)
	} else {
		logger.Info("created initial admin account", "username", "admin")
	}
	return nil
}
