package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
)

// Claims is kept for backward compatibility; prefer AccessClaims.
type Claims = AccessClaims

// MintToken issues a signed JWT for the given admin (legacy path).
func MintToken(cfg *config.Config, adminID uuid.UUID, username string) (string, time.Time, error) {
	return LegacyMintToken(cfg, adminID, username)
}

// ParseToken validates the token and returns claims.
func ParseToken(cfg *config.Config, tokenStr string) (*Claims, error) {
	return LegacyParseToken(cfg, tokenStr)
}

// RegisteredClaims alias for tests referencing old jwt.go.
var _ = jwt.RegisteredClaims{}
