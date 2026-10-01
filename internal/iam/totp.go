package iam

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

const totpIssuer = "Efoy"

var totpOpts = totp.ValidateOpts{
	Period:    30,
	Skew:      1, // accept the previous and next 30 s step for clock drift
	Digits:    otp.DigitsSix,
	Algorithm: otp.AlgorithmSHA1, // what authenticator apps support universally
}

// SecretCipher encrypts TOTP secrets at rest (users.totp_secret_enc) with
// AES-256-GCM. The nonce is prepended to the ciphertext.
type SecretCipher struct {
	aead cipher.AEAD
}

func NewSecretCipher(key []byte) (*SecretCipher, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("iam: totp key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &SecretCipher{aead: aead}, nil
}

func (c *SecretCipher) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return c.aead.Seal(nonce, nonce, plaintext, nil), nil
}

func (c *SecretCipher) Decrypt(ciphertext []byte) ([]byte, error) {
	n := c.aead.NonceSize()
	if len(ciphertext) <= n {
		return nil, errors.New("iam: totp ciphertext too short")
	}
	return c.aead.Open(nil, ciphertext[:n], ciphertext[n:], nil)
}

// NewTOTPSecret creates a secret for accountName and the otpauth:// URL that
// authenticator apps import (usually shown as a QR code).
func NewTOTPSecret(accountName string) (secret, url string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      totpIssuer,
		AccountName: accountName,
		Period:      totpOpts.Period,
		Digits:      totpOpts.Digits,
		Algorithm:   totpOpts.Algorithm,
	})
	if err != nil {
		return "", "", err
	}
	return key.Secret(), key.URL(), nil
}

// ValidateTOTP checks a 6-digit code against secret at time at.
func ValidateTOTP(code, secret string, at time.Time) bool {
	ok, err := totp.ValidateCustom(code, secret, at, totpOpts)
	return err == nil && ok
}
