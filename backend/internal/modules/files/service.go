package files

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/filescanner"
	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
	"github.com/trademind-ai/trademind/backend/internal/pkg/repository"
	"github.com/trademind-ai/trademind/backend/internal/providers/storage"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
	"golang.org/x/image/webp"
	"gorm.io/gorm"
)

// Service handles uploads and file metadata.
type Service struct {
	DB        *gorm.DB
	Redis     *rdb.Client
	Settings  *settings.Service
	MaxBytes  int64
	Metrics   *metrics.Catalog
	scanQueue scanQueue
}

// Storage is configured as a system-wide integration in the Admin settings UI.
// Tenant identity scopes metadata and object keys, not the provider credentials.
const globalStorageSettingsTenantID int64 = 0

// UploadResult is returned to the HTTP layer after a successful upload.
type UploadResult struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	ObjectKey   string `json:"objectKey"`
	URL         string `json:"url"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
}

// Upload reads multipart bytes, stores via Provider, persists metadata.
func (s *Service) Upload(c *gin.Context, originalName string, r io.Reader) (*UploadResult, error) {
	if s == nil || s.DB == nil || s.Settings == nil {
		return nil, fmt.Errorf("files: misconfigured")
	}
	reqCtx := c.Request.Context()
	max := s.MaxBytes
	if max <= 0 {
		max = 10 << 20
	}
	limited := io.LimitReader(r, max+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("files: read upload: %w", err)
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("file exceeds maximum size (%d bytes)", max)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("empty file")
	}

	ct := http.DetectContentType(data)
	ext, ok := extForContentType(ct)
	if !ok {
		ext2, ok2 := extFromOriginalName(originalName)
		if !ok2 {
			return nil, fmt.Errorf("only jpg, jpeg, png, webp, gif images are allowed")
		}
		if ct != "application/octet-stream" {
			return nil, fmt.Errorf("file content is not a recognized image")
		}
		ext = ext2
		ct = mimeTypeForExt(ext)
	}

	if _, _, decErr := image.DecodeConfig(bytes.NewReader(data)); decErr != nil {
		if _, werr := webp.DecodeConfig(bytes.NewReader(data)); werr != nil {
			return nil, fmt.Errorf("file is not a decodable image")
		}
	}
	if strings.Contains(objKeyPath(originalName), "..") {
		return nil, fmt.Errorf("invalid filename")
	}

	var adminID *uuid.UUID
	if idStr, ok := c.Get(ctxkey.AdminID); ok {
		if sub, ok := idStr.(string); ok {
			if u, err := uuid.Parse(sub); err == nil {
				adminID = &u
			}
		}
	}

	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}

	plain, err := s.Settings.PlainByGroup(reqCtx, globalStorageSettingsTenantID, "storage")
	if err != nil {
		return nil, err
	}
	prov, kind, err := storage.NewFromPlain(plain)
	if err != nil {
		return nil, err
	}
	if err := requirePrivateQuarantineStorage(kind, plain); err != nil {
		return nil, err
	}

	day := time.Now().UTC().Format("2006/01/02")
	objKey := fmt.Sprintf("quarantine/t%d/%s/%s%s", tid, day, uuid.NewString(), ext)
	if strings.Contains(objKey, "..") {
		return nil, fmt.Errorf("invalid object key")
	}

	if err := prov.Put(reqCtx, objKey, bytes.NewReader(data), int64(len(data)), ct); err != nil {
		return nil, err
	}

	row := &FileRecord{
		TenantID:       tid,
		OriginalName:   strings.TrimSpace(originalName),
		ObjectKey:      objKey,
		PublicURL:      "",
		ContentType:    ct,
		Size:           int64(len(data)),
		StorageKind:    kind,
		SecurityStatus: SecurityPendingScan,
		ScanStatus:     SecurityPendingScan,
		CreatedBy:      adminID,
	}
	if err := s.DB.WithContext(reqCtx).Create(row).Error; err != nil {
		_ = prov.Delete(reqCtx, objKey)
		return nil, err
	}
	if err := s.EnqueueSecurityScan(reqCtx, tid, row.ID); err != nil {
		dbErr := s.DB.WithContext(reqCtx).Delete(&FileRecord{}, "id = ?", row.ID).Error
		objectErr := prov.Delete(reqCtx, objKey)
		return nil, fmt.Errorf("files: enqueue security scan: %w", errors.Join(err, dbErr, objectErr))
	}

	return &UploadResult{
		ID:          row.ID.String(),
		Filename:    row.OriginalName,
		ObjectKey:   row.ObjectKey,
		URL:         "",
		ContentType: row.ContentType,
		Size:        row.Size,
	}, nil
}

// The current storage Provider abstraction has one namespace per provider. A
// public object-store/CDN namespace cannot safely hold pending objects: callers
// could construct public_base/quarantine/<key> outside the /static gate.
// Local storage is safe only when callers use the same-origin /static route,
// where StaticHandler authorizes every read by status. A direct local/CDN
// public_base would expose the returned quarantine object key.
// Remote quarantine requires a separate private provider/bucket adapter.
func requirePrivateQuarantineStorage(kind string, plain map[string]string) error {
	if !strings.EqualFold(strings.TrimSpace(kind), "local") {
		return fmt.Errorf("files: remote public storage cannot be used for quarantine without a private storage provider")
	}
	publicBase := strings.TrimRight(strings.TrimSpace(plain["public_base"]), "/")
	if publicBase == "" || publicBase == "/static" {
		return nil
	}
	return fmt.Errorf("files: local quarantine requires public_base=/static so reads pass through the security-status gate")
}

// SaveProcessedOpts writes arbitrary bytes via the configured Storage Provider (e.g. AI pipeline output).
type SaveProcessedOpts struct {
	TenantID     int64
	SourceFileID uuid.UUID
	OriginalName string
	ObjectKey    string
	Data         []byte
	ContentType  string
	CreatedBy    *uuid.UUID
}

// SaveProcessed stores bytes under ObjectKey and persists a files row (same path as multipart Upload).
func (s *Service) SaveProcessed(ctx context.Context, opts SaveProcessedOpts) (*FileRecord, error) {
	return s.saveProcessed(ctx, opts)
}

// SaveUntrustedProcessed persists internally-produced or remotely-fetched bytes
// without asserting a clean source file. It is deliberately not exported by an
// HTTP handler; bytes are synchronously scanned before publication.
func (s *Service) SaveUntrustedProcessed(ctx context.Context, opts SaveProcessedOpts) (*FileRecord, error) {
	return s.saveProcessed(ctx, opts)
}

func (s *Service) saveProcessed(ctx context.Context, opts SaveProcessedOpts) (*FileRecord, error) {
	if s == nil || s.DB == nil || s.Settings == nil {
		return nil, fmt.Errorf("files: misconfigured")
	}
	if opts.TenantID <= 0 {
		return nil, fmt.Errorf("files: tenant is required")
	}
	if opts.SourceFileID != uuid.Nil {
		var source FileRecord
		if err := s.DB.WithContext(ctx).Where("id = ? AND tenant_id = ? AND security_status = ?", opts.SourceFileID, opts.TenantID, SecurityClean).First(&source).Error; err != nil {
			return nil, fmt.Errorf("files: clean source file not found: %w", err)
		}
	}
	max := s.MaxBytes
	if max <= 0 {
		max = 10 << 20
	}
	data := opts.Data
	if int64(len(data)) > max {
		return nil, fmt.Errorf("file exceeds maximum size (%d bytes)", max)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("empty payload")
	}
	objKey := strings.TrimSpace(opts.ObjectKey)
	if objKey == "" {
		return nil, fmt.Errorf("objectKey required")
	}
	if strings.HasPrefix(objKey, "quarantine/") || strings.Contains(objKey, "..") {
		return nil, fmt.Errorf("invalid processed object key")
	}
	ct := strings.TrimSpace(opts.ContentType)
	if ct == "" {
		ct = http.DetectContentType(data)
	}
	if err := scanProcessedBytes(ctx, opts.TenantID, objKey, ct, data, max); err != nil {
		return nil, err
	}

	plain, err := s.Settings.PlainByGroup(ctx, globalStorageSettingsTenantID, "storage")
	if err != nil {
		return nil, err
	}
	prov, kind, err := storage.NewFromPlain(plain)
	if err != nil {
		return nil, err
	}

	if err := prov.Put(ctx, objKey, bytes.NewReader(data), int64(len(data)), ct); err != nil {
		return nil, err
	}
	pubURL, err := prov.GetURL(ctx, objKey)
	if err != nil {
		_ = prov.Delete(ctx, objKey)
		return nil, err
	}
	row := &FileRecord{
		TenantID:       opts.TenantID,
		OriginalName:   strings.TrimSpace(opts.OriginalName),
		ObjectKey:      objKey,
		PublicURL:      pubURL,
		ContentType:    ct,
		Size:           int64(len(data)),
		StorageKind:    kind,
		SecurityStatus: SecurityClean,
		ScanStatus:     SecurityClean,
		CreatedBy:      opts.CreatedBy,
	}
	if err := s.DB.WithContext(ctx).Create(row).Error; err != nil {
		_ = prov.Delete(ctx, objKey)
		return nil, err
	}
	return row, nil
}

func scanProcessedBytes(ctx context.Context, tenantID int64, key, contentType string, data []byte, max int64) error {
	f, err := os.CreateTemp("", "tm-processed-scan-*")
	if err != nil {
		return err
	}
	path := f.Name()
	defer os.Remove(path)
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	sc := filescanner.NewImageDecodeScanner(max, 50_000_000, 8192, 8192, 300)
	res, err := (&filescanner.CompositeFileScanner{Scanners: []filescanner.FileScanner{&filescanner.BasicFilePolicyScanner{}, sc}}).Scan(ctx, filescanner.ScanInput{TenantID: tenantID, ObjectKey: key, MimeType: contentType, Size: int64(len(data)), LocalTempPath: path})
	if err != nil {
		return fmt.Errorf("files: processed scan: %w", err)
	}
	if res.Status != filescanner.ResultClean {
		return fmt.Errorf("files: processed scan rejected: %s", res.ReasonCode)
	}
	return nil
}

func extForContentType(ct string) (string, bool) {
	base := strings.ToLower(strings.TrimSpace(strings.Split(ct, ";")[0]))
	switch base {
	case "image/jpeg":
		return ".jpg", true
	case "image/png":
		return ".png", true
	case "image/webp":
		return ".webp", true
	case "image/gif":
		return ".gif", true
	default:
		return "", false
	}
}

func extFromOriginalName(name string) (string, bool) {
	e := strings.ToLower(filepath.Ext(name))
	switch e {
	case ".jpg", ".jpeg":
		return ".jpg", true
	case ".png":
		return ".png", true
	case ".webp":
		return ".webp", true
	case ".gif":
		return ".gif", true
	default:
		return "", false
	}
}

func mimeTypeForExt(ext string) string {
	switch ext {
	case ".jpg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return mime.TypeByExtension(ext)
	}
}

func objKeyPath(name string) string {
	return strings.ReplaceAll(filepath.Clean("/"+name), "\\", "/")
}

// ListQuery binds list filters.
type ListQuery struct {
	Page        int
	PageSize    int
	ContentType string
}

// ListResult is paginated file rows.
type ListResult struct {
	Items      []FileRecord
	Total      int64
	Page       int
	PageSize   int
	TotalPages int
}

// List returns paginated file metadata.
func (s *Service) List(c *gin.Context, q ListQuery) (*ListResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("files: no db")
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	ps := q.PageSize
	if ps < 1 {
		ps = 20
	}
	if ps > 100 {
		ps = 100
	}
	tx := s.DB.WithContext(c.Request.Context()).Model(&FileRecord{})
	if scoped, _, err := adminperm.ApplyTenantScope(c, tx); err != nil {
		return nil, err
	} else {
		tx = scoped
	}
	if v := strings.TrimSpace(q.ContentType); v != "" {
		tx = tx.Where("content_type = ?", v)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	offset := (page - 1) * ps
	var items []FileRecord
	if err := tx.Order("created_at DESC").Offset(offset).Limit(ps).Find(&items).Error; err != nil {
		return nil, err
	}
	pages := int(total) / ps
	if int(total)%ps != 0 {
		pages++
	}
	if pages == 0 && total > 0 {
		pages = 1
	}
	return &ListResult{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   ps,
		TotalPages: pages,
	}, nil
}

// Delete removes DB metadata and the stored object when using a supported provider.
func (s *Service) Delete(c *gin.Context, id uuid.UUID) error {
	return s.DeleteRecordByTenant(c, id)
}

// DeleteRecordByID removes a file row and its storage object.
func (s *Service) DeleteRecordByID(ctx context.Context, id uuid.UUID) error {
	if s == nil || s.DB == nil || s.Settings == nil {
		return fmt.Errorf("files: misconfigured")
	}
	var row FileRecord
	if err := s.DB.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return err
	}
	return s.deleteRecord(ctx, &row)
}

// DeleteRecordByTenant removes a tenant-scoped file row.
func (s *Service) DeleteRecordByTenant(c *gin.Context, id uuid.UUID) error {
	if s == nil || s.DB == nil || s.Settings == nil {
		return fmt.Errorf("files: misconfigured")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return err
	}
	var row FileRecord
	if err := repository.FindByID(c.Request.Context(), s.DB, &row, tid, id); err != nil {
		return err
	}
	return s.deleteRecord(c.Request.Context(), &row)
}

func (s *Service) deleteRecord(ctx context.Context, row *FileRecord) error {
	plain, err := s.Settings.PlainByGroup(ctx, globalStorageSettingsTenantID, "storage")
	if err != nil {
		return err
	}
	prov, _, err := storage.NewFromPlainForStoredKind(plain, strings.TrimSpace(row.StorageKind))
	if err != nil {
		return err
	}
	if err := prov.Delete(ctx, row.ObjectKey); err != nil {
		return err
	}
	return s.DB.WithContext(ctx).Delete(&FileRecord{}, "id = ?", row.ID).Error
}

// DeleteStorageObject removes an object from configured storage (ignores empty keys).
func (s *Service) DeleteStorageObject(ctx context.Context, objectKey string) error {
	objectKey = strings.TrimSpace(objectKey)
	if s == nil || s.Settings == nil || objectKey == "" {
		return nil
	}
	plain, err := s.Settings.PlainByGroup(ctx, globalStorageSettingsTenantID, "storage")
	if err != nil {
		return err
	}
	prov, _, err := storage.NewFromPlain(plain)
	if err != nil {
		return err
	}
	return prov.Delete(ctx, objectKey)
}
