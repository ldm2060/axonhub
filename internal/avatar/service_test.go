package avatar

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServiceSaveAndOpen(t *testing.T) {
	dir := t.TempDir()
	service := NewService(Config{Directory: dir})

	var data bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 12, G: 34, B: 56, A: 255})
	require.NoError(t, png.Encode(&data, img))
	require.NoError(t, service.Save(7, bytes.NewReader(data.Bytes())))

	file, err := service.Open(7)
	require.NoError(t, err)
	defer file.Close()

	decoded, err := png.Decode(file)
	require.NoError(t, err)
	require.Equal(t, image.Rect(0, 0, 1, 1), decoded.Bounds())
}

func TestServiceRejectsSVG(t *testing.T) {
	service := NewService(Config{Directory: t.TempDir()})
	err := service.Save(1, bytes.NewBufferString(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`))
	require.ErrorIs(t, err, ErrInvalidImage)
	_, statErr := os.Stat(service.Path(1))
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestServiceRejectsOversizedInput(t *testing.T) {
	service := NewService(Config{Directory: t.TempDir(), MaxBytes: 8})
	err := service.Save(1, bytes.NewReader(make([]byte, 9)))
	require.ErrorIs(t, err, ErrInvalidImage)
}
