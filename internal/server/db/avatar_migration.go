package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/ldm2060/axonhub/internal/avatar"
)

func migrateLegacyAvatars(ctx context.Context, db *sql.DB, dialectName string, service *avatar.Service) error {
	exists, err := legacyAvatarColumnExists(ctx, db, dialectName)
	if err != nil {
		return fmt.Errorf("check legacy avatar column: %w", err)
	}
	if !exists {
		return nil
	}

	rows, err := db.QueryContext(ctx, "SELECT id, avatar FROM users WHERE avatar IS NOT NULL AND avatar <> ''")
	if err != nil {
		return fmt.Errorf("query legacy avatars: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			userID int
			value  string
		)
		if err := rows.Scan(&userID, &value); err != nil {
			return fmt.Errorf("scan legacy avatar: %w", err)
		}

		switch {
		case strings.HasPrefix(value, "data:image/"):
			if err := service.SaveLegacyDataURL(userID, value); err != nil {
				return fmt.Errorf("migrate avatar for user %d: %w", userID, err)
			}
		case strings.HasPrefix(value, "http://"), strings.HasPrefix(value, "https://"):
			if err := service.SaveLegacyURL(userID, value); err != nil {
				return fmt.Errorf("migrate avatar URL for user %d: %w", userID, err)
			}
		case value == service.DefaultURL(), strings.HasPrefix(value, "/avatars/"):
			// Already represented by the file-backed avatar contract.
		default:
			return fmt.Errorf("migrate avatar for user %d: unsupported legacy avatar value", userID)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate legacy avatars: %w", err)
	}
	return nil
}

func legacyAvatarColumnExists(ctx context.Context, db *sql.DB, dialectName string) (bool, error) {
	var query string
	switch dialectName {
	case "sqlite", "sqlite3":
		query = "SELECT COUNT(*) FROM pragma_table_info('users') WHERE name = 'avatar'"
	case "postgres":
		query = "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'users' AND column_name = 'avatar'"
	case "mysql":
		query = "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'users' AND column_name = 'avatar'"
	default:
		return false, fmt.Errorf("unsupported database dialect %q", dialectName)
	}

	var count int
	if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return count > 0, nil
}
