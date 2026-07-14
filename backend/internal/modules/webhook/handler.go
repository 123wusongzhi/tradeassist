package webhook

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/pagination"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Handler exposes the public webhook HTTP receiver.
type Handler struct {
	Svc *Service
}

func atoiQ(c *gin.Context, key string, def int) int {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return def
	}
	return n
}

// ListEvents GET /api/v1/webhook-events
func (h *Handler) ListEvents(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "webhook unavailable")
		return
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "tenant context required")
		return
	}
	q := EventListQuery{
		TenantID:  tid,
		Platform:  strings.TrimSpace(c.Query("platform")),
		Status:    strings.TrimSpace(c.Query("status")),
		EventType: strings.TrimSpace(c.Query("eventType")),
		Page:      atoiQ(c, "page", 1),
		PageSize:  atoiQ(c, "pageSize", 20),
		Cursor:    strings.TrimSpace(c.Query("cursor")),
		Limit:     atoiQ(c, "limit", 0),
	}
	q.UseCursor = q.Cursor != "" || q.Limit > 0
	if raw := strings.TrimSpace(c.Query("shopId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.InternalShopID = &u
		}
	}
	if raw := strings.TrimSpace(c.Query("start")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			q.Start = &t
		}
	}
	if raw := strings.TrimSpace(c.Query("end")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			q.End = &t
		}
	}
	res, err := h.Svc.ListEvents(c.Request.Context(), q)
	if err != nil {
		if code := pagination.ErrorCode(err); code != "" {
			response.JSON(c, http.StatusBadRequest, response.CodeBadRequest, code, gin.H{"errorCode": code})
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{
		"items":      res.Items,
		"nextCursor": res.NextCursor,
		"hasMore":    res.HasMore,
		"limit":      res.Limit,
		"list":       res.Items,
		"pagination": gin.H{
			"page":       res.Page,
			"pageSize":   res.PageSize,
			"total":      res.Total,
			"totalPages": res.TotalPages,
		},
	})
}

// Receive POST /api/v1/webhooks/:platform/:eventType
func (h *Handler) Receive(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "webhook unavailable")
		return
	}
	platform := strings.TrimSpace(c.Param("platform"))
	eventType := strings.TrimSpace(c.Param("eventType"))
	if platform == "" || eventType == "" {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "platform and eventType are required")
		return
	}

	maxBytes := h.Svc.maxPayload()
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)

	ct := strings.ToLower(strings.TrimSpace(c.GetHeader("Content-Type")))
	if !strings.HasPrefix(ct, "application/json") {
		failWebhook(c, newCodeError(CodeInvalidContentType, http.StatusUnsupportedMediaType, CodeInvalidContentType))
		return
	}

	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) || isBodyTooLarge(err) {
			failWebhook(c, newCodeError(CodePayloadTooLarge, http.StatusRequestEntityTooLarge, CodePayloadTooLarge))
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "failed to read body")
		return
	}

	sig := extractSignatureHeader(c.Request.Header)
	nonce := extractNonceHeader(c.Request.Header)
	ts, tsErr := parseWebhookTimestamp(extractTimestampHeader(c.Request.Header), raw)
	if tsErr != nil && extractTimestampHeader(c.Request.Header) != "" {
		failWebhook(c, newCodeError(CodeTimestampExpired, http.StatusUnauthorized, CodeTimestampExpired))
		return
	}

	if h.Svc.Verifiers != nil {
		if err := h.Svc.Verifiers.Verify(c.Request.Context(), VerifyInput{
			Platform:  platform,
			Headers:   c.Request.Header,
			RawBody:   raw,
			Timestamp: ts,
			Nonce:     nonce,
			Signature: sig,
		}); err != nil {
			failWebhook(c, err)
			return
		}
	} else {
		failWebhook(c, newCodeError(CodeVerifierNotConfigured, http.StatusUnauthorized, CodeVerifierNotConfigured))
		return
	}

	if err := h.Svc.ValidateTimestamp(ts, !ts.IsZero()); err != nil {
		failWebhook(c, err)
		return
	}

	if !json.Valid(raw) {
		failWebhook(c, newCodeError(CodeInvalidJSON, http.StatusBadRequest, CodeInvalidJSON))
		return
	}

	var resolved *ResolvedWebhookShop
	if isDouyinWebhookPlatform(platform) {
		resolveInput, err := ExtractResolveWebhookShopInput(platform, eventType, c.Request.Header, raw)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid webhook payload")
			return
		}
		resolveInput.AppEnv = h.Svc.AppEnv
		resolveInput.RequestID = c.GetString("requestId")
		if h.Svc.ShopResolver == nil {
			failWebhook(c, newCodeError(CodeDouyinWebhookShopNotResolved, http.StatusForbidden, CodeDouyinWebhookShopNotResolved))
			return
		}
		resolved, err = h.Svc.ShopResolver.Resolve(c.Request.Context(), resolveInput)
		if err != nil {
			if ce, ok := AsCodeError(err); ok {
				failWebhook(c, ce)
				return
			}
			response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "webhook shop resolve failed")
			return
		}
	}

	eventID := extractEventID(raw)
	result, err := h.Svc.Ingest(c.Request.Context(), IngestRequest{
		Platform:     platform,
		EventType:    eventType,
		EventID:      eventID,
		Payload:      json.RawMessage(raw),
		Timestamp:    ts,
		ResolvedShop: resolved,
	})
	if err != nil {
		if ce, ok := AsCodeError(err); ok {
			failWebhook(c, ce)
			return
		}
		// Persist / internal failure: do not leak details; do not ACK success.
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "webhook persist failed")
		return
	}

	response.JSON(c, http.StatusOK, response.CodeOK, "accepted", gin.H{
		"eventId":   result.EventID,
		"status":    result.Status,
		"duplicate": result.Duplicate,
	})
}

