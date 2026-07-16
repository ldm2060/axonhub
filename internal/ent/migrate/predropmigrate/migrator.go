package predropmigrate

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/ldm2060/axonhub/internal/log"
)

type Migration interface {
	Version() string
	Name() string
	Migrate(ctx context.Context, db *sql.DB, dialectName string) (bool, error)
}

type Migrator struct {
	db         *sql.DB
	dialect    string
	migrations []Migration
}

func NewMigrator(db *sql.DB, dialectName string, migrations ...Migration) *Migrator {
	return &Migrator{db: db, dialect: dialectName, migrations: migrations}
}

func (m *Migrator) Run(ctx context.Context) error {
	for _, migration := range m.migrations {
		key := markerKey(migration)
		completed, err := markerCompleted(ctx, m.db, m.dialect, key)
		if err != nil {
			return fmt.Errorf("check pre-drop migration %s: %w", migration.Name(), err)
		}
		if completed {
			continue
		}

		log.Info(ctx, "executing pre-drop migration",
			log.String("version", migration.Version()),
			log.String("name", migration.Name()),
		)
		executed, err := migration.Migrate(ctx, m.db, m.dialect)
		if err != nil {
			return fmt.Errorf("pre-drop migration %s failed: %w", migration.Name(), err)
		}
		if !executed {
			continue
		}
		if err := writeMarker(ctx, m.db, m.dialect, key); err != nil {
			return fmt.Errorf("record pre-drop migration %s: %w", migration.Name(), err)
		}
	}
	return nil
}

func markerKey(migration Migration) string {
	return "migration:predrop:" + migration.Version() + ":" + migration.Name()
}

func markerCompleted(ctx context.Context, db *sql.DB, dialectName, key string) (bool, error) {
	var value string
	query := "SELECT value FROM systems WHERE key = ?"
	if dialectName == "postgres" {
		query = "SELECT value FROM systems WHERE key = $1"
	}
	err := db.QueryRowContext(ctx, query, key).Scan(&value)
	if err == nil {
		return value == "completed", nil
	}
	if err == sql.ErrNoRows || isMissingSystemsTableError(err) {
		return false, nil
	}
	return false, err
}

func writeMarker(ctx context.Context, db *sql.DB, dialectName, key string) error {
	var query string
	switch dialectName {
	case "sqlite", "sqlite3":
		query = "INSERT INTO systems (key, value) VALUES (?, 'completed') ON CONFLICT (key) DO UPDATE SET value = 'completed'"
	case "postgres":
		query = "INSERT INTO systems (key, value) VALUES ($1, 'completed') ON CONFLICT (key) DO UPDATE SET value = 'completed'"
	case "mysql":
		query = "INSERT INTO systems (`key`, `value`) VALUES (?, 'completed') ON DUPLICATE KEY UPDATE `value` = 'completed'"
	default:
		return fmt.Errorf("unsupported database dialect %q", dialectName)
	}
	_, err := db.ExecContext(ctx, query, key)
	return err
}

func isMissingSystemsTableError(err error) bool {
	return err != nil && (containsError(err, "no such table: systems") || containsError(err, "does not exist") || containsError(err, "doesn't exist"))
}

func containsError(err error, text string) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), strings.ToLower(text))
}
