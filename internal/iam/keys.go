package iam

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log/slog"

	"github.com/blinge12/efoy/internal/config"
	"github.com/blinge12/efoy/pkg/clock"
)

// Keyring holds the secrets the iam domain needs, loaded from config.
type Keyring struct {
	Tokens    *Tokens
	TOTP      *SecretCipher
	OTPPepper []byte
}

// Dev-only key material. config.validate refuses to start any environment
// other than dev without real keys, so these never protect real data. They
// are fixed (not random per process) so tokens and TOTP secrets survive the
// hot-reload restarts of `make dev`.
const (
	devSigningSeed = "efoy-dev-only-signing-key-do-not-use"
	devTOTPSeed    = "efoy-dev-only-totp-key-do-not-use"
)

func devKey(seed string) []byte {
	sum := sha256.Sum256([]byte(seed))
	return sum[:]
}

// LoadKeyring parses the signing and encryption keys from cfg.
func LoadKeyring(cfg config.Auth, env string, clk clock.Clock, log *slog.Logger) (*Keyring, error) {
	var signing ed25519.PrivateKey
	if cfg.SigningKey == "" {
		if env != "dev" {
			return nil, fmt.Errorf("iam: auth.signing_key is required in %s", env)
		}
		log.Warn("auth.signing_key not set: using the fixed dev signing key")
		signing = ed25519.NewKeyFromSeed(devKey(devSigningSeed))
	} else {
		k, err := ParsePrivateKey(cfg.SigningKey)
		if err != nil {
			return nil, err
		}
		signing = k
	}

	previous := map[string]ed25519.PublicKey{}
	if cfg.PreviousPublicKey != "" {
		pub, err := ParsePublicKey(cfg.PreviousPublicKey)
		if err != nil {
			return nil, err
		}
		previous[cfg.PreviousKeyID] = pub
	}

	var totpKey []byte
	if cfg.TOTPKey == "" {
		if env != "dev" {
			return nil, fmt.Errorf("iam: auth.totp_key is required in %s", env)
		}
		log.Warn("auth.totp_key not set: using the fixed dev TOTP key")
		totpKey = devKey(devTOTPSeed)
	} else {
		k, err := base64.StdEncoding.DecodeString(cfg.TOTPKey)
		if err != nil {
			return nil, fmt.Errorf("iam: auth.totp_key must be base64: %w", err)
		}
		totpKey = k
	}
	cipher, err := NewSecretCipher(totpKey)
	if err != nil {
		return nil, err
	}

	// Derive a separate key for OTP hashing instead of reusing the AES key.
	m := hmac.New(sha256.New, totpKey)
	m.Write([]byte("efoy/otp-pepper"))

	return &Keyring{
		Tokens: NewTokens(TokenConfig{
			KeyID:     cfg.SigningKeyID,
			Key:       signing,
			Previous:  previous,
			Issuer:    cfg.Issuer,
			Audience:  cfg.Audience,
			AccessTTL: cfg.AccessTTL,
			Clock:     clk,
		}),
		TOTP:      cipher,
		OTPPepper: m.Sum(nil),
	}, nil
}