func failWebhook(c *gin.Context, err error) {
	ce, ok := AsCodeError(err)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, err.Error())
		return
	}
	status := ce.HTTPStatus
	if status == 0 {
		status = http.StatusBadRequest
	}
	biz := response.CodeBadRequest
	switch {
	case status == http.StatusUnauthorized:
		biz = response.CodeUnauthorized
	case status == http.StatusForbidden:
		biz = response.CodeForbidden
	case status == http.StatusConflict:
		biz = response.CodeBadRequest
	case status >= 500:
		biz = response.CodeInternalError
	}
	response.Fail(c, status, biz, ce.Code)
}

func isBodyTooLarge(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "http: request body too large") || strings.Contains(msg, "body size exceeds")
}

func parseWebhookTimestamp(headerVal string, body []byte) (time.Time, error) {
	headerVal = strings.TrimSpace(headerVal)
	if headerVal != "" {
		return parseTimestampValue(headerVal)
	}
	// Optional body fields: timestamp / eventTime / createdAt (unix or RFC3339).
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return time.Time{}, nil
	}
	for _, key := range []string{"timestamp", "eventTime", "event_time", "createdAt", "created_at"} {
		raw, ok := m[key]
		if !ok {
			continue
		}
		var s string
		if err := json.Unmarshal(raw, &s); err == nil && strings.TrimSpace(s) != "" {
			return parseTimestampValue(s)
		}
		var n json.Number
		if err := json.Unmarshal(raw, &n); err == nil {
			return parseTimestampValue(n.String())
		}
		var f float64
		if err := json.Unmarshal(raw, &f); err == nil {
			return parseTimestampValue(strconv.FormatInt(int64(f), 10))
		}
	}
	return time.Time{}, nil
}

func parseTimestampValue(v string) (time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}, errors.New("empty timestamp")
	}
	if unix, err := strconv.ParseInt(v, 10, 64); err == nil {
		// Support seconds or milliseconds.
		if unix > 1e12 {
			return time.UnixMilli(unix).UTC(), nil
		}
		return time.Unix(unix, 0).UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, errors.New("invalid timestamp")
}

func extractEventID(raw []byte) string {
	if id := extractEventIDFromObject(raw); id != "" {
		return id
	}
	var arr []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		for _, item := range arr {
			if id := extractEventIDFromMap(item); id != "" {
				return id
			}
		}
	}
	return ""
}

func extractEventIDFromObject(raw []byte) string {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	return extractEventIDFromMap(m)
}

func extractEventIDFromMap(m map[string]json.RawMessage) string {
	for _, key := range []string{"eventId", "event_id", "msg_id", "msgId", "id"} {
		rawVal, ok := m[key]
		if !ok {
			continue
		}
		var s string
		if err := json.Unmarshal(rawVal, &s); err == nil {
			s = strings.TrimSpace(s)
			if s != "" {
				return s
			}
		}
		// numeric id
		var n json.Number
		dec := json.NewDecoder(bytes.NewReader(rawVal))
		dec.UseNumber()
		if err := dec.Decode(&n); err == nil {
			s := strings.TrimSpace(n.String())
			if s != "" {
				return s
			}
		}
	}
	return ""
}
