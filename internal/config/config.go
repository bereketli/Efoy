// Package config loads service configuration with koanf.
//
// Sources are layered, later ones overriding earlier ones:
//  1. built-in defaults (suitable for local development),
//  2. an optional YAML file named by EFOY_CONFIG_FILE,
//  3. EFOY_* environment variables, where a double underscore separates
//     nested keys: EFOY_HTTP__READ_TIMEOUT=10s sets http.read_timeout.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

const envPrefix = "EFOY_"

// Config is shared by all Efoy binaries; each service reads the parts it needs.
type Config struct {
	Env      string   `koanf:"env"`
	HTTP     HTTP     `koanf:"http"`
	Log      Log      `koanf:"log"`
	Database Database `koanf:"database"`
	Redis    Redis    `koanf:"redis"`
	NATS     NATS     `koanf:"nats"`
	S3       S3       `koanf:"s3"`
	OSRM     OSRM     `koanf:"osrm"`
}

type HTTP struct {
	Addr            string        `koanf:"addr"`
	ReadTimeout     time.Duration `koanf:"read_timeout"`
	WriteTimeout    time.Duration `koanf:"write_timeout"`
	IdleTimeout     time.Duration `koanf:"idle_timeout"`
	ShutdownTimeout time.Duration `koanf:"shutdown_timeout"`
}

type Log struct {
	Level  string `koanf:"level"`  // debug | info | warn | error
	Format string `koanf:"format"` // json | text
}

type Database struct {
	URL      string `koanf:"url"`
	MaxConns int32  `koanf:"max_conns"`
}

type Redis struct {
	Addr     string `koanf:"addr"`
	Password string `koanf:"password"`
	DB       int    `koanf:"db"`
}

type NATS struct {
	URL string `koanf:"url"`
}

type S3 struct {
	Endpoint  string `koanf:"endpoint"`
	AccessKey string `koanf:"access_key"`
	SecretKey string `koanf:"secret_key"`
	UseSSL    bool   `koanf:"use_ssl"`
}

type OSRM struct {
	URL string `koanf:"url"`
}

func defaults() map[string]any {
	return map[string]any{
		"env":                   "dev",
		"http.addr":             ":8080",
		"http.read_timeout":     "15s",
		"http.write_timeout":    "30s",
		"http.idle_timeout":     "120s",
		"http.shutdown_timeout": "20s",
		"log.level":             "info",
		"log.format":            "json",
		"database.url":          "postgres://efoy:efoy@localhost:5432/efoy?sslmode=disable",
		"database.max_conns":    20,
		"redis.addr":            "localhost:6379",
		"redis.db":              0,
		"nats.url":              "nats://localhost:4222",
		"s3.endpoint":           "localhost:9000",
		"s3.use_ssl":            false,
		"osrm.url":              "http://localhost:5000",
	}
}

// Load builds the configuration from defaults, an optional file and the environment.
func Load() (*Config, error) {
	k := koanf.New(".")

	if err := k.Load(confmap.Provider(defaults(), "."), nil); err != nil {
		return nil, fmt.Errorf("config: load defaults: %w", err)
	}
	if path := os.Getenv(envPrefix + "CONFIG_FILE"); path != "" {
		if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
			return nil, fmt.Errorf("config: load %s: %w", path, err)
		}
	}
	if err := k.Load(env.Provider(envPrefix, ".", envKey), nil); err != nil {
		return nil, fmt.Errorf("config: load env: %w", err)
	}

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	return &cfg, nil
}

// envKey maps EFOY_HTTP__READ_TIMEOUT to http.read_timeout.
func envKey(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimPrefix(s, envPrefix)), "__", ".")
}

func (c *Config) validate() error {
	var errs []error
	switch c.Env {
	case "dev", "staging", "prod":
	default:
		errs = append(errs, fmt.Errorf("env must be dev, staging or prod, got %q", c.Env))
	}
	if c.HTTP.Addr == "" {
		errs = append(errs, errors.New("http.addr is required"))
	}
	switch c.Log.Format {
	case "json", "text":
	default:
		errs = append(errs, fmt.Errorf("log.format must be json or text, got %q", c.Log.Format))
	}
	return errors.Join(errs...)
}
