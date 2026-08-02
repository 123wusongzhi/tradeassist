package auth

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/pkg/authutil"
)

var jwtRotationState = struct {
	sync.Mutex
	started map[string]time.Time
}{started: make(map[string]time.Time)}

func resetJWTRotationStateForTest() {
	jwtRotationState.Lock()
	defer jwtRotationState.Unlock()
	jwtRotationState.started = make(map[string]time.Time)
}

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
	// A previous key is only usable during an explicitly positive grace period.
	// Keeping it out of the key set otherwise makes a disabled/misconfigured
	// rotation fail closed instead of accepting old tokens indefinitely.
	if prevID != "" && prevSecret != "" && cfg.Auth.JWTRotationGraceMinutes > 0 {
		ks.PreviousID = prevID
		ks.PreviousSecret = []byte(prevSecret)
		// The grace clock belongs to the configured keyring transition, not to
		// an individual request. Rebuilding a KeySet must never extend it.
		started, err := jwtRotationStartedAt(cfg, activeID, activeSecret, prevID, prevSecret)
		if err != nil {
			return nil, err
		}
		ks.GraceUntil = started.Add(time.Duration(cfg.Auth.JWTRotationGraceMinutes) * time.Minute)
	}
	return ks, nil
}

func jwtRotationStartedAt(cfg *config.Config, activeID, activeSecret, previousID, previousSecret string) (time.Time, error) {
	if raw := strings.TrimSpace(cfg.Auth.JWTRotationStartedAt); raw != "" {
		started, err := time.Parse(time.RFC3339, raw)
		if err != nil || !strings.HasSuffix(raw, "Z") || started.After(time.Now().UTC()) {
			return time.Time{}, fmt.Errorf("jwt: invalid rotation start")
		}
		return started.UTC(), nil
	}
	// Development/test fallback only. Production validation requires explicit time.
	fingerprint := sha256.Sum256([]byte(activeID + "\x00" + activeSecret + "\x00" + previousID + "\x00" + previousSecret))
	key := string(fingerprint[:])
	jwtRotationState.Lock()
	defer jwtRotationState.Unlock()
	started := jwtRotationState.started[key]
	if started.IsZero() {
		started = time.Now().UTC()
		jwtRotationState.started[key] = started
	}
	return started, nil
}

// MintAccessToken issues a signed access JWT with kid and session binding.
func MintAccessToken(cfg *config.Config, ks *KeySet, input MintAccessInput) (string, time.Time, error) {
	if cfg == nil || ks == nil {
		return "", time.Time{}, fmt.Errorf("jwt: misconfigured")
	}
	if cfg.UsesSecureSession() && input.SessionID == uuid.Nil {
		return "", time.Time{}, fmt.Errorf("jwt: secure access token requires session binding")
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
			if len(ks.PreviousSecret) == 0 || ks.GraceUntil.IsZero() {
				return nil, fmt.Errorf("jwt: unknown kid %q", kid)
			}
			if !time.Now().UTC().Before(ks.GraceUntil) {
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
	if c.TokenType != "access" && (cfg.UsesSecureSession() || c.TokenType != "") {
		return nil, fmt.Errorf("jwt: wrong token type")
	}
	if strings.TrimSpace(c.Subject) == "" {
		return nil, fmt.Errorf("jwt: empty subject")
	}
	return c, nil
}

// LegacyMintToken issues JWT without session binding (legacy_local_storage mode).
func LegacyMintToken(cfg *config.Config, adminID uuid.UUID, username string) (string, time.Time, error) {
	return legacyMintTokenForTenant(cfg, adminID, username, 0)
}

// legacyMintTokenForTenant preserves tenant scope while supporting the
// transitional local-storage session mode.
func legacyMintTokenForTenant(cfg *config.Config, adminID uuid.UUID, username string, tenantID int64) (string, time.Time, error) {
	ks, err := BuildKeySet(cfg)
	if err != nil {
		return "", time.Time{}, err
	}
	return MintAccessToken(cfg, ks, MintAccessInput{
		UserID:       adminID,
		Username:     username,
		TenantID:     tenantID,
		SessionID:    uuid.Nil,
		TokenVersion: 1,
	})
}

// LegacyParseToken validates legacy or session-bound access tokens.
func LegacyParseToken(cfg *config.Config, tokenStr string) (*AccessClaims, error) {
	return ParseAccessToken(cfg, nil, tokenStr)
}
