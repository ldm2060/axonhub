package avatar

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ldm2060/axonhub/internal/objects"
)

var ErrInvalidImage = errors.New("invalid avatar image")

type Service struct {
	config Config
}

func NewService(config Config) *Service {
	return &Service{config: config.withDefaults()}
}

func (s *Service) URL(userID int) string {
	return objects.UserAvatarURL(userID)
}

func (s *Service) DefaultURL() string {
	return objects.DefaultUserAvatarURL
}

func (s *Service) MaxBytes() int64 {
	return s.config.MaxBytes
}

func (s *Service) Path(userID int) string {
	return filepath.Join(s.config.Directory, strconv.Itoa(userID)+".png")
}

func (s *Service) legacyURLPath(userID int) string {
	return filepath.Join(s.config.Directory, strconv.Itoa(userID)+".url")
}

func (s *Service) Exists(userID int) bool {
	info, err := os.Stat(s.Path(userID))
	return err == nil && !info.IsDir()
}

func (s *Service) Open(userID int) (*os.File, error) {
	return os.Open(s.Path(userID))
}

func (s *Service) LegacyURL(userID int) (string, bool) {
	data, err := os.ReadFile(s.legacyURLPath(userID))
	if err != nil {
		return "", false
	}
	value := strings.TrimSpace(string(data))
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", false
	}
	return value, true
}

func (s *Service) SaveLegacyURL(userID int, value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("%w: unsupported legacy avatar URL", ErrInvalidImage)
	}
	if err := os.MkdirAll(s.config.Directory, 0o750); err != nil {
		return fmt.Errorf("create avatar directory: %w", err)
	}
	return os.WriteFile(s.legacyURLPath(userID), []byte(parsed.String()), 0o644)
}

func (s *Service) Save(userID int, src io.Reader) error {
	data, err := io.ReadAll(io.LimitReader(src, s.config.MaxBytes+1))
	if err != nil {
		return fmt.Errorf("read avatar: %w", err)
	}
	if int64(len(data)) > s.config.MaxBytes {
		return fmt.Errorf("%w: image exceeds %d bytes", ErrInvalidImage, s.config.MaxBytes)
	}

	img, err := decodeImage(data, s.config.MaxPixels)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(s.config.Directory, 0o750); err != nil {
		return fmt.Errorf("create avatar directory: %w", err)
	}

	temp, err := os.CreateTemp(s.config.Directory, ".avatar-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary avatar: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)

	if err := temp.Chmod(0o644); err != nil {
		_ = temp.Close()
		return fmt.Errorf("set avatar permissions: %w", err)
	}
	if err := png.Encode(temp, img); err != nil {
		_ = temp.Close()
		return fmt.Errorf("encode avatar: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("sync avatar: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close avatar: %w", err)
	}

	if err := replaceFile(tempName, s.Path(userID)); err != nil {
		return fmt.Errorf("replace avatar: %w", err)
	}
	_ = os.Remove(s.legacyURLPath(userID))
	return nil
}

func (s *Service) SaveLegacyDataURL(userID int, value string) error {
	header, encoded, ok := strings.Cut(value, ",")
	if !ok || !strings.HasPrefix(header, "data:image/") {
		return fmt.Errorf("%w: unsupported legacy avatar value", ErrInvalidImage)
	}

	var decoded []byte
	var err error
	if strings.HasSuffix(strings.ToLower(header), ";base64") {
		decoded, err = base64.StdEncoding.DecodeString(encoded)
	} else {
		var text string
		text, err = url.PathUnescape(encoded)
		decoded = []byte(text)
	}
	if err != nil {
		return fmt.Errorf("%w: decode legacy data URL: %v", ErrInvalidImage, err)
	}
	return s.Save(userID, bytes.NewReader(decoded))
}

func decodeImage(data []byte, maxPixels int64) (image.Image, error) {
	format := httpImageFormat(data)
	if format == "" {
		return nil, fmt.Errorf("%w: only PNG, JPEG, and GIF are supported", ErrInvalidImage)
	}

	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return nil, fmt.Errorf("%w: decode image dimensions", ErrInvalidImage)
	}
	if int64(config.Width)*int64(config.Height) > maxPixels {
		return nil, fmt.Errorf("%w: image exceeds %d pixels", ErrInvalidImage, maxPixels)
	}

	var img image.Image
	switch format {
	case "png":
		img, err = png.Decode(bytes.NewReader(data))
	case "jpeg":
		img, err = jpeg.Decode(bytes.NewReader(data))
	case "gif":
		img, err = gif.Decode(bytes.NewReader(data))
	}
	if err != nil {
		return nil, fmt.Errorf("%w: decode image", ErrInvalidImage)
	}
	return img, nil
}

func httpImageFormat(data []byte) string {
	switch {
	case len(data) >= 8 && bytes.Equal(data[:8], []byte("\x89PNG\r\n\x1a\n")):
		return "png"
	case len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff:
		return "jpeg"
	case len(data) >= 6 && (bytes.Equal(data[:6], []byte("GIF87a")) || bytes.Equal(data[:6], []byte("GIF89a"))):
		return "gif"
	default:
		return ""
	}
}

func replaceFile(source, destination string) error {
	if err := os.Rename(source, destination); err == nil {
		return nil
	}
	if err := os.Remove(destination); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(source, destination)
}
