package config

import (
	"fmt"
	"strconv"
	"time"

	"github.com/kastoras/go-utilities/env_parameters"
)

type Config struct {
	Port string

	RedisEnabled bool
	S3Enabled    bool

	AuthenticationType string
	APIToken           string
	ZitadelIssuer      string
	ZitadelClientID    string
	ZitadelAudience    string

	RedisURL      string
	RedisPassword string

	S3Endpoint     string
	S3Region       string
	S3Bucket       string
	S3AccessKey    string
	S3SecretKey    string
	S3UsePathStyle bool

	MaxWorkers     int
	MaxQueueDepth  int
	ProcessTimeout time.Duration

	// How long to keep retrying Redis/S3 at startup before giving up and
	// running degraded. Set to 0 for a single attempt.
	DependencyWaitTimeout time.Duration

	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration

	LogLevel string
}

func Load() (*Config, error) {
	cfg := &Config{}

	var (
		s   string
		err error
	)

	cfg.Port, err = env_parameters.GetString("API_PORT", "8080")
	if err != nil {
		return nil, err
	}

	cfg.RedisEnabled, err = env_parameters.GetBool("REDIS_ENABLED", false)
	if err != nil {
		return nil, err
	}

	cfg.S3Enabled, err = env_parameters.GetBool("S3_ENABLED", false)
	if err != nil {
		return nil, err
	}

	err = cfg.authenticationConfig()
	if err != nil {
		return nil, err
	}

	cfg.RedisURL, err = env_parameters.GetString("REDIS_URL", "redis:6379")
	if err != nil {
		return nil, err
	}

	cfg.RedisPassword, err = env_parameters.GetString("REDIS_PASSWORD", "")
	if err != nil {
		cfg.RedisPassword = "" // empty password is valid (Redis without auth)
	}

	cfg.S3Endpoint, err = env_parameters.GetString("S3_ENDPOINT", "")
	if err != nil {
		return nil, err
	}

	cfg.S3Region, err = env_parameters.GetString("S3_REGION", "us-east-1")
	if err != nil {
		return nil, err
	}

	cfg.S3Bucket, err = env_parameters.GetString("S3_BUCKET", "image-processor")
	if err != nil {
		return nil, err
	}

	cfg.S3AccessKey, err = env_parameters.GetString("S3_ACCESS_KEY", "")
	if err != nil {
		return nil, err
	}

	cfg.S3SecretKey, err = env_parameters.GetString("S3_SECRET_KEY", "")
	if err != nil {
		return nil, err
	}

	s, err = env_parameters.GetString("S3_USE_PATH_STYLE", "false")
	if err != nil {
		return nil, err
	}
	cfg.S3UsePathStyle, _ = strconv.ParseBool(s)

	cfg.MaxWorkers, err = env_parameters.GetInt("MAX_WORKERS", 10)
	if err != nil {
		return nil, err
	}
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = 10
	}

	cfg.MaxQueueDepth, err = env_parameters.GetInt("MAX_QUEUE_DEPTH", 50)
	if err != nil {
		return nil, err
	}
	if cfg.MaxQueueDepth <= 0 {
		cfg.MaxQueueDepth = 50
	}

	cfg.ProcessTimeout, err = env_parameters.GetDuration("PROCESSING_TIMEOUT", 10, time.Second)
	if err != nil {
		cfg.ProcessTimeout = 10 * time.Second
	}

	cfg.DependencyWaitTimeout, err = env_parameters.GetDuration("DEPENDENCY_WAIT_TIMEOUT", 60, time.Second)
	if err != nil {
		cfg.DependencyWaitTimeout = 60 * time.Second
	}

	cfg.ReadTimeout, err = env_parameters.GetDuration("READ_TIMEOUT", 15, time.Second)
	if err != nil {
		cfg.ReadTimeout = 15 * time.Second
	}

	cfg.WriteTimeout, err = env_parameters.GetDuration("WRITE_TIMEOUT", 15, time.Second)
	if err != nil {
		cfg.WriteTimeout = 15 * time.Second
	}

	cfg.IdleTimeout, err = env_parameters.GetDuration("IDLE_TIMEOUT", 60, time.Second)
	if err != nil {
		cfg.IdleTimeout = 60 * time.Second
	}

	cfg.LogLevel, err = env_parameters.GetString("LOG_LEVEL", "info")
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) authenticationConfig() error {
	var err error

	c.AuthenticationType, err = env_parameters.GetString("AUTHENTICATION_TYPE", "bearer")
	if err != nil {
		return err
	}

	switch c.AuthenticationType {
	case "bearer":
		c.APIToken, err = env_parameters.GetString("API_TOKEN", "")
		if err != nil {
			return err
		}
		return nil
	case "zitadel":
		c.ZitadelIssuer, err = env_parameters.GetString("ZITADEL_ISSUER", "")
		if err != nil {
			return err
		}

		c.ZitadelClientID, err = env_parameters.GetString("ZITADEL_CLIENT_ID", "")
		if err != nil {
			return err
		}

		c.ZitadelAudience, err = env_parameters.GetString("ZITADEL_AUDIENCE", "image-api")
		if err != nil {
			return err
		}
		return nil
	}

	return fmt.Errorf("no supported authentication type selected: %q", c.AuthenticationType)
}
