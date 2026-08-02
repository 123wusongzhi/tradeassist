package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
)

func TestSecureAccessTokenRequiresSessionBinding(t *testing.T) {
	cfg := &config.Config{JWTSecret: "secure-session-mint-test-secret", Auth: config.AuthConfig{SessionMode: config.AuthSessionModeSecure}}
	keys, err := BuildKeySet(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := MintAccessToken(cfg, keys, MintAccessInput{UserID: uuid.New(), TenantID: 1, SessionID: uuid.Nil}); err == nil {
		t.Fatal("secure access token was minted without a session binding")
	}
}

func TestPreviousJWTKeyGraceHasStableStart(t *testing.T) {
	resetJWTRotationStateForTest()
	cfg := &config.Config{JWTSecret: "active", Auth: config.AuthConfig{JWTActiveKeyID: "new", JWTActiveSecret: "active", JWTPreviousKeyID: "old", JWTPreviousSecret: "previous", JWTRotationGraceMinutes: 1}}
	first, err := BuildKeySet(cfg)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	second, err := BuildKeySet(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !first.GraceUntil.Equal(second.GraceUntil) {
		t.Fatalf("grace changed: %s -> %s", first.GraceUntil, second.GraceUntil)
	}
	claims := AccessClaims{TokenType: "access", RegisteredClaims: jwt.RegisteredClaims{Subject: "00000000-0000-0000-0000-000000000001", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
	old := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	old.Header["kid"] = "old"
	oldToken, err := old.SignedString([]byte("previous"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseAccessToken(cfg, second, oldToken); err != nil {
		t.Fatalf("previous key inside grace: %v", err)
	}
	jwtRotationState.Lock()
	for key := range jwtRotationState.started {
		jwtRotationState.started[key] = time.Now().UTC().Add(-2 * time.Minute)
	}
	jwtRotationState.Unlock()
	expired, err := BuildKeySet(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !expired.GraceUntil.Before(time.Now().UTC()) {
		t.Fatal("expected expired grace")
	}
	if _, err := ParseAccessToken(cfg, expired, oldToken); err == nil {
		t.Fatal("previous key was accepted after grace")
	}
}

func TestJWTExplicitRotationStartSurvivesStateReset(t *testing.T) {
	cfg := &config.Config{JWTSecret: "active", Auth: config.AuthConfig{JWTActiveKeyID: "new", JWTActiveSecret: "active", JWTPreviousKeyID: "old", JWTPreviousSecret: "previous", JWTRotationGraceMinutes: 60, JWTRotationStartedAt: "2026-01-02T03:04:05Z"}}
	first, err := BuildKeySet(cfg)
	if err != nil {
		t.Fatal(err)
	}
	resetJWTRotationStateForTest()
	second, err := BuildKeySet(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !first.GraceUntil.Equal(second.GraceUntil) {
		t.Fatal("explicit rotation time changed across reset")
	}
}

func TestJWTActiveSecretChangesFallbackRotationStart(t *testing.T) {
	resetJWTRotationStateForTest()
	base := config.AuthConfig{JWTActiveKeyID: "new", JWTPreviousKeyID: "old", JWTPreviousSecret: "previous", JWTRotationGraceMinutes: 1}
	cfg1 := &config.Config{JWTSecret: "a", Auth: base}
	cfg1.Auth.JWTActiveSecret = "a"
	if _, err := BuildKeySet(cfg1); err != nil {
		t.Fatal(err)
	}
	jwtRotationState.Lock()
	before := len(jwtRotationState.started)
	jwtRotationState.Unlock()
	cfg2 := &config.Config{JWTSecret: "b", Auth: base}
	cfg2.Auth.JWTActiveSecret = "b"
	if _, err := BuildKeySet(cfg2); err != nil {
		t.Fatal(err)
	}
	jwtRotationState.Lock()
	after := len(jwtRotationState.started)
	jwtRotationState.Unlock()
	if after != before+1 {
		t.Fatalf("active secret change did not create new fallback state: %d -> %d", before, after)
	}
}

func TestPreviousJWTKeyWithoutPositiveGraceIsRejected(t *testing.T) {
	cfg := &config.Config{JWTSecret: "active", Auth: config.AuthConfig{
		JWTActiveKeyID:          "new",
		JWTActiveSecret:         "active",
		JWTPreviousKeyID:        "old",
		JWTPreviousSecret:       "previous",
		JWTRotationGraceMinutes: 0,
	}}
	ks, err := BuildKeySet(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if ks.PreviousID != "" || len(ks.PreviousSecret) != 0 {
		t.Fatal("previous key must not be installed without a positive grace period")
	}
	claims := AccessClaims{TokenType: "access", RegisteredClaims: jwt.RegisteredClaims{
		Subject:   "00000000-0000-0000-0000-000000000001",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}}
	old := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	old.Header["kid"] = "old"
	oldToken, err := old.SignedString([]byte("previous"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseAccessToken(cfg, ks, oldToken); err == nil {
		t.Fatal("previous key was accepted without a positive grace period")
	}
}
