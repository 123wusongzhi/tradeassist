package douyinshop_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
)

func TestMapDouyinOrderWebhookEventFromFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "testdata", "douyin", "webhook_order_created.json"))
	if err != nil {
		t.Fatal(err)
	}
	var items platformdouyin.JinriteimaiPushEnvelope
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 {
		t.Fatal("empty fixture")
	}
	ev := platformdouyin.NormalizeJinriteimaiItem(items[0], raw)
	mapped, err := platformdouyin.MapDouyinOrderWebhookEvent(ev)
	if err != nil {
		t.Fatal(err)
	}
	if mapped.PlatformOrderID == "" {
		t.Fatal("missing platform order id")
	}
	if mapped.PlatformUpdatedAt == nil {
		t.Fatal("missing platform updated at")
	}
	if mapped.EventType != "order_created" {
		t.Fatalf("event type: %s", mapped.EventType)
	}
}

func TestMapDouyinOrderWebhookEventMissingOrderID(t *testing.T) {
	ev := &platformdouyin.NormalizedWebhookEvent{
		EventType: "order_paid",
		MsgID:     "m1",
		Data:      map[string]any{"order_status": "105"},
	}
	_, err := platformdouyin.MapDouyinOrderWebhookEvent(ev)
	if err == nil {
		t.Fatal("expected error")
	}
	var de *platformdouyin.Error
	if !platformdouyin.AsError(err, &de) || de.Code != platformdouyin.CodeDouyinOrderEventMissingOrderID {
		t.Fatalf("got %v", err)
	}
}

func TestContractGateBlockedIM(t *testing.T) {
	gate := platformdouyin.NewDefaultContractGate("development")
	if err := gate.Require(platformdouyin.CapDouyinIMSend); err == nil {
		t.Fatal("expected block")
	}
	st := gate.Status(platformdouyin.CapDouyinIMSend)
	if st.Status != platformdouyin.ContractStatusBlockedByContractVerification {
		t.Fatalf("status=%s", st.Status)
	}
}

func TestContractGateProductionWebhookSignature(t *testing.T) {
	gate := platformdouyin.NewDefaultContractGate("production")
	if err := gate.Require(platformdouyin.CapDouyinWebhookSignatureV1); err == nil {
		t.Fatal("fixture_verified must block production webhook signature")
	}
}
