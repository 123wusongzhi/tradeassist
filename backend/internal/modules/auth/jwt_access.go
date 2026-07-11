package auth

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/pkg/authutil"
)

// AccessClaims is the JWT payload for short-lived access tokens.
type AccessClaims struct {
	Username     string `json:"username"`
	TokenType    string `json:"typ"`
	TenantID     int64  `json:"tenant_id"`
	SessionID    string `json:"session_id"`
	TokenVersion int    `json:"token_version"`
	jwt.RegisteredClaims
}

// KeySet holds active and previous JWT signing keys for rotation.
type KeySet struct {
	ActiveID       string
	ActiveSecret   []byte
	PreviousID     string
	PreviousSecret []byte
	GraceUntil     time.Time
}

// BuildKeySet resolves signing keys from config.
func BuildKeySet(cfg *config.Config) (*KeySet, error) {
	if cfg == nil {
		return nil, fmt.Errorf("jwt: nil config")
	}
	ks := &KeySet{}
	activeID := strings.TrimSpace(cfg.Auth.JWTActiveKeyID)
	activeSecret := strings.TrimSpace(cfg.Auth.JWTActiveSecret)
	if activeSecret == "" {
		activeSecret = strings.TrimSpace(cfg.JWTSecret)
	}
	if activeID == "" {
		activeID = "default"
	}
	if activeSecret == "" {
		return nil, fmt.Errorf("jwt: empty signing secret")
	}
	ks.ActiveID = activeID
	ks.ActiveSecret = []byte(activeSecret)

	prevID := strings.TrimSpace(cfg.Auth.JWTPreviousKeyID)
	prevSecret := strings.TrimSpace(cfg.Auth.JWTPreviousSecret)
	if prevID != "" && prevSecret != "" {
		ks.PreviousID = prevID
		ks.PreviousSecret = []byte(prevSecret)
		if cfg.Auth.JWTRotationGraceMinutes > 0 {
			ks.GraceUntil = time.Now().UTC().Add(time.Duration(cfg.Auth.JWTRotationGraceMinutes) * time.Minute)
		}
	}
	return ks, nil
}

// MintAccessToken issues a signed access JWT with kid and session binding.
func MintAccessToken(cfg *config.Config, ks *KeySet, input MintAccessInput) (string, time.Time, error) {
	if cfg == nil || ks == nil {
		return "", time.Time{}, fmt.Errorf("jwt: misconfigured")
	}
	ttl := cfg.AccessTokenTTL()
	exp := time.Now().UTC().Add(ttl)
	jti, err := authutil.NewOpaqueToken(16)
	if err != nil {
		return "", time.Time{}, err
	}
	claims := AccessClaims{
		Username:     input.Username,
		TokenType:    "access",
		TenantID:     input.TenantID,
		SessionID:    input.SessionID.String(),
		TokenVersion: input.TokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   input.UserID.String(),
			ID:        jti,
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	t.Header["kid"] = ks.ActiveID
	signed, err := t.SignedString(ks.ActiveSecret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}

// MintAccessInput binds access token to session and tenant.
type MintAccessInput struct {
	UserID       uuid.UUID
	Username     string
	TenantID     int64
	SessionID    uuid.UUID
	TokenVersion int
}

// ParseAccessToken validates access JWT and returns claims.
func ParseAccessToken(cfg *config.Config, ks *KeySet, tokenStr string) (*AccessClaims, error) {
	if cfg == nil {
		return nil, fmt.Errorf("jwt: nil config")
	}
	if ks == nil {
		var err error
		ks, err = BuildKeySet(cfg)
		if err != nil {
			return nil, err
		}
	}
	parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	t, err := parser.ParseWithClaims(tokenStr, &AccessClaims{}, func(t *jwt.Token) (interface{}, error) {
		kid, _ := t.Header["kid"].(string)
		kid = strings.TrimSpace(kid)
		if kid == "" {
			kid = ks.ActiveID
		}
		switch kid {
		case ks.ActiveID:
			return ks.ActiveSecret, nil
		case ks.PreviousID:
			if len(ks.PreviousSecret) == 0 {
				return nil, fmt.Errorf("jwt: unknown kid %q", kid)
			}
			if !ks.GraceUntil.IsZero() && time.Now().UTC().After(ks.GraceUntil) {
				return nil, fmt.Errorf("jwt: expired previous key %q", kid)
			}
			return ks.PreviousSecret, nil
		default:
			return nil, fmt.Errorf("jwt: unknown kid %q", kid)
		}
	})
	if err != nil {
		return nil, err
	}
	c, ok := t.Claims.(*AccessClaims)
	if !ok || !t.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}
	if c.TokenType != "" && c.TokenType != "access" {
		return nil, fmt.Errorf("jwt: wrong token type")
	}
	if strings.TrimSpace(c.Subject) == "" {
		return nil, fmt.Errorf("jwt: empty subject")
	}
	return c, nil
}

// LegacyMintToken issues JWT without session binding (legacy_local_storage mode).
func LegacyMintToken(cfg *config.Config, adminID uuid.UUID, username string) (string, time.Time, error) {
	ks, err := BuildKeySet(cfg)
	if err != nil {
		return "", time.Time{}, err
	}
	return MintAccessToken(cfg, ks, MintAccessInput{
		UserID:       adminID,
		Username:     username,
		TenantID:     0,
		SessionID:    uuid.Nil,
		TokenVersion: 1,
	})
}

// LegacyParseToken validates legacy or session-bound access tokens.
func LegacyParseToken(cfg *config.Config, tokenStr string) (*AccessClaims, error) {
	return ParseAccessToken(cfg, nil, tokenStr)
}
