package ordersync

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
	"gorm.io/gorm"
)

// DouyinOrderWebhookHandler upserts orders from Douyin webhook events via the unified order service.
type DouyinOrderWebhookHandler struct {
	DB     *gorm.DB
	Shops  *shop.Service
	Orders *order.Service
}

// HandleDouyinOrderEvent implements webhook.OrderEventHandler.
func (h *DouyinOrderWebhookHandler) HandleDouyinOrderEvent(ctx context.Context, ev *platformdouyin.NormalizedWebhookEvent) error {
	if h == nil || h.Orders == nil {
		return fmt.Errorf("ordersync: douyin order webhook handler not configured")
	}
	mapped, err := platformdouyin.MapDouyinOrderWebhookEvent(ev)
	if err != nil {
		return err
	}
	shopID, shopPlatform, err := h.resolveDouyinShop(ctx, mapped.ShopIDHint)
	if err != nil {
		return err
	}

	payload := ToSyncedPayloads([]platformp.PlatformOrder{mapped.NormalizedOrder})
	if len(payload) == 0 {
		return platformdouyin.NewError(platformdouyin.CodeDouyinOrderEventInvalid, "empty normalized order payload", "", "", "")
	}

	res, upErr := h.Orders.UpsertPlatformOrder(ctx, order.PlatformOrderUpsertInput{
		Platform:          shopPlatform,
		ShopID:            shopID,
		PlatformOrderID:   mapped.PlatformOrderID,
		PlatformUpdatedAt: mapped.PlatformUpdatedAt,
		PlatformRevision:  mapped.PlatformRevision,
		EventType:         mapped.EventType,
		EventID:           mapped.EventID,
		Source:            order.UpsertSourceWebhook,
		NormalizedOrder:   payload[0],
	})
	if upErr != nil {
		return upErr
	}
	if res != nil && res.StaleIgnored {
		slog.InfoContext(ctx, "douyin order webhook stale event ignored",
			"shopId", shopID.String(),
			"platformOrderId", mapped.PlatformOrderID,
			"eventType", mapped.EventType,
			"eventId", mapped.EventID,
			"code", res.ResponseCode)
		return nil
	}
	if res != nil && res.OrderID != uuid.Nil && !res.StaleIgnored && !res.Replayed {
		if _, err := h.Orders.MatchOrderItemsForOrder(ctx, res.OrderID, order.MatchOrderItemsOptions{
			Source: "douyin_order_webhook",
		}); err != nil {
			slog.WarnContext(ctx, "douyin order webhook sku match failed",
				"orderId", res.OrderID.String(), "err", err.Error())
		}
	}
	return nil
}

func (h *DouyinOrderWebhookHandler) resolveDouyinShop(ctx context.Context, shopHint string) (uuid.UUID, string, error) {
	if h == nil || h.DB == nil {
		return uuid.Nil, "", fmt.Errorf("ordersync: no db")
	}
	hint := strings.TrimSpace(shopHint)
	if hint != "" {
		if sid, err := uuid.Parse(hint); err == nil {
			var row shop.Shop
			if err := h.DB.WithContext(ctx).First(&row, "id = ?", sid).Error; err == nil {
				if isDouyinPlatform(row.Platform) {
					return row.ID, row.Platform, nil
				}
			}
		}
		var byExt shop.Shop
		q := h.DB.WithContext(ctx).Where("platform IN ? AND external_shop_id = ?", douyinPlatforms(), hint)
		if err := q.First(&byExt).Error; err == nil {
			return byExt.ID, byExt.Platform, nil
		}
	}

	var shops []shop.Shop
	if err := h.DB.WithContext(ctx).
		Where("platform IN ? AND auth_status = ?", douyinPlatforms(), shop.AuthAuthorized).
		Find(&shops).Error; err != nil {
		return uuid.Nil, "", err
	}
	if len(shops) == 1 {
		return shops[0].ID, shops[0].Platform, nil
	}
	if len(shops) == 0 {
		return uuid.Nil, "", platformdouyin.NewError(platformdouyin.CodeDouyinStoreNotAuthorized,
			"no authorized douyin shop for webhook order event", "", "shop_not_found", "")
	}
	ids := make([]string, 0, len(shops))
	for _, s := range shops {
		ids = append(ids, s.ID.String())
	}
	summary, _ := json.Marshal(map[string]any{"authorizedShopIds": ids})
	return uuid.Nil, "", platformdouyin.NewError(platformdouyin.CodeDouyinValidationFailed,
		fmt.Sprintf("ambiguous douyin shop for webhook: %s", string(summary)), "", "shop_ambiguous", "")
}

func isDouyinPlatform(p string) bool {
	p = strings.TrimSpace(strings.ToLower(p))
	return p == "douyin_shop" || p == "douyin"
}

func douyinPlatforms() []string {
	return []string{"douyin_shop", "douyin"}
}
