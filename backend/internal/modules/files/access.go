package files

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/repository"
	"gorm.io/gorm"
)

// ObjectAccessService authorizes private file downloads.
type ObjectAccessService struct {
	DB  *gorm.DB
	Cfg *config.Config
}

// SignedURL carries a short-lived download authorization.
type SignedURL struct {
	AssetID   string `json:"assetId"`
	ExpiresAt int64  `json:"expiresAt"`
	Token     string `json:"token"`
	Path      string `json:"path"`
}

// CreateDownloadURL issues a short-lived signed download path.
func (s *ObjectAccessService) CreateDownloadURL(c *gin.Context, assetID uuid.UUID, kind string) (*SignedURL, error) {
	if s == nil || s.DB == nil || s.Cfg == nil {
		return nil, fmt.Errorf("files: access misconfigured")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var row FileRecord
	if err := repository.FindByID(c.Request.Context(), s.DB, &row, tid, assetID); err != nil {
		return nil, err
	}
	if !IsAccessible(row.SecurityStatus) {
		return nil, gorm.ErrRecordNotFound
	}
	ttl := s.ttlForKind(kind)
	exp := time.Now().UTC().Add(ttl)
	token := signDownload(s.Cfg.JWTSecret, assetID.String(), tid, exp.Unix())
	return &SignedURL{
		AssetID:   assetID.String(),
		ExpiresAt: exp.Unix(),
		Token:     token,
		Path:      fmt.Sprintf("/api/v1/files/%s/download?expires=%d&token=%s", assetID.String(), exp.Unix(), token),
	}, nil
}

func (s *ObjectAccessService) ttlForKind(kind string) time.Duration {
	switch strings.TrimSpace(strings.ToLower(kind)) {
	case "export":
		sec := s.Cfg.Tenant.ExportDownloadURLTTL
		if sec <= 0 {
			sec = 3600
		}
		return time.Duration(sec) * time.Second
	case "sensitive":
		sec := s.Cfg.Tenant.SensitiveDownloadURLTTL
		if sec <= 0 {
			sec = 120
		}
		return time.Duration(sec) * time.Second
	default:
		sec := s.Cfg.Tenant.PrivateDownloadURLTTL
		if sec <= 0 {
			sec = 300
		}
		return time.Duration(sec) * time.Second
	}
}

func signDownload(secret, assetID string, tenantID int64, exp int64) string {
	payload := assetID + "|" + strconv.FormatInt(tenantID, 10) + "|" + strconv.FormatInt(exp, 10)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyDownloadToken validates signed download token.
func VerifyDownloadToken(secret, assetID string, tenantID int64, exp int64, token string) bool {
	if time.Now().UTC().Unix() > exp {
		return false
	}
	want := signDownload(secret, assetID, tenantID, exp)
	return hmac.Equal([]byte(want), []byte(strings.TrimSpace(token)))
}

// LoadForDownload loads asset with tenant + status checks.
func (s *ObjectAccessService) LoadForDownload(ctx context.Context, tenantID int64, assetID uuid.UUID) (*FileRecord, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("files: access misconfigured")
	}
	var row FileRecord
	if err := repository.FindByID(ctx, s.DB, &row, tenantID, assetID); err != nil {
		return nil, err
	}
	if !IsAccessible(row.SecurityStatus) {
		return nil, gorm.ErrRecordNotFound
	}
	return &row, nil
}
