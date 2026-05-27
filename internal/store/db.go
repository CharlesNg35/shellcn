package store

import (
	"fmt"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/charlesng35/shellcn/internal/app"
	"github.com/charlesng35/shellcn/internal/models"
)

// allModels is the full set of GORM models AutoMigrate manages. Additive schema
// changes are applied automatically; destructive changes are never automatic.
func allModels() []any {
	return []any{
		&models.User{}, &models.Connection{}, &models.Credential{}, &models.Grant{},
		&models.ConnectionFolder{}, &models.ConnectionPlacement{}, &models.CredentialGrant{},
		&models.AuditEntry{}, &models.Snippet{}, &models.Preference{},
		&models.AgentEnrollment{}, &models.PolicyRule{}, &models.Invitation{},
		&models.Recording{},
	}
}

// Driver selects the SQL engine. SQLite is the zero-config single-binary default.
type Driver string

const (
	DriverSQLite   Driver = "sqlite"
	DriverPostgres Driver = "postgres"
	DriverMySQL    Driver = "mysql"
)

// Config configures the control-plane store.
type Config struct {
	Driver Driver
	// DSN is the connection string. For SQLite this is a file path (or
	// "file::memory:?cache=shared" for tests).
	DSN string
	// LogSQL enables GORM's SQL logger at info level.
	LogSQL bool
}

// Open connects using a pure-Go driver, runs AutoMigrate, and wires the repos.
func Open(cfg Config) (*Store, error) {
	dialector, err := dialector(cfg)
	if err != nil {
		return nil, err
	}

	logLevel := logger.Warn
	if cfg.LogSQL {
		logLevel = logger.Info
	}
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger:                                   logger.Default.LogMode(logLevel),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", cfg.Driver, err)
	}

	if err := db.AutoMigrate(allModels()...); err != nil {
		return nil, fmt.Errorf("auto-migrate: %w", err)
	}

	return newGormStore(db), nil
}

func dialector(cfg Config) (gorm.Dialector, error) {
	switch cfg.Driver {
	case DriverSQLite, "":
		dsn := cfg.DSN
		if dsn == "" {
			dsn = app.DefaultDatabaseDSN
		}
		return sqlite.Open(dsn), nil
	case DriverPostgres:
		return postgres.Open(cfg.DSN), nil
	case DriverMySQL:
		return mysql.Open(cfg.DSN), nil
	default:
		return nil, fmt.Errorf("unsupported driver %q", cfg.Driver)
	}
}

// newGormStore wires the GORM-backed repositories.
func newGormStore(db *gorm.DB) *Store {
	return &Store{
		Users:                &gormUserStore{db: db},
		Connections:          &gormConnectionStore{db: db},
		ConnectionFolders:    &gormConnectionFolderStore{db: db},
		ConnectionPlacements: &gormConnectionPlacementStore{db: db},
		Credentials:          &gormCredentialStore{db: db},
		Grants:               &gormGrantStore{db: db},
		CredentialGrants:     &gormCredentialGrantStore{db: db},
		Audit:                &gormAuditStore{db: db},
		Snippets:             &gormSnippetStore{db: db},
		Preferences:          &gormPreferenceStore{db: db},
		Enrollments:          &gormEnrollmentStore{db: db},
		Policies:             &gormPolicyStore{db: db},
		Invitations:          &gormInvitationStore{db: db},
		Recordings:           &gormRecordingStore{db: db},
		close: func() error {
			sqlDB, err := db.DB()
			if err != nil {
				return err
			}
			return sqlDB.Close()
		},
	}
}
