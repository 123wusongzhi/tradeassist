package pagination

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const (
	DefaultLimit = 50
	MaxLimit     = 200
	MaxOffset    = 10000
	MaxCursorLen = 512
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
		return Page{}, fmt.Errorf("pagination offset %d exceeds max %d; use cursor pagination", offset, MaxOffset)
	}
	return Page{Page: page, Limit: limit, Offset: offset, Truncated: truncated}, nil
}

func EncodeCursor(p CursorPayload) (string, error) {
	p.Version = 1
	if p.SortField == "" || p.SortValue == "" || p.TieID == "" {
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

func DecodeCursor(raw string, tenantID int64, shopID string) (CursorPayload, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return CursorPayload{}, nil
	}
	if len(raw) > MaxCursorLen {
		return CursorPayload{}, fmt.Errorf("cursor exceeds max length")
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return CursorPayload{}, fmt.Errorf("invalid cursor encoding")
	}
	var envelope signedCursor
	if err := json.Unmarshal(b, &envelope); err != nil {
		return CursorPayload{}, fmt.Errorf("invalid cursor payload")
	}
	payload, err := json.Marshal(envelope.Payload)
	if err != nil {
		return CursorPayload{}, fmt.Errorf("invalid cursor payload")
	}
	if envelope.Sig == "" || !hmac.Equal([]byte(envelope.Sig), []byte(signCursor(payload))) {
		return CursorPayload{}, fmt.Errorf("invalid cursor signature")
	}
	p := envelope.Payload
	if p.Version != 1 {
		return CursorPayload{}, fmt.Errorf("unsupported cursor version")
	}
	if p.TenantID != tenantID {
		return CursorPayload{}, fmt.Errorf("cursor tenant scope mismatch")
	}
	if strings.TrimSpace(p.ShopID) != strings.TrimSpace(shopID) {
		return CursorPayload{}, fmt.Errorf("cursor shop scope mismatch")
	}
	if p.SortField == "" || p.SortValue == "" || p.TieID == "" {
		return CursorPayload{}, fmt.Errorf("invalid cursor sort payload")
	}
	return p, nil
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
