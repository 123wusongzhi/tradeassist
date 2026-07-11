package douyinshop

import (
	"encoding/json"
	"fmt"
	"strings"
)

// DouyinWebhookEnvelope is the standard Douyin Open Platform push envelope.
// Source: Douyin Open Platform webhook documentation (event push spec).
type DouyinWebhookEnvelope struct {
	Event     string         `json:"event"`
	ClientKey string         `json:"client_key,omitempty"`
	Content   map[string]any `json:"content,omitempty"`
}

// JinriteimaiPushItem is one item in the jinritemai array-style push.
// Source: jinritemai open platform push spec.
type JinriteimaiPushItem struct {
	Tag   string         `json:"tag"`
	MsgID string         `json:"msg_id,omitempty"`
	Data  map[string]any `json:"data,omitempty"`
}

// JinriteimaiPushEnvelope is the top-level jinritemai push array.
type JinriteimaiPushEnvelope []JinriteimaiPushItem

// EventType mapped from known tag values.
// Tag "0" is the health-check ping.
// Other tags are contract-mapped from Douyin/jinritemai documentation.
// Unknown tags MUST be ACK'd safely without processing.
var jinriteimaiTagToEventType = map[string]string{
	"0":   "health_check",
	"100": "order_created",
	"101": "order_paid",
	"102": "order_shipped",
	"103": "order_completed",
	"104": "order_cancelled",
	"200": "inventory_alert",
	"300": "product_status_changed",
	// Tags not in this map → "unknown:{tag}" — safe ACK, no processing
}

// EventTypeFromJinriteimaiTag returns the normalized event type for a jinritemai push tag.
// For unknown tags, returns "unknown:{tag}" so callers can safely ACK and log.
func EventTypeFromJinriteimaiTag(tag string) string {
	tag = strings.TrimSpace(tag)
	if et, ok := jinriteimaiTagToEventType[tag]; ok {
		return et
	}
	return "unknown:" + tag
}

// IsHealthCheck returns true if the push item is a health-check ping.
func IsHealthCheck(tag string) bool {
	return strings.TrimSpace(tag) == "0"
}

// ParseDouyinWebhookEnvelope parses the standard Douyin webhook push body.
func ParseDouyinWebhookEnvelope(raw []byte) (*DouyinWebhookEnvelope, error) {
	var env DouyinWebhookEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("parse douyin webhook envelope: %w", err)
	}
	return &env, nil
}

// ParseJinriteimaiPushEnvelope parses the jinritemai array-style push body.
func ParseJinriteimaiPushEnvelope(raw []byte) (JinriteimaiPushEnvelope, error) {
	var items JinriteimaiPushEnvelope
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("parse jinritemai push envelope: %w", err)
	}
	return items, nil
}

// NormalizedWebhookEvent is the unified event after parsing either envelope format.
type NormalizedWebhookEvent struct {
	EventType    string
	MsgID        string
	Tag          string
	ClientKey    string
	IsHealthPing bool
	Data         map[string]any
	Raw          []byte
}

// NormalizeDouyinEnvelope converts a parsed DouyinWebhookEnvelope to NormalizedWebhookEvent.
func NormalizeDouyinEnvelope(env *DouyinWebhookEnvelope, raw []byte) *NormalizedWebhookEvent {
	if env == nil {
		return nil
	}
	return &NormalizedWebhookEvent{
		EventType: strings.TrimSpace(env.Event),
		ClientKey: strings.TrimSpace(env.ClientKey),
		Data:      env.Content,
		Raw:       raw,
	}
}

// NormalizeJinriteimaiItem converts one push item to NormalizedWebhookEvent.
func NormalizeJinriteimaiItem(item JinriteimaiPushItem, raw []byte) *NormalizedWebhookEvent {
	et := EventTypeFromJinriteimaiTag(item.Tag)
	return &NormalizedWebhookEvent{
		EventType:    et,
		Tag:          strings.TrimSpace(item.Tag),
		MsgID:        strings.TrimSpace(item.MsgID),
		IsHealthPing: IsHealthCheck(item.Tag),
		Data:         item.Data,
		Raw:          raw,
	}
}
