package server

import (
	"os"

	"github.com/kastoras/images-api/internal/config"
	"github.com/rs/zerolog"
)

func initLogger(cfg *config.Config) zerolog.Logger {
	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)
	return zerolog.New(os.Stdout).With().Timestamp().Logger()
}
