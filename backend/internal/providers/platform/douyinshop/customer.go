package douyinshop

import (
	"context"
	"strings"
	"time"
)

// CapCustomerMessage is the capability identifier for Douyin customer messaging.
const CapCustomerMessage = "customer_message"

// CustomerCapability is the P3 customer messaging facade.
// All live methods are gated behind contract verification.
type CustomerCapability interface {
	// IsEnabled returns true only when the platform has confirmed the IM API contract.
	IsEnabled() bool

	// PullMessages fetches recent customer messages (gated).
	PullMessages(ctx context.Context, req PullMessagesRequest) ([]CustomerMessage, error)

	// SendMessage sends a reply (gated, manual confirmation required first).
	SendMessage(ctx context.Context, req SendMessageRequest) error
}

// PullMessagesRequest is the normalized pull messages input.
type PullMessagesRequest struct {
	ConversationID string
	PageToken      string
	Limit          int
}

// SendMessageRequest is the normalized send message input.
type SendMessageRequest struct {
	ConversationID  string
	ClientMessageID string
	Content         string
	ContentType     string // text, image, etc.
}

// CustomerMessage is a normalized platform message envelope.
type CustomerMessage struct {
	MessageID      string         `json:"messageId"`
	ConversationID string         `json:"conversationId"`
	SenderType     string         `json:"senderType"` // buyer, seller
	ContentType    string         `json:"contentType"`
	Content        string         `json:"content"`
	SentAt         *time.Time     `json:"sentAt,omitempty"`
	Raw            map[string]any `json:"raw,omitempty"`
}

// customerCapabilityImpl is the live implementation, gated by config.
type customerCapabilityImpl struct {
	c       *Client
	enabled bool
}

// NewCustomerCapabilityForTest exposes the customer capability for test use.
func NewCustomerCapabilityForTest(c *Client) CustomerCapability {
	return newCustomerCapabilityFromClient(c)
}

func newCustomerCapabilityFromClient(c *Client) CustomerCapability {
	if c == nil {
		return &customerCapabilityImpl{enabled: false}
	}
	// TODO: read customer_message_api_enabled from RuntimeConfig when the field is added
	return &customerCapabilityImpl{c: c, enabled: false}
}

func (cap *customerCapabilityImpl) IsEnabled() bool {
	return cap != nil && cap.enabled
}

func (cap *customerCapabilityImpl) PullMessages(_ context.Context, _ PullMessagesRequest) ([]CustomerMessage, error) {
	return nil, contractMismatchError("PullMessages")
}

func (cap *customerCapabilityImpl) SendMessage(_ context.Context, _ SendMessageRequest) error {
	return contractMismatchError("SendMessage")
}

func contractMismatchError(method string) *Error {
	e := NewError(CodeDouyinContractMismatch,
		"抖店 IM 消息接口需通过合同核查后方可启用；当前阶段已阻断: "+strings.TrimSpace(method),
		"", "blocked_by_contract_verification", "")
	e.Retryable = false
	return e
}

// CustomerMessageEnvelope is the synthetic DTO for fixture-driven parsing tests.
// Real API shape is blocked_by_contract_verification; this is used for unit testing only.
type CustomerMessageEnvelope struct {
	Synthetic bool              `json:"synthetic"`
	Messages  []CustomerMessage `json:"messages"`
	NextToken string            `json:"nextToken,omitempty"`
}

// ParseCustomerMessageEnvelope parses a synthetic envelope for testing purposes.
// Do not call with real Douyin IM API responses — shape is unverified.
func ParseCustomerMessageEnvelope(raw map[string]any) (*CustomerMessageEnvelope, error) {
	if raw == nil {
		return &CustomerMessageEnvelope{Synthetic: true}, nil
	}
	env := &CustomerMessageEnvelope{Synthetic: true}
	if msgs, ok := raw["messages"].([]any); ok {
		for _, m := range msgs {
			if mm, ok := m.(map[string]any); ok {
				msg := CustomerMessage{
					Raw: mm,
				}
				if v, ok := mm["message_id"].(string); ok {
					msg.MessageID = strings.TrimSpace(v)
				}
				if v, ok := mm["conversation_id"].(string); ok {
					msg.ConversationID = strings.TrimSpace(v)
				}
				if v, ok := mm["content"].(string); ok {
					msg.Content = strings.TrimSpace(v)
				}
				env.Messages = append(env.Messages, msg)
			}
		}
	}
	if v, ok := raw["next_token"].(string); ok {
		env.NextToken = strings.TrimSpace(v)
	}
	return env, nil
}
