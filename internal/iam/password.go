package iam

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

// argon2id parameters (OWASP: m=64 MiB, t=3, p=2 is well above the minimum).
const (
	argonMemory  = 64 * 1024
	argonTime    = 3
	argonThreads = 2
	argonKeyLen  = 32
	argonSaltLen = 16

	// MinPasswordLength applies to staff portal passwords.
	MinPasswordLength = 12
)

var b64 = base64.RawStdEncoding

// HashPassword returns an argon2id hash in PHC string format.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("iam: password salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// VerifyPassword checks password against a PHC-encoded argon2id hash.
func VerifyPassword(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("iam: unsupported password hash format")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, errors.New("iam: unsupported argon2 version")
	}
	var memory, iterations uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &threads); err != nil {
		return false, fmt.Errorf("iam: argon2 parameters: %w", err)
	}
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("iam: argon2 salt: %w", err)
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil || len(want) == 0 || len(want) > 128 {
		return false, errors.New("iam: argon2 key")
	}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, threads, uint32(len(want))) //nolint:gosec // len checked above
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

var (
	dummyHashOnce sync.Once
	dummyHash     string
)

// burnPasswordCheck spends the same time as a real verification so unknown
// emails cannot be told apart by response time.
func burnPasswordCheck(password string) {
	dummyHashOnce.Do(func() {
		dummyHash, _ = HashPassword("efoy-timing-equaliser")
	})
	_, _ = VerifyPassword(password, dummyHash)
}

// GeneratePassword returns a random URL-safe password of about 128 bits.
func GeneratePassword() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
