package watcher

import (
	"github.com/ldm2060/axonhub/internal/pkg/xredis"
)

const (
	ModeMemory = "memory"
	ModeRedis  = "redis"
)

type Config struct {
	Mode  string        `conf:"mode" yaml:"mode" json:"mode"`
	Redis xredis.Config `conf:"redis" yaml:"redis" json:"redis"`
}
