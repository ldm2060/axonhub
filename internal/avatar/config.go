package avatar

const (
	DefaultDirectory = "data/avatars"
	DefaultMaxBytes  = int64(5 << 20)
	DefaultMaxPixels = int64(16_000_000)
)

type Config struct {
	Directory string `conf:"directory" yaml:"directory" json:"directory"`
	MaxBytes  int64  `conf:"max_bytes" yaml:"max_bytes" json:"max_bytes"`
	MaxPixels int64  `conf:"max_pixels" yaml:"max_pixels" json:"max_pixels"`
}

func (c Config) withDefaults() Config {
	if c.Directory == "" {
		c.Directory = DefaultDirectory
	}
	if c.MaxBytes <= 0 {
		c.MaxBytes = DefaultMaxBytes
	}
	if c.MaxPixels <= 0 {
		c.MaxPixels = DefaultMaxPixels
	}
	return c
}
