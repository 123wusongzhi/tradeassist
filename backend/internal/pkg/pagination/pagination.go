package pagination

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	DefaultLimit = 50
	MaxLimit     = 200
	MaxOffset    = 10000
	MaxCursorLen = 512
)

const (
	ErrCodeOffsetTooDeep            = "pagination_offset_too_deep"
	ErrCodeCursorScopeMismatch      = "pagination_cursor_scope_mismatch"
	ErrCodeCursorFilterMismatch     = "pagination_cursor_filter_mismatch"
	ErrCodeCursorSignatureInvalid   = "pagination_cursor_signature_invalid"
	ErrCodeCursorVersionUnsupported = "pagination_cursor_version_unsupported"
)

var (
	ErrOffsetTooDeep            = errors.New(ErrCodeOffsetTooDeep)
	ErrCursorScopeMismatch      = errors.New(ErrCodeCursorScopeMismatch)
	ErrCursorFilterMismatch     = errors.New(ErrCodeCursorFilterMismatch)
	ErrCursorSignatureInvalid   = errors.New(ErrCodeCursorSignatureInvalid)
	ErrCursorVersionUnsupported = errors.New(ErrCodeCursorVersionUnsupported)
)

// Page is the bounded offset pagination shape used while legacy list APIs are
// migrated toward signed cursor pagination.
type Page struct {
	Page      int
	Limit     int
	Offset    int
	Truncated bool
}

// CursorMeta is the public cursor response shape for keyset APIs.
type CursorMeta struct {
	Cursor     string `json:"cursor,omitempty"`
	Limit      int    `json:"limit"`
	HasMore    bool   `json:"hasMore"`
	NextCursor string `json:"nextCursor,omitempty"`
}

type CursorPayload struct {
	Version   int    `json:"v"`
	TenantID  int64  `json:"tenantId"`
	ShopID    string `json:"shopId,omitempty"`
	ScopeHash string `json:"scopeHash"`
	SortField string `json:"sortField"`
	SortValue string `json:"sortValue"`
	TieID     string `json:"tieId"`
}

type signedCursor struct {
	Payload CursorPayload `json:"p"`
	Sig     string        `json:"s"`
}

func NormalizePage(page, limit int) (Page, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = DefaultLimit
	}
	truncated := false
	if limit > MaxLimit {
		limit = MaxLimit
		truncated = true
	}
	offset := (page - 1) * limit
	if offset > MaxOffset {
		return Page{}, fmt.Errorf("%w: offset %d exceeds max %d; use cursor pagination", ErrOffsetTooDeep, offset, MaxOffset)
	}
	return Page{Page: page, Limit: limit, Offset: offset, Truncated: truncated}, nil
}

func EncodeCursor(p CursorPayload) (string, error) {
	p.Version = 1
	if p.SortField == "" || p.SortValue == "" || p.TieID == "" || p.ScopeHash == "" {
		return "", fmt.Errorf("cursor payload requires sort field, sort value and tie id")
	}
	payload, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(signedCursor{Payload: p, Sig: signCursor(payload)})
	if err != nil {
		return "", err
	}
	out := base64.RawURLEncoding.EncodeToString(b)
	if len(out) > MaxCursorLen {
		return "", fmt.Errorf("cursor exceeds max length")
	}
	return out, nil
}

func DecodeCursor(raw string, tenantID int64, shopID string, scopeHash string) (CursorPayload, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return CursorPayload{}, nil
	}
	if len(raw) > MaxCursorLen {
		return CursorPayload{}, fmt.Errorf("%w: cursor exceeds max length", ErrCursorSignatureInvalid)
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return CursorPayload{}, fmt.Errorf("%w: invalid cursor encoding", ErrCursorSignatureInvalid)
	}
	var envelope signedCursor
	if err := json.Unmarshal(b, &envelope); err != nil {
		return CursorPayload{}, fmt.Errorf("%w: invalid cursor payload", ErrCursorSignatureInvalid)
	}
	payload, err := json.Marshal(envelope.Payload)
	if err != nil {
		return CursorPayload{}, fmt.Errorf("%w: invalid cursor payload", ErrCursorSignatureInvalid)
	}
	if envelope.Sig == "" || !hmac.Equal([]byte(envelope.Sig), []byte(signCursor(payload))) {
		return CursorPayload{}, fmt.Errorf("%w: invalid cursor signature", ErrCursorSignatureInvalid)
	}
	p := envelope.Payload
	if p.Version != 1 {
		return CursorPayload{}, ErrCursorVersionUnsupported
	}
	if p.TenantID != tenantID {
		return CursorPayload{}, ErrCursorScopeMismatch
	}
	if strings.TrimSpace(p.ShopID) != strings.TrimSpace(shopID) {
		return CursorPayload{}, ErrCursorScopeMismatch
	}
	if strings.TrimSpace(p.ScopeHash) != strings.TrimSpace(scopeHash) {
		return CursorPayload{}, ErrCursorFilterMismatch
	}
	if p.SortField == "" || p.SortValue == "" || p.TieID == "" {
		return CursorPayload{}, fmt.Errorf("%w: invalid cursor sort payload", ErrCursorSignatureInvalid)
	}
	return p, nil
}

