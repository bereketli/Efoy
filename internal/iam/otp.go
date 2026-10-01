package iam

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/blinge12/efoy/internal/config"
)

// OTPStore keeps one-time passwords in Redis (design doc 11.3: otp:<phone>).
// Only an HMAC of the code is stored, never the code itself.
type OTPStore struct {
	rdb    redis.UniversalClient
	cfg    config.OTP
	pepper []byte
}

func NewOTPStore(rdb redis.UniversalClient, cfg config.OTP, pepper []byte) *OTPStore {
	return &OTPStore{rdb: rdb, cfg: cfg, pepper: pepper}
}

func otpKey(phone string) string { return "otp:" + phone }

func (s *OTPStore) hash(phone, code string) string {
	m := hmac.New(sha256.New, s.pepper)
	m.Write([]byte(phone + ":" + code))
	return hex.EncodeToString(m.Sum(nil))
}

// Issue creates a fresh 6-digit code for phone, replacing any previous one.
func (s *OTPStore) Issue(ctx context.Context, phone string) (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", fmt.Errorf("iam: otp random: %w", err)
	}
	code := fmt.Sprintf("%06d", n.Int64())
	key := otpKey(phone)
	_, err = s.rdb.TxPipelined(ctx, func(p redis.Pipeliner) error {
		p.Del(ctx, key)
		p.HSet(ctx, key, "h", s.hash(phone, code), "a", 0)
		p.Expire(ctx, key, s.cfg.TTL)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("iam: store otp: %w", err)
	}
	return code, nil
}

// Discard deletes the pending code for phone.
func (s *OTPStore) Discard(ctx context.Context, phone string) error {
	return s.rdb.Del(ctx, otpKey(phone)).Err()
}

// verifyScript reads the stored hash and counts the attempt atomically, so an
// expiring key can never be recreated without a TTL.
var verifyScript = redis.NewScript(`
local h = redis.call('HGET', KEYS[1], 'h')
if not h then return false end
local a = redis.call('HINCRBY', KEYS[1], 'a', 1)
return {h, a}
`)

// Verify consumes one attempt. The code is deleted on success and after the
// last allowed attempt.
func (s *OTPStore) Verify(ctx context.Context, phone, code string) error {
	key := otpKey(phone)
	res, err := verifyScript.Run(ctx, s.rdb, []string{key}).Slice()
	if errors.Is(err, redis.Nil) {
		return ErrOTPInvalid
	}
	if err != nil {
		return fmt.Errorf("iam: verify otp: %w", err)
	}
	if len(res) != 2 {
		return errors.New("iam: verify otp: unexpected script result")
	}
	want, ok1 := res[0].(string)
	attempts, ok2 := res[1].(int64)
	if !ok1 || !ok2 {
		return errors.New("iam: verify otp: unexpected script result")
	}

	maxAttempts := int64(s.cfg.MaxAttempts)
	if attempts > maxAttempts {
		_ = s.Discard(ctx, phone)
		return ErrOTPAttempts
	}
	if subtle.ConstantTimeCompare([]byte(want), []byte(s.hash(phone, code))) != 1 {
		if attempts == maxAttempts {
			_ = s.Discard(ctx, phone)
			return ErrOTPAttempts
		}
		return ErrOTPInvalid
	}
	return s.Discard(ctx, phone)
}

// MarkTOTPUsed records a TOTP code as spent for its validity window and
// reports false if it was already used (replay protection).
func (s *OTPStore) MarkTOTPUsed(ctx context.Context, userID uuid.UUID, code string) (bool, error) {
	return s.rdb.SetNX(ctx, "totp:"+userID.String()+":"+code, 1, 90*time.Second).Result()
}

// Limiter is a fixed-window rate limiter on rl:<scope>:<id> (design doc 11.3).
type Limiter struct {
	rdb redis.UniversalClient
}

func NewLimiter(rdb redis.UniversalClient) *Limiter { return &Limiter{rdb: rdb} }

var limitScript = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[1]) end
return {n, redis.call('PTTL', KEYS[1])}
`)

// Allow counts one event and reports whether it is within limit per window.
// When it is not, retryAfter says when the window resets.
func (l *Limiter) Allow(ctx context.Context, scope, id string, limit int, window time.Duration) (ok bool, retryAfter time.Duration, err error) {
	res, err := limitScript.Run(ctx, l.rdb, []string{"rl:" + scope + ":" + id}, window.Milliseconds()).Slice()
	if err != nil {
		return false, 0, fmt.Errorf("iam: rate limit: %w", err)
	}
	if len(res) != 2 {
		return false, 0, errors.New("iam: rate limit: unexpected script result")
	}
	count, ok1 := res[0].(int64)
	pttl, ok2 := res[1].(int64)
	if !ok1 || !ok2 {
		return false, 0, errors.New("iam: rate limit: unexpected script result")
	}
	if count > int64(limit) {
		return false, time.Duration(pttl) * time.Millisecond, nil
	}
	return true, 0, nil
}
