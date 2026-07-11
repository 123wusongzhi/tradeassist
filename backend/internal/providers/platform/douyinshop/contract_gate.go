package douyinshop

import (
	"fmt"
	"os"
	"strings"
)

// Contract capability status values (explicit — no ambiguous booleans).
const (
	ContractStatusVerified                      = "verified"
	ContractStatusFixtureVerified               = "fixture_verified"
	ContractStatusBlockedByContractVerification = "blocked_by_contract_verification"
	ContractStatusUnsupported                   = "unsupported"
	ContractStatusDisabled                      = "disabled"
)

// Well-known Douyin contract capability keys.
const (
	CapDouyinIMConversationList = "douyin_im_conversation_list"
	CapDouyinIMMessageList      = "douyin_im_message_list"
	CapDouyinIMSend             = "douyin_im_send"
	CapDouyinBrandList          = "douyin_brand_list"
	CapDouyinWebhookSignatureV1 = "douyin_webhook_signature_v1"
	CapDouyinOrderWebhookEvents = "douyin_order_webhook_events"
	CapDouyinInventoryQuery     = "douyin_inventory_query"
	CapDouyinProductDraftCreate = "douyin_product_draft_create"
)

// ContractCapabilityStatus describes one gated capability.
type ContractCapabilityStatus struct {
	Capability string `json:"capability"`
	Status     string `json:"status"`
	Message    string `json:"message,omitempty"`
	DTOVersion string `json:"dtoVersion,omitempty"`
	Scope      string `json:"scope,omitempty"`
	MethodPath string `json:"methodPath,omitempty"`
}

// ContractCapabilityGate gates Douyin capabilities by explicit contract state.
type ContractCapabilityGate interface {
	Status(capability string) ContractCapabilityStatus
	Require(capability string) error
}

// DefaultContractGate is the production default registry of Douyin contract states.
type DefaultContractGate struct {
	// AppEnv when "production" rejects fixture_verified for signature verification.
	AppEnv string
	// Overrides optional per-capability status (tests).
	Overrides map[string]string
}

// NewDefaultContractGate builds the P3.1 default gate.
func NewDefaultContractGate(appEnv string) *DefaultContractGate {
	return &DefaultContractGate{
		AppEnv:    strings.TrimSpace(appEnv),
		Overrides: map[string]string{},
	}
}

func (g *DefaultContractGate) statusOf(cap string) string {
	if g != nil && g.Overrides != nil {
		if v, ok := g.Overrides[cap]; ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	switch cap {
	case CapDouyinIMConversationList, CapDouyinIMMessageList, CapDouyinIMSend:
		return ContractStatusBlockedByContractVerification
	case CapDouyinBrandList:
		return ContractStatusBlockedByContractVerification
	case CapDouyinWebhookSignatureV1:
		return ContractStatusFixtureVerified
	case CapDouyinOrderWebhookEvents:
		return ContractStatusFixtureVerified
	case CapDouyinInventoryQuery, CapDouyinProductDraftCreate:
		return ContractStatusFixtureVerified
	default:
		return ContractStatusUnsupported
	}
}

func (g *DefaultContractGate) meta(cap string) ContractCapabilityStatus {
	st := ContractCapabilityStatus{
		Capability: cap,
		Status:     g.statusOf(cap),
	}
	switch cap {
	case CapDouyinIMConversationList:
		st.MethodPath = "blocked — IM conversation list path pending contract"
		st.Scope = "im.conversation.read (pending)"
		st.DTOVersion = "v1_pending"
		st.Message = "适配代码已完成，等待平台契约确认"
	case CapDouyinIMMessageList:
		st.MethodPath = "blocked — IM message list path pending contract"
		st.Scope = "im.message.read (pending)"
		st.DTOVersion = "v1_pending"
		st.Message = "适配代码已完成，等待平台契约确认"
	case CapDouyinIMSend:
		st.MethodPath = "blocked — IM send path pending contract"
		st.Scope = "im.message.send (pending)"
		st.DTOVersion = "v1_pending"
		st.Message = "真实发送继续要求人工确认"
	case CapDouyinBrandList:
		st.MethodPath = "blocked — brand.list path pending contract"
		st.Scope = "product.brand.read (pending)"
		st.DTOVersion = "v1_pending"
		st.Message = "品牌接口仍等待平台契约确认"
	case CapDouyinWebhookSignatureV1:
		st.MethodPath = "SHA1(appSecret+rawBody)"
		st.DTOVersion = "v1"
		if st.Status == ContractStatusFixtureVerified {
			st.Message = "签名实现已完成，真实契约验证待完成"
		}
	case CapDouyinOrderWebhookEvents:
		st.DTOVersion = "jinritemai_tag_v1"
		st.Message = "订单 Webhook 事件映射基于契约 Fixture"
	case CapDouyinInventoryQuery:
		st.MethodPath = MethodProductDetail
		st.DTOVersion = "v1_fixture"
	case CapDouyinProductDraftCreate:
		st.MethodPath = MethodProductAddV2
		st.DTOVersion = "v1_fixture"
	default:
		st.Message = "unsupported capability"
	}
	return st
}

// Status returns the contract status for a capability.
func (g *DefaultContractGate) Status(capability string) ContractCapabilityStatus {
	return g.meta(strings.TrimSpace(capability))
}

// Require blocks when the capability is not allowed in the current environment.
func (g *DefaultContractGate) Require(capability string) error {
	capability = strings.TrimSpace(capability)
	st := g.statusOf(capability)
	switch st {
	case ContractStatusVerified, ContractStatusFixtureVerified:
		if st == ContractStatusFixtureVerified && g != nil && isProductionEnv(g.AppEnv) {
			if capability == CapDouyinWebhookSignatureV1 {
				return contractRequiredError(capability, "production requires verified webhook signature contract")
			}
		}
		return nil
	case ContractStatusDisabled:
		return contractRequiredError(capability, "capability disabled by configuration")
	case ContractStatusBlockedByContractVerification:
		return contractRequiredError(capability, "blocked_by_contract_verification")
	default:
		return contractRequiredError(capability, "unsupported capability")
	}
}

func isProductionEnv(appEnv string) bool {
	e := strings.ToLower(strings.TrimSpace(appEnv))
	if e == "" {
		e = strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	}
	return e == "production" || e == "prod"
}

func contractRequiredError(cap, msg string) error {
	return NewError(CodeDouyinContractVerificationRequired,
		fmt.Sprintf("%s: %s", cap, msg), "", "contract_verification_required", "")
}

// AllContractCapabilities returns the P3.1 capability catalog for config status.
func AllContractCapabilities() []string {
	return []string{
		CapDouyinIMConversationList,
		CapDouyinIMMessageList,
		CapDouyinIMSend,
		CapDouyinBrandList,
		CapDouyinWebhookSignatureV1,
		CapDouyinOrderWebhookEvents,
		CapDouyinInventoryQuery,
		CapDouyinProductDraftCreate,
	}
}
