package predropmigrate

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"testing"

	_ "github.com/ldm2060/axonhub/internal/pkg/sqlite"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/avatar"
)

func TestAvatarMigrationRecordsCompletionMarker(t *testing.T) {
	db := newLegacyAvatarDB(t, "predrop_avatar_success", validAvatarDataURL(t))
	defer db.Close()

	service := avatar.NewService(avatar.Config{Directory: filepath.Join(t.TempDir(), "avatars")})
	migrator := NewMigrator(db, "sqlite3", NewAvatarMigration(service))
	require.NoError(t, migrator.Run(context.Background()))

	var marker string
	err := db.QueryRow(`SELECT value FROM systems WHERE key = 'migration:predrop:v0.1.60:file-backed-user-avatars'`).Scan(&marker)
	require.NoError(t, err)
	require.Equal(t, "completed", marker)
	require.FileExists(t, service.Path(1))
}

func TestAvatarMigrationFailureDoesNotRecordMarker(t *testing.T) {
	db := newLegacyAvatarDB(t, "predrop_avatar_failure", "unsupported://avatar")
	defer db.Close()

	service := avatar.NewService(avatar.Config{Directory: t.TempDir()})
	migrator := NewMigrator(db, "sqlite3", NewAvatarMigration(service))
	require.Error(t, migrator.Run(context.Background()))

	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM systems WHERE key = 'migration:predrop:v0.1.60:file-backed-user-avatars'`).Scan(&count)
	require.NoError(t, err)
	require.Zero(t, count)

	exists, err := avatarColumnExists(context.Background(), db, "sqlite3")
	require.NoError(t, err)
	require.True(t, exists)
}

func TestAvatarMigrationSkipsCompletedMarker(t *testing.T) {
	db := newLegacyAvatarDB(t, "predrop_avatar_completed", "unsupported://avatar")
	defer db.Close()

	_, err := db.Exec(`INSERT INTO systems (key, value) VALUES ('migration:predrop:v0.1.60:file-backed-user-avatars', 'completed')`)
	require.NoError(t, err)

	migrator := NewMigrator(db, "sqlite3", NewAvatarMigration(avatar.NewService(avatar.Config{Directory: t.TempDir()})))
	require.NoError(t, migrator.Run(context.Background()))
}

func newLegacyAvatarDB(t *testing.T, name, value string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", "file:"+name+"?mode=memory&cache=shared")
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE systems (id INTEGER PRIMARY KEY, key TEXT NOT NULL UNIQUE, value TEXT NOT NULL)`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, avatar TEXT)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO users (id, avatar) VALUES (1, ?)`, value)
	require.NoError(t, err)
	return db
}

func validAvatarDataURL(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 1, G: 2, B: 3, A: 255})
	var data bytes.Buffer
	require.NoError(t, png.Encode(&data, img))
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data.Bytes())
}
