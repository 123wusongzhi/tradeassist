package idempotency

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// Key builders use stable business semantics; never embed secrets or PII.

func OrderSync(platform, shopID, platformOrderID string) string {
	return fmt.Sprintf("order-sync:%s:%s:%s",
		norm(platform), norm(shopID), norm(platformOrderID))
}

func InventoryDeduct(orderID, orderItemID, skuID string) string {
	return fmt.Sprintf("inventory-deduct:%s:%s:%s",
		norm(orderID), norm(orderItemID), norm(skuID))
}

func InventoryPush(shopID, skuID, stockVersion string) string {
	return fmt.Sprintf("inventory-push:%s:%s:%s",
		norm(shopID), norm(skuID), norm(stockVersion))
}

func CustomerSend(conversationID, clientMessageID string) string {
	return fmt.Sprintf("customer-send:%s:%s",
		norm(conversationID), norm(clientMessageID))
}

func PublishDraft(shopID, productDraftID, publishVersion string) string {
	return fmt.Sprintf("publish-draft:%s:%s:%s",
		norm(shopID), norm(productDraftID), norm(publishVersion))
}

func AITextBatch(productID, contentHash, operationType string) string {
	return fmt.Sprintf("ai-text-batch:%s:%s:%s",
		norm(productID), norm(contentHash), norm(operationType))
}

func AIImageBatch(productID, imageHash, operationType string) string {
	return fmt.Sprintf("ai-image-batch:%s:%s:%s",
		norm(productID), norm(imageHash), norm(operationType))
}

func Webhook(platform, eventID string) string {
	return fmt.Sprintf("webhook:%s:%s", norm(platform), norm(eventID))
}

// HashRequest returns a stable SHA-256 hex digest of normalized request payload bytes.
func HashRequest(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func norm(s string) string {
	return strings.TrimSpace(s)
}
