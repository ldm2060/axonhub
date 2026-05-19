package api

import (
	"github.com/ldm2060/axonhub/internal/log"
)

var logger *log.Logger

func initLogger(l *log.Logger) {
	logger = l.WithName("api")
}
