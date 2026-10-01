package config

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clearEnv unsets EFOY_* variables (e.g. exported from .env by make) for the
// duration of the test; t.Setenv restores the original values afterwards.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		key, value, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(key, envPrefix) {
			t.Setenv(key, value)
			require.NoError(t, os.Unsetenv(key))
		}
	}
}

func TestLoadDefaults(t *testing.T) {
	clearEnv(t)
	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "dev", cfg.Env)
	assert.Equal(t, ":8080", cfg.HTTP.Addr)
	assert.Equal(t, 15*time.Second, cfg.HTTP.ReadTimeout)
	assert.Equal(t, "json", cfg.Log.Format)
	assert.Equal(t, int32(20), cfg.Database.MaxConns)
}

func TestLoadEnvOverridesNestedKeys(t *testing.T) {
	clearEnv(t)
	t.Setenv("EFOY_ENV", "staging")
	t.Setenv("EFOY_AUTH__SIGNING_KEY", "pem")
	t.Setenv("EFOY_AUTH__TOTP_KEY", "key")
	t.Setenv("EFOY_HTTP__ADDR", ":9999")
	t.Setenv("EFOY_HTTP__READ_TIMEOUT", "7s")
	t.Setenv("EFOY_DATABASE__MAX_CONNS", "5")
	t.Setenv("EFOY_S3__USE_SSL", "true")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "staging", cfg.Env)
	assert.Equal(t, ":9999", cfg.HTTP.Addr)
	assert.Equal(t, 7*time.Second, cfg.HTTP.ReadTimeout)
	assert.Equal(t, int32(5), cfg.Database.MaxConns)
	assert.True(t, cfg.S3.UseSSL)
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	clearEnv(t)
	t.Setenv("EFOY_ENV", "production")
	t.Setenv("EFOY_LOG__FORMAT", "xml")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "env must be")
	assert.Contains(t, err.Error(), "log.format must be")
}

func TestEnvKey(t *testing.T) {
	assert.Equal(t, "http.read_timeout", envKey("EFOY_HTTP__READ_TIMEOUT"))
	assert.Equal(t, "env", envKey("EFOY_ENV"))
}

func TestLoadRequiresKeysOutsideDev(t *testing.T) {
	clearEnv(t)
	t.Setenv("EFOY_ENV", "staging")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth.signing_key is required")
	assert.Contains(t, err.Error(), "auth.totp_key is required")

	t.Setenv("EFOY_AUTH__SIGNING_KEY", "pem")
	t.Setenv("EFOY_AUTH__TOTP_KEY", "key")
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 15*time.Minute, cfg.Auth.AccessTTL)
	assert.Equal(t, 5, cfg.Auth.OTP.MaxAttempts)
}

func TestLoadRefusesConsoleSMSInProd(t *testing.T) {
	clearEnv(t)
	t.Setenv("EFOY_ENV", "prod")
	t.Setenv("EFOY_AUTH__SIGNING_KEY", "pem")
	t.Setenv("EFOY_AUTH__TOTP_KEY", "key")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sms.provider console")
}
