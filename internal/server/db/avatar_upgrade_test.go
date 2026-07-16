package db

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/avatar"
	_ "github.com/ldm2060/axonhub/internal/pkg/sqlite"
)

func TestNewEntClientMigratesAvatarBeforeDroppingColumn(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "legacy.db")
	dsn := "file:" + filepath.ToSlash(dbPath) + "?_fk=1"

	db, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE systems (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		key TEXT NOT NULL UNIQUE,
		value TEXT NOT NULL
	)`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE users (
		id INTEGER PRIMARY KEY,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		deleted_at INTEGER NOT NULL DEFAULT 0,
		email TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		prefer_language TEXT NOT NULL DEFAULT 'en',
		password TEXT NOT NULL,
		first_name TEXT NOT NULL DEFAULT '',
		last_name TEXT NOT NULL DEFAULT '',
		avatar TEXT,
		is_owner BOOL NOT NULL DEFAULT false,
		scopes JSON,
		email_verified_at DATETIME,
		private_project_id INTEGER
	)`)
	require.NoError(t, err)

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 1, G: 2, B: 3, A: 255})
	var imageData bytes.Buffer
	require.NoError(t, png.Encode(&imageData, img))
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(imageData.Bytes())
	_, err = db.Exec(`INSERT INTO users (id, email, password, avatar) VALUES (1, 'legacy@example.com', 'hash', ?)`, dataURL)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	service := avatar.NewService(avatar.Config{Directory: filepath.Join(dir, "avatars")})
	client := NewEntClient(Config{
		Dialect:              "sqlite3",
		DSN:                  dsn,
		DisableSQLiteAutoWAL: true,
		MaxOpenConns:         1,
		MaxIdleConns:         1,
		ConnMaxLifetime:      time.Minute,
		ConnMaxIdleTime:      time.Minute,
	}, service)
	defer client.Close()

	_, err = os.Stat(service.Path(1))
	require.NoError(t, err)

	rawDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	defer rawDB.Close()
	var avatarColumns int
	err = rawDB.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('users') WHERE name = 'avatar'`).Scan(&avatarColumns)
	require.NoError(t, err)
	require.Zero(t, avatarColumns)

	var marker string
	err = rawDB.QueryRow(`SELECT value FROM systems WHERE key = 'migration:predrop:v0.1.60:file-backed-user-avatars'`).Scan(&marker)
	require.NoError(t, err)
	require.Equal(t, "completed", marker)
}