func Fingerprint(parts map[string]any) string {
	if len(parts) == 0 {
		parts = map[string]any{"default": ""}
	}
	keys := make([]string, 0, len(parts))
	for k := range parts {
		keys = append(keys, strings.TrimSpace(strings.ToLower(k)))
	}
	sort.Strings(keys)
	canon := make(map[string]any, len(keys))
	for _, k := range keys {
		canon[k] = normalizeFingerprintValue(parts[k])
	}
	raw, _ := json.Marshal(canon)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:16])
}

func normalizeFingerprintValue(v any) any {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(strings.ToLower(t))
	case *string:
		if t == nil {
			return ""
		}
		return strings.TrimSpace(strings.ToLower(*t))
	case time.Time:
		return t.UTC().Format(time.RFC3339Nano)
	case *time.Time:
		if t == nil {
			return ""
		}
		return t.UTC().Format(time.RFC3339Nano)
	case []string:
		cp := make([]string, 0, len(t))
		for _, item := range t {
			cp = append(cp, strings.TrimSpace(strings.ToLower(item)))
		}
		sort.Strings(cp)
		return cp
	default:
		return t
	}
}

func ApplyDescKeyset(tx *gorm.DB, sortColumn string, idColumn string, cursor CursorPayload) (*gorm.DB, error) {
	if tx == nil || strings.TrimSpace(cursor.SortValue) == "" {
		return tx, nil
	}
	if strings.TrimSpace(sortColumn) == "" || strings.TrimSpace(idColumn) == "" {
		return nil, fmt.Errorf("pagination: keyset columns required")
	}
	tie := strings.TrimSpace(cursor.TieID)
	if tie == "" {
		return nil, ErrCursorSignatureInvalid
	}
	if tm, err := time.Parse(time.RFC3339Nano, cursor.SortValue); err == nil {
		return tx.Where(fmt.Sprintf("(%s < ? OR (%s = ? AND %s < ?))", sortColumn, sortColumn, idColumn), tm, tm, tie), nil
	}
	return tx.Where(fmt.Sprintf("(%s < ? OR (%s = ? AND %s < ?))", sortColumn, sortColumn, idColumn), cursor.SortValue, cursor.SortValue, tie), nil
}

func BuildNextCursor(hasMore bool, tenantID int64, shopID string, scopeHash string, sortField string, sortValue time.Time, tieID string) (string, error) {
	if !hasMore {
		return "", nil
	}
	return EncodeCursor(CursorPayload{
		TenantID:  tenantID,
		ShopID:    shopID,
		ScopeHash: scopeHash,
		SortField: sortField,
		SortValue: sortValue.UTC().Format(time.RFC3339Nano),
		TieID:     tieID,
	})
}

func ErrorCode(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrOffsetTooDeep):
		return ErrCodeOffsetTooDeep
	case errors.Is(err, ErrCursorScopeMismatch):
		return ErrCodeCursorScopeMismatch
	case errors.Is(err, ErrCursorFilterMismatch):
		return ErrCodeCursorFilterMismatch
	case errors.Is(err, ErrCursorVersionUnsupported):
		return ErrCodeCursorVersionUnsupported
	case errors.Is(err, ErrCursorSignatureInvalid):
		return ErrCodeCursorSignatureInvalid
	default:
		return ""
	}
}

func signCursor(payload []byte) string {
	mac := hmac.New(sha256.New, []byte(cursorSigningKey()))
	_, _ = mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func cursorSigningKey() string {
	key := strings.TrimSpace(os.Getenv("PAGINATION_CURSOR_SIGNING_KEY"))
	if key != "" {
		return key
	}
	return "trademind-p7-development-cursor-signing-key"
}
