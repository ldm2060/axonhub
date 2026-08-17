package datamigrate

import (
	"context"
	"database/sql"
	"fmt"

	"entgo.io/ent/dialect"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/log"
)

// V0_1_69 implements DataMigrator for version 0.1.69 migration.
type V0_1_69 struct{}

// NewV0_1_69 creates the v0.1.69 data migrator.
func NewV0_1_69() *V0_1_69 {
	return &V0_1_69{}
}

// Version returns the migration version.
func (v *V0_1_69) Version() string {
	return "v0.1.69"
}

// Migrate purges the stale settings.providerQuota key from channels.
//
// The old OpenCode Go quota checker stored workspaceId and a live auth cookie
// under settings.providerQuota.opencodeGo. Quota now uses the official usage
// API keyed by the channel's own API key, so the key is never read again and
// must not linger in the database (it may hold a live session credential).
func (v *V0_1_69) Migrate(ctx context.Context, client *ent.Client) (err error) {
	ctx = authz.WithSystemBypass(ctx, "database-migrate")

	// Use the dialect.Driver.Exec interface rather than asserting a concrete
	// *entsql.Driver: under read-replica configuration the ent client wraps the
	// driver in a routerDriver (internal/server/db), which still implements
	// dialect.Driver.Exec (routing writes to the master).
	dialectName := client.Driver().Dialect()

	var stmt string
	switch dialectName {
	case dialect.Postgres:
		stmt = `UPDATE channels SET settings = settings #- '{providerQuota}' WHERE settings ? 'providerQuota'`
	case dialect.SQLite:
		// json_type (not json_extract) distinguishes a present JSON-null value
		// from an absent key, so "providerQuota": null is purged as well.
		stmt = `UPDATE channels SET settings = json_remove(settings, '$.providerQuota') WHERE json_type(settings, '$.providerQuota') IS NOT NULL`
	default:
		log.Info(ctx, "Unsupported dialect, skipping stale providerQuota cleanup",
			log.String("dialect", dialectName))
		return nil
	}

	var result sql.Result
	if err := client.Driver().Exec(ctx, stmt, []any{}, &result); err != nil {
		return fmt.Errorf("failed to purge settings.providerQuota: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		log.Warn(ctx, "failed to read affected rows after providerQuota purge", log.Cause(err))
	} else {
		log.Info(ctx, "Purged stale settings.providerQuota from channels",
			log.Int64("affected", affected))
	}

	return nil
}
