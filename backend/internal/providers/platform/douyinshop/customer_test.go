package douyinshop_test

import (
	"context"
	"testing"

	douyinshop "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
)

func TestCustomerCapability_BlockedByContractVerification(t *testing.T) {
	// Customer messaging is explicitly blocked in P3
	cap := douyinshop.NewFacade(nil)
	if cap != nil {
		t.Fatal("expected nil facade for nil client")
	}

	// Direct capability check using nil client
	impl := douyinshop.NewCustomerCapabilityForTest(nil)
	if impl.IsEnabled() {
		t.Error("customer capability should be disabled when client is nil")
	}

	_, err := impl.PullMessages(context.Background(), douyinshop.PullMessagesRequest{})
	if err == nil {
		t.Error("expected error from PullMessages when blocked by contract")
	}
	var de *douyinshop.Error
	if douyinshop.AsError(err, &de) {
		if de.Code != douyinshop.CodeDouyinContractMismatch {
			t.Errorf("expected CodeDouyinContractMismatch, got %s", de.Code)
		}
	}

	sendErr := impl.SendMessage(context.Background(), douyinshop.SendMessageRequest{})
	if sendErr == nil {
		t.Error("expected error from SendMessage when blocked by contract")
	}
}

func TestParseCustomerMessageEnvelope_Synthetic(t *testing.T) {
	raw := map[string]any{
		"messages": []any{
			map[string]any{
				"message_id":      "msg-001",
				"conversation_id": "conv-001",
				"content":         "test message",
			},
		},
		"next_token": "page2",
	}
	env, err := douyinshop.ParseCustomerMessageEnvelope(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !env.Synthetic {
		t.Error("expected synthetic=true")
	}
	if len(env.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(env.Messages))
	}
	if env.Messages[0].MessageID != "msg-001" {
		t.Errorf("expected message_id=msg-001, got %s", env.Messages[0].MessageID)
	}
	if env.NextToken != "page2" {
		t.Errorf("expected next_token=page2, got %s", env.NextToken)
	}
}
