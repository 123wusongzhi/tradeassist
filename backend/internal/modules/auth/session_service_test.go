package auth

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/authutil"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func TestRefreshTokenConcurrentRotation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&AuthSession{}, &AuthRefreshToken{}, &AuthLoginAttempt{}, &admin.AdminUser{}); err != nil {
		t.Fatal(err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("test-password-123"), bcrypt.DefaultCost)
	uid := uuid.New()
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: uid},
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: string(hash),
		Role:         "admin",
		Status:       "active",
	}).Error; err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		JWTSecret: "test-jwt-secret-with-enough-length-32",
		Auth: config.AuthConfig{
			SessionMode:           config.AuthSessionModeSecure,
			AccessTokenTTLMinutes: 15,
			RefreshTokenTTLDays:   7,
		},
	}
	svc := &SessionService{Cfg: cfg, DB: db, Admins: &admin.Store{DB: db}}
	res, err := svc.CreateSession(context.Background(), "test@example.com", "test-password-123", "127.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	raw := res.RefreshToken
	var okCount int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.RotateRefresh(context.Background(), raw, "127.0.0.1", "test")
			if err == nil {
				atomic.AddInt32(&okCount, 1)
			}
		}()
	}
	wg.Wait()
	if okCount != 1 {
		t.Fatalf("expected exactly 1 successful rotation, got %d", okCount)
	}
	var active int64
	db.Model(&AuthRefreshToken{}).Where("status = ?", RefreshStatusActive).Count(&active)
	if active != 1 {
		t.Fatalf("expected 1 active refresh token, got %d", active)
	}
}

func TestHashTokenStable(t *testing.T) {
	a := authutil.HashToken("abc", "pepper")
	b := authutil.HashToken("abc", "pepper")
	if a != b {
		t.Fatal("hash not stable")
	}
}

func TestLoginGuardLockout(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&AuthLoginAttempt{}); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Auth: config.AuthConfig{
			LoginMaxAttempts:   3,
			LoginWindowMinutes: 15,
			AccountLockMinutes: 30,
		},
	}
	g := &LoginGuard{Cfg: cfg, DB: db}
	for i := 0; i < 3; i++ {
		_ = g.RecordFailure(context.Background(), "user@example.com", "1.2.3.4")
	}
	if err := g.CheckAllowed(context.Background(), "user@example.com", "1.2.3.4"); err == nil {
		t.Fatal("expected lockout")
	}
	_ = g.ClearFailures(context.Background(), "user@example.com", "1.2.3.4")
	if err := g.CheckAllowed(context.Background(), "user@example.com", "1.2.3.4"); err != nil {
		t.Fatal("expected unlocked after clear")
	}
}
