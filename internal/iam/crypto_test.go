package iam

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blinge12/efoy/pkg/authz"
	"github.com/blinge12/efoy/pkg/clock"
	"github.com/blinge12/efoy/pkg/idgen"
)

func TestPasswordHashing(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	require.NoError(t, err)
	assert.Regexp(t, `^\$argon2id\$v=19\$m=65536,t=3,p=2\$[^$]+\$[^$]+$`, hash)

	ok, err := VerifyPassword("correct horse battery staple", hash)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = VerifyPassword("correct horse battery stapler", hash)
	require.NoError(t, err)
	assert.False(t, ok)

	_, err = VerifyPassword("x", "$2a$10$bcrypt-hash")
	assert.Error(t, err)
}

func TestSecretCipher(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	c, err := NewSecretCipher(key)
	require.NoError(t, err)

	ct, err := c.Encrypt([]byte("JBSWY3DPEHPK3PXP"))
	require.NoError(t, err)
	pt, err := c.Decrypt(ct)
	require.NoError(t, err)
	assert.Equal(t, "JBSWY3DPEHPK3PXP", string(pt))

	ct[len(ct)-1] ^= 1
	_, err = c.Decrypt(ct)
	assert.Error(t, err, "tampering is detected")

	_, err = NewSecretCipher(key[:16])
	assert.Error(t, err)
}

func newTestTokens(t *testing.T, kid string, clk clock.Clock) (*Tokens, ed25519.PrivateKey) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return NewTokens(TokenConfig{
		KeyID:     kid,
		Key:       priv,
		Issuer:    "efoy",
		Audience:  "efoy-api",
		AccessTTL: 15 * time.Minute,
		Clock:     clk,
	}), priv
}

func TestAccessTokens(t *testing.T) {
	clk := clock.NewFake(time.Date(2026, 10, 1, 6, 0, 0, 0, time.UTC))
	tokens, _ := newTestTokens(t, "k1", clk)
	user, session := idgen.New(), idgen.New()
	school := idgen.New()
	grants := []authz.Grant{
		{Role: authz.RoleGuardian, Scope: authz.ScopeGlobal},
		{Role: authz.RoleInstitutionAdmin, Scope: authz.ScopeInstitution, ScopeID: school},
	}

	tok, ttl, err := tokens.Issue(user, session, grants, "om")
	require.NoError(t, err)
	assert.Equal(t, 15*time.Minute, ttl)

	actor, err := tokens.Verify(tok)
	require.NoError(t, err)
	assert.Equal(t, authz.Actor{UserID: user, SessionID: session, Grants: grants, Language: "om"}, actor)

	clk.Advance(15*time.Minute + time.Second)
	_, err = tokens.Verify(tok)
	assert.Error(t, err, "expired")
}

func TestAccessTokensFromOtherKeysAreRejected(t *testing.T) {
	clk := clock.NewFake(time.Date(2026, 10, 1, 6, 0, 0, 0, time.UTC))
	ours, _ := newTestTokens(t, "k1", clk)
	theirs, _ := newTestTokens(t, "k1", clk) // same kid, different key

	tok, _, err := theirs.Issue(idgen.New(), idgen.New(), nil, "en")
	require.NoError(t, err)
	_, err = ours.Verify(tok)
	assert.Error(t, err)
}

func TestKeyRotationKeepsOldTokensValid(t *testing.T) {
	clk := clock.NewFake(time.Date(2026, 10, 1, 6, 0, 0, 0, time.UTC))
	old, oldPriv := newTestTokens(t, "2026-07", clk)
	tok, _, err := old.Issue(idgen.New(), idgen.New(), nil, "am")
	require.NoError(t, err)

	_, newPriv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	rotated := NewTokens(TokenConfig{
		KeyID:     "2026-10",
		Key:       newPriv,
		Previous:  map[string]ed25519.PublicKey{"2026-07": oldPriv.Public().(ed25519.PublicKey)},
		Issuer:    "efoy",
		Audience:  "efoy-api",
		AccessTTL: 15 * time.Minute,
		Clock:     clk,
	})
	_, err = rotated.Verify(tok)
	assert.NoError(t, err)

	rec := httptest.NewRecorder()
	rotated.JWKS(rec, httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	var jwks struct {
		Keys []map[string]string `json:"keys"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &jwks))
	require.Len(t, jwks.Keys, 2)
	assert.Equal(t, "2026-07", jwks.Keys[0]["kid"])
	assert.Equal(t, "OKP", jwks.Keys[1]["kty"])
	assert.Equal(t, "Ed25519", jwks.Keys[1]["crv"])
	x, err := base64.RawURLEncoding.DecodeString(jwks.Keys[1]["x"])
	require.NoError(t, err)
	assert.Equal(t, []byte(newPriv.Public().(ed25519.PublicKey)), x)
}

func TestKeyPairPEMRoundTrip(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	privPEM, pubPEM, err := EncodeKeyPair(priv)
	require.NoError(t, err)

	parsed, err := ParsePrivateKey(privPEM)
	require.NoError(t, err)
	assert.Equal(t, priv, parsed)
	pub, err := ParsePublicKey(pubPEM)
	require.NoError(t, err)
	assert.Equal(t, priv.Public(), pub)

	_, err = ParsePrivateKey("not pem")
	assert.Error(t, err)
}

func TestLoadKeyringNeedsKeysOutsideDev(t *testing.T) {
	cfg := testAuthConfig()
	_, err := LoadKeyring(cfg, "staging", clock.Real{}, discardLogger())
	assert.Error(t, err)

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	cfg.SigningKey, _, err = EncodeKeyPair(priv)
	require.NoError(t, err)
	cfg.TOTPKey = base64.StdEncoding.EncodeToString(make([]byte, 32))
	_, err = LoadKeyring(cfg, "staging", clock.Real{}, discardLogger())
	assert.NoError(t, err)
}
