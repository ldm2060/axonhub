package db

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/ldm2060/axonhub/internal/pkg/sqlite"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/avatar"
)

func TestMigrateLegacyAvatarsExportsDataURLs(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:legacy_avatar?mode=memory&cache=shared")
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, avatar TEXT)`)
	require.NoError(t, err)

	var imageData bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	require.NoError(t, png.Encode(&imageData, img))
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(imageData.Bytes())
	_, err = db.Exec(`INSERT INTO users (id, avatar) VALUES (?, ?)`, 42, dataURL)
	require.NoError(t, err)

	dir := t.TempDir()
	service := avatar.NewService(avatar.Config{Directory: dir})
	require.NoError(t, migrateLegacyAvatars(context.Background(), db, "sqlite3", service))

	avatarBytes, err := os.ReadFile(filepath.Join(dir, "42.png"))
	require.NoError(t, err)
	require.NotEmpty(t, avatarBytes)

	exists, err := legacyAvatarColumnExists(context.Background(), db, "sqlite3")
	require.NoError(t, err)
	require.True(t, exists)
}

func TestMigrateLegacyAvatarsPreservesHTTPURLs(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:legacy_avatar_url?mode=memory&cache=shared")
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, avatar TEXT)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO users (id, avatar) VALUES (?, ?)`, 7, "https://example.com/avatar.jpg")
	require.NoError(t, err)

	service := avatar.NewService(avatar.Config{Directory: t.TempDir()})
	require.NoError(t, migrateLegacyAvatars(context.Background(), db, "sqlite3", service))
	value, ok := service.LegacyURL(7)
	require.True(t, ok)
	require.Equal(t, "https://example.com/avatar.jpg", value)
}

func TestMigrateLegacyAvatarsSkipsMissingColumn(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:no_legacy_avatar?mode=memory&cache=shared")
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY)`)
	require.NoError(t, err)

	service := avatar.NewService(avatar.Config{Directory: t.TempDir()})
	require.NoError(t, migrateLegacyAvatars(context.Background(), db, "sqlite3", service))
}
