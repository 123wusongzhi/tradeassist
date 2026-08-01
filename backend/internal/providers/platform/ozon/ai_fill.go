package ozon

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	aigate "github.com/trademind-ai/trademind/backend/internal/providers/ai"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

// chatFunc abstracts the AI gateway so tests can inject a fake without network.
type chatFunc func(ctx context.Context, req aigate.ChatRequest) (*aigate.ChatResponse, error)

var boundChat chatFunc

// BindAIGateway wires the project AI gateway for LLM auto-fill of missing
// category attributes. Call once at startup after the gateway is constructed;
// nil gateway is ignored (AI fill simply stays disabled).
func BindAIGateway(g *aigate.Gateway) {
	if g == nil {
		return
	}
	boundChat = g.Chat
}

// bindChatForTest replaces the global chat function for tests and returns a
// restore func. Production code always goes through BindAIGateway.
func bindChatForTest(fn chatFunc) func() {
	prev := boundChat
	boundChat = fn
	return func() { boundChat = prev }
}

// aiAttributeFill is the strict JSON shape requested from the LLM.
type aiAttributeFill struct {
	Attributes map[string]string `json:"attributes"`
}

// fillMissingAttributesWithAI asks the LLM for suggested values for missing
// required category attributes. It degrades to an error on any failure so the
// caller can record aiFillFailed and continue with the original payload.
func fillMissingAttributesWithAI(
	ctx context.Context,
	chat chatFunc,
	d platformp.PlatformProductDraft,
	missing []ozonAttribute,
) (map[string]string, error) {
	if chat == nil || len(missing) == 0 {
		return nil, nil
	}
	names := make([]string, 0, len(missing))
	for _, a := range missing {
		names = append(names, a.Name)
	}
	prompt := fmt.Sprintf(
		"你是跨境电商商品刊登助手。请为 Ozon 商品补全以下缺失的类目必填属性值。\n商品标题：%s\n商品描述：%s\n商品属性：%s\n缺失属性：%s\n只输出 JSON，格式 {\"attributes\":{\"<属性名>\":\"<建议值>\"}}；值用俄语或平台接受的通用写法，每个值不超过 120 字符，不要输出其他内容。",
		truncateRunes(d.Title, 500),
		truncateRunes(d.Description, 1200),
		attrsForPrompt(d.Attributes),
		strings.Join(names, "、"),
	)
	resp, err := chat(ctx, aigate.ChatRequest{
		Messages:       []aigate.Message{{Role: "user", Content: prompt}},
		Temperature:    0.3,
		MaxTokens:      1000,
		ResponseFormat: &aigate.ResponseFormat{Type: "json_object"},
	})
	if err != nil {
		return nil, err
	}
	var out aiAttributeFill
	if err := json.Unmarshal([]byte(strings.TrimSpace(resp.Content)), &out); err != nil {
		return nil, fmt.Errorf("parse ai fill response: %w", err)
	}
	clean := make(map[string]string, len(missing))
	for _, a := range missing {
		v := strings.TrimSpace(out.Attributes[a.Name])
		if v == "" {
			continue
		}
		clean[a.Name] = truncateRunes(v, 120)
	}
	return clean, nil
}

func attrsForPrompt(attrs map[string]any) string {
	if len(attrs) == 0 {
		return "（无）"
	}
	flat := localAttributeMap(platformp.PlatformProductDraft{Attributes: attrs})
	if len(flat) == 0 {
		return "（无）"
	}
	parts := make([]string, 0, len(flat))
	for k, v := range flat {
		parts = append(parts, k+":"+v)
	}
	return strings.Join(parts, "；")
}
