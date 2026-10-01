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
	Auth     Auth     `koanf:"auth"`
	SMS      SMS      `koanf:"sms"`
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
	URL      string `koanf:"url"`
	Replicas int    `koanf:"replicas"` // JetStream stream replicas: 1 on a single node, 3 in prod
}

type S3 struct {
	Endpoint  string `koanf:"endpoint"` // host:port the services use
	AccessKey string `koanf:"access_key"`
	SecretKey string `koanf:"secret_key"`
	UseSSL    bool   `koanf:"use_ssl"`
	Region    string `koanf:"region"`
	// PublicURL is the base URL apps and browsers use for pre-signed URLs.
	PublicURL       string `koanf:"public_url"`
	DocumentsBucket string `koanf:"documents_bucket"`
}

type OSRM struct {
	URL string `koanf:"url"`
}

// Auth configures tokens, sessions and login (design doc 13.2).
type Auth struct {
	Issuer          string        `koanf:"issuer"`
	Audience        string        `koanf:"audience"`
	AccessTTL       time.Duration `koanf:"access_ttl"`
	RefreshTTL      time.Duration `koanf:"refresh_ttl"`       // sliding, mobile apps
	StaffSessionTTL time.Duration `koanf:"staff_session_ttl"` // absolute, web portals
	// SigningKey is the active Ed25519 private key (PKCS#8 PEM) for access tokens.
	SigningKey   string `koanf:"signing_key"`
	SigningKeyID string `koanf:"signing_key_id"`
	// PreviousPublicKey (PKIX PEM) keeps tokens signed before a key rotation valid.
	PreviousPublicKey string `koanf:"previous_public_key"`
	PreviousKeyID     string `koanf:"previous_key_id"`
	// TOTPKey is a base64 32-byte AES-256-GCM key for users.totp_secret_enc.
	TOTPKey string `koanf:"totp_key"`
	OTP     OTP    `koanf:"otp"`
}

// OTP configures SMS one-time passwords (FR-IAM-1).
type OTP struct {
	TTL         time.Duration `koanf:"ttl"`
	MaxAttempts int           `koanf:"max_attempts"`
	RateLimit   int           `koanf:"rate_limit"` // requests per phone per RateWindow
	RateWindow  time.Duration `koanf:"rate_window"`
	ResendAfter time.Duration `koanf:"resend_after"`
}

type SMS struct {
	Provider string `koanf:"provider"` // console (dev fake); AfroMessage/GeezSMS from day 5
}

func defaults() map[string]any {
	return map[string]any{
		"env":                    "dev",
		"http.addr":              ":8080",
		"http.read_timeout":      "15s",
		"http.write_timeout":     "30s",
		"http.idle_timeout":      "120s",
		"http.shutdown_timeout":  "20s",
		"log.level":              "info",
		"log.format":             "json",
		"database.url":           "postgres://efoy:efoy@localhost:5432/efoy?sslmode=disable",
		"database.max_conns":     20,
		"redis.addr":             "localhost:6379",
		"redis.db":               0,
		"nats.url":               "nats://localhost:4222",
		"s3.endpoint":            "localhost:9000",
		"s3.region":              "us-east-1",
		"s3.public_url":          "http://localhost:9000",
		"s3.documents_bucket":    "efoy-documents",
		"nats.replicas":          1,
		"s3.use_ssl":             false,
		"osrm.url":               "http://localhost:5000",
		"auth.issuer":            "efoy",
		"auth.audience":          "efoy-api",
		"auth.access_ttl":        "15m",
		"auth.refresh_ttl":       "720h",
		"auth.staff_session_ttl": "12h",
		"auth.signing_key_id":    "dev",
		"auth.otp.ttl":           "5m",
		"auth.otp.max_attempts":  5,
		"auth.otp.rate_limit":    3,
		"auth.otp.rate_window":   "10m",
		"auth.otp.resend_after":  "60s",
		"sms.provider":           "console",
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
	if c.Env != "dev" {
		// Dev falls back to fixed, publicly known keys; nothing else may.
		if c.Auth.SigningKey == "" {
			errs = append(errs, errors.New("auth.signing_key is required outside dev"))
		}
		if c.Auth.TOTPKey == "" {
			errs = append(errs, errors.New("auth.totp_key is required outside dev"))
		}
	}
	if c.Env == "prod" && c.SMS.Provider == "console" {
		errs = append(errs, errors.New("sms.provider console is not allowed in prod"))
	}
	if c.Auth.AccessTTL <= 0 || c.Auth.RefreshTTL <= 0 || c.Auth.StaffSessionTTL <= 0 {
		errs = append(errs, errors.New("auth TTLs must be positive"))
	}
	if c.Auth.OTP.MaxAttempts < 1 || c.Auth.OTP.RateLimit < 1 {
		errs = append(errs, errors.New("auth.otp.max_attempts and auth.otp.rate_limit must be at least 1"))
	}
	return errors.Join(errs...)
}
