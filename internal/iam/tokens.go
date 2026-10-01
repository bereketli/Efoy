package iam

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/blinge12/efoy/pkg/authz"
	"github.com/blinge12/efoy/pkg/clock"
	"github.com/blinge12/efoy/pkg/httpx"
	"github.com/blinge12/efoy/pkg/idgen"
)

// Claims are the access token claims (design doc 12.1: sub, roles, scopes,
// lang, exp). sid identifies the refresh session so logout can revoke it.
type Claims struct {
	jwt.RegisteredClaims
	SessionID string   `json:"sid"`
	Roles     []string `json:"roles"`
	Scopes    []string `json:"scopes"`
	Lang      string   `json:"lang,omitempty"`
}

// Tokens issues and verifies EdDSA access tokens. It keeps the active
// signing key plus any previous public keys so a rotation does not log
// everyone out; all of them are published as a JWKS.
type Tokens struct {
	kid      string
	priv     ed25519.PrivateKey
	pubs     map[string]ed25519.PublicKey
	issuer   string
	audience string
	ttl      time.Duration
	clock    clock.Clock
}

// TokenConfig configures Tokens.
type TokenConfig struct {
	KeyID     string
	Key       ed25519.PrivateKey
	Previous  map[string]ed25519.PublicKey
	Issuer    string
	Audience  string
	AccessTTL time.Duration
	Clock     clock.Clock
}

func NewTokens(cfg TokenConfig) *Tokens {
	pubs := map[string]ed25519.PublicKey{}
	for kid, k := range cfg.Previous {
		pubs[kid] = k
	}
	pubs[cfg.KeyID] = cfg.Key.Public().(ed25519.PublicKey)
	return &Tokens{
		kid:      cfg.KeyID,
		priv:     cfg.Key,
		pubs:     pubs,
		issuer:   cfg.Issuer,
		audience: cfg.Audience,
		ttl:      cfg.AccessTTL,
		clock:    cfg.Clock,
	}
}

// Issue signs an access token for the user and session.
func (t *Tokens) Issue(userID, sessionID uuid.UUID, grants []authz.Grant, lang string) (string, time.Duration, error) {
	now := t.clock.Now()
	actor := authz.Actor{Grants: grants}
	roles := make([]string, 0, len(grants))
	for _, r := range actor.Roles() {
		roles = append(roles, string(r))
	}
	scopes := make([]string, 0, len(grants))
	for _, g := range grants {
		scopes = append(scopes, g.String())
	}
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        idgen.New().String(),
			Issuer:    t.issuer,
			Subject:   userID.String(),
			Audience:  jwt.ClaimStrings{t.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(t.ttl)),
		},
		SessionID: sessionID.String(),
		Roles:     roles,
		Scopes:    scopes,
		Lang:      lang,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	tok.Header["kid"] = t.kid
	signed, err := tok.SignedString(t.priv)
	if err != nil {
		return "", 0, fmt.Errorf("iam: sign access token: %w", err)
	}
	return signed, t.ttl, nil
}

// Verify checks signature, issuer, audience and expiry, and returns the actor.
// It implements authz.Verifier.
func (t *Tokens) Verify(token string) (authz.Actor, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims, func(tok *jwt.Token) (any, error) {
		kid, _ := tok.Header["kid"].(string)
		key, ok := t.pubs[kid]
		if !ok {
			return nil, fmt.Errorf("unknown key id %q", kid)
		}
		return key, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithIssuer(t.issuer),
		jwt.WithAudience(t.audience),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(t.clock.Now),
	)
	if err != nil {
		return authz.Actor{}, err
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return authz.Actor{}, fmt.Errorf("iam: token subject: %w", err)
	}
	sessionID, err := uuid.Parse(claims.SessionID)
	if err != nil {
		return authz.Actor{}, fmt.Errorf("iam: token sid: %w", err)
	}
	grants := make([]authz.Grant, 0, len(claims.Scopes))
	for _, s := range claims.Scopes {
		g, err := authz.ParseGrant(s)
		if err != nil {
			return authz.Actor{}, err
		}
		grants = append(grants, g)
	}
	return authz.Actor{UserID: userID, SessionID: sessionID, Grants: grants, Language: claims.Lang}, nil
}

type jwk struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	Use string `json:"use"`
}

// JWKS serves the public keys at /.well-known/jwks.json so other services and
// the web console can verify access tokens.
func (t *Tokens) JWKS(w http.ResponseWriter, _ *http.Request) {
	kids := make([]string, 0, len(t.pubs))
	for kid := range t.pubs {
		kids = append(kids, kid)
	}
	sort.Strings(kids)
	keys := make([]jwk, 0, len(kids))
	for _, kid := range kids {
		keys = append(keys, jwk{
			Kty: "OKP",
			Crv: "Ed25519",
			X:   base64.RawURLEncoding.EncodeToString(t.pubs[kid]),
			Kid: kid,
			Alg: "EdDSA",
			Use: "sig",
		})
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	httpx.WriteJSON(w, http.StatusOK, map[string][]jwk{"keys": keys})
}

// ParsePrivateKey decodes a PKCS#8 PEM Ed25519 private key.
func ParsePrivateKey(pemText string) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemText))
	if block == nil {
		return nil, errors.New("iam: signing key is not PEM")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("iam: signing key: %w", err)
	}
	priv, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("iam: signing key is not Ed25519")
	}
	return priv, nil
}

// ParsePublicKey decodes a PKIX PEM Ed25519 public key.
func ParsePublicKey(pemText string) (ed25519.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemText))
	if block == nil {
		return nil, errors.New("iam: public key is not PEM")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("iam: public key: %w", err)
	}
	pub, ok := key.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("iam: public key is not Ed25519")
	}
	return pub, nil
}

// EncodeKeyPair returns PEM encodings of a new Ed25519 key pair, for the
// `core-api gen-keys` command.
func EncodeKeyPair(priv ed25519.PrivateKey) (privPEM, pubPEM string, err error) {
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return "", "", err
	}
	pubDER, err := x509.MarshalPKIXPublicKey(priv.Public())
	if err != nil {
		return "", "", err
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})), nil
}
