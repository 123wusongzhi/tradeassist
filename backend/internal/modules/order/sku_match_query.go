package order

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

// ListSKUMatchRowsForOrder returns one DTO per order line merged with match row.
func (s *Service) ListSKUMatchRowsForOrder(c *gin.Context, orderID uuid.UUID) ([]SKUMatchDetailDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	if _, err := s.findOrderBare(c, orderID); err != nil {
		return nil, err
	}
	var items []OrderItem
	if err := tenantOrderScope(s.DB.WithContext(c.Request.Context()), orderID, tid).Order("created_at ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	out := make([]SKUMatchDetailDTO, 0, len(items))
	for _, it := range items {
		var mrow OrderItemSKUMatch
		if err := s.DB.WithContext(c.Request.Context()).Where("order_item_id = ?", it.ID).First(&mrow).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		dto := SKUMatchDetailDTO{
			OrderItemSKUMatch: mrow,
			ProductTitle:      strings.TrimSpace(it.ProductTitle),
			CandidateSKUs:     parseCandidatesFromRaw(mrow.RawData),
		}
		if it.ProductSKUID != nil && *it.ProductSKUID != uuid.Nil {
			var loc product.ProductSKU
			if err := s.DB.WithContext(c.Request.Context()).Model(&product.ProductSKU{}).
				Joins("JOIN products p ON p.id = product_skus.product_id AND p.deleted_at IS NULL AND p.tenant_id = ?", tid).
				First(&loc, "product_skus.id = ?", *it.ProductSKUID).Error; err == nil {
				dto.LocalSkuCode = strings.TrimSpace(loc.SKUCode)
			}
		}
		dto.CandidateSKUs = s.filterCandidatesForTenant(c.Request.Context(), tid, dto.CandidateSKUs)
		out = append(out, dto)
	}
	return out, nil
}

func parseCandidatesFromRaw(raw []byte) []SKUCandidateDTO {
	if len(raw) == 0 {
		return nil
	}
	var wrap map[string]any
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return nil
	}
	cand, ok := wrap["candidates"].([]any)
	if !ok || len(cand) == 0 {
		return nil
	}
	var out []SKUCandidateDTO
	for _, e := range cand {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		sidStr, _ := m["productSkuId"].(string)
		pidStr, _ := m["productId"].(string)
		sid, err1 := uuid.Parse(sidStr)
		pid, err2 := uuid.Parse(pidStr)
		if err1 != nil || err2 != nil {
			continue
		}
		sc, _ := m["skuCode"].(string)
		sn, _ := m["skuName"].(string)
		pt, _ := m["productTitle"].(string)
		out = append(out, SKUCandidateDTO{
			ProductSKUID: sid,
			ProductID:    pid,
			SKUCode:      sc,
			SKUName:      sn,
			ProductTitle: pt,
		})
	}
	return out
}

// ListSKUMatchGlobal returns paginated audit rows with shop/order labels.
func (s *Service) ListSKUMatchGlobal(c *gin.Context, q SKUMatchListQuery) ([]SKUMatchListRow, int64, error) {
	if s == nil || s.DB == nil {
		return nil, 0, fmt.Errorf("order: no db")
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	ps := q.PageSize
	if ps < 1 {
		ps = 20
	}
	if ps > 100 {
		ps = 100
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, 0, err
	}
	tx := s.DB.WithContext(c.Request.Context()).Model(&OrderItemSKUMatch{}).
		Joins("JOIN orders o ON o.id = order_item_sku_matches.order_id AND o.deleted_at IS NULL AND o.tenant_id = ?", tid)
	tx, err = adminperm.ApplyStoreScope(c, s.DB, tx, "o.shop_id")
	if err != nil {
		return nil, 0, err
	}
	if v := strings.TrimSpace(q.Platform); v != "" {
		tx = tx.Where("platform = ?", v)
	}
	if v := strings.TrimSpace(q.MatchStatus); v != "" {
		tx = tx.Where("match_status = ?", v)
	}
	if v := strings.TrimSpace(q.MatchType); v != "" {
		tx = tx.Where("match_type = ?", v)
	}
	if q.OrderID != nil && *q.OrderID != uuid.Nil {
		tx = tx.Where("order_id = ?", *q.OrderID)
	}
	if q.ProductSKUID != nil && *q.ProductSKUID != uuid.Nil {
		tx = tx.Where("product_sku_id = ?", *q.ProductSKUID)
	}
	if q.Start != nil {
		tx = tx.Where("created_at >= ?", *q.Start)
	}
	if q.End != nil {
		tx = tx.Where("created_at <= ?", *q.End)
	}
	if q.ShopID != nil && *q.ShopID != uuid.Nil {
		tx = tx.Where("o.shop_id = ?", *q.ShopID)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var matches []OrderItemSKUMatch
	if err := tx.Order("created_at DESC, id DESC").Offset((page - 1) * ps).Limit(ps).Find(&matches).Error; err != nil {
		return nil, 0, err
	}
	out := make([]SKUMatchListRow, 0, len(matches))
	for _, m := range matches {
		row := SKUMatchListRow{OrderItemSKUMatch: m}
		var o Order
		if err := s.DB.WithContext(c.Request.Context()).First(&o, "id = ? AND tenant_id = ? AND deleted_at IS NULL", m.OrderID, tid).Error; err == nil {
			row.OrderNo = o.OrderNo
			if o.ShopID != nil && s.Shops != nil {
				if sum, _ := s.Shops.GetSummary(c, *o.ShopID); sum != nil {
					row.ShopName = sum.ShopName
				}
			}
		}
		var oi OrderItem
		if err := s.DB.WithContext(c.Request.Context()).First(&oi, "id = ? AND order_id = ?", m.OrderItemID, m.OrderID).Error; err == nil {
			row.LineProductTitle = oi.ProductTitle
		}
		if m.ProductSKUID != nil && *m.ProductSKUID != uuid.Nil {
			var psku product.ProductSKU
			if err := s.DB.WithContext(c.Request.Context()).Model(&product.ProductSKU{}).
				Joins("JOIN products p ON p.id = product_skus.product_id AND p.deleted_at IS NULL AND p.tenant_id = ?", tid).
				First(&psku, "product_skus.id = ?", *m.ProductSKUID).Error; err == nil {
				row.LocalSkuCode = strings.TrimSpace(psku.SKUCode)
			}
		}
		out = append(out, row)
	}
	return out, total, nil
}

// filterCandidatesForTenant prevents stale/corrupt raw match metadata from
// enriching an otherwise tenant-scoped response with another tenant's SKU.
func (s *Service) filterCandidatesForTenant(ctx context.Context, tenantID int64, in []SKUCandidateDTO) []SKUCandidateDTO {
	out := make([]SKUCandidateDTO, 0, len(in))
	for _, candidate := range in {
		var count int64
		err := s.DB.WithContext(ctx).Table("product_skus AS ps").
			Joins("JOIN products p ON p.id = ps.product_id AND p.deleted_at IS NULL AND p.tenant_id = ?", tenantID).
			Where("ps.id = ? AND ps.product_id = ?", candidate.ProductSKUID, candidate.ProductID).
			Count(&count).Error
		if err == nil && count == 1 {
			out = append(out, candidate)
		}
	}
	return out
}

// BindOrderItemSKUInput is the service input for manual bind.
type BindOrderItemSKUInput struct {
	OrderItemID         uuid.UUID
	ProductSKUID        uuid.UUID
	CandidateConfidence *int
	CandidateSource     string
}

type requestTenantContextKey struct{}

func orderRequestContext(c *gin.Context) (context.Context, error) {
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	return context.WithValue(c.Request.Context(), requestTenantContextKey{}, tid), nil
}

func requestTenantID(ctx context.Context) (int64, bool) {
	tid, ok := ctx.Value(requestTenantContextKey{}).(int64)
	return tid, ok
}

func tenantOrderItemScope(tx *gorm.DB, tenantID int64) *gorm.DB {
	return tx.Where("EXISTS (SELECT 1 FROM orders WHERE orders.id = order_items.order_id AND orders.tenant_id = ? AND orders.deleted_at IS NULL)", tenantID)
}

// BindOrderItemSKU performs manual binding (inventory deduct handled in HTTP layer).
func (s *Service) BindOrderItemSKU(ctx context.Context, in BindOrderItemSKUInput, admin *uuid.UUID) (*OrderItem, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	itemQuery := s.DB.WithContext(ctx)
	tenantID, requestScoped := requestTenantID(ctx)
	if requestScoped {
		itemQuery = tenantOrderItemScope(itemQuery, tenantID)
	}
	var it OrderItem
	if err := itemQuery.First(&it, "id = ?", in.OrderItemID).Error; err != nil {
		return nil, err
	}
	var o Order
	orderQuery := s.DB.WithContext(ctx)
	if requestScoped {
		orderQuery = orderQuery.Where("tenant_id = ?", tenantID)
	}
	if err := orderQuery.First(&o, "id = ? AND deleted_at IS NULL", it.OrderID).Error; err != nil {
		return nil, err
	}
	sku, err := s.LoadSKUForBind(ctx, o.TenantID, in.ProductSKUID)
	if err != nil {
		return nil, err
	}
	pid := sku.ProductID
	sid := sku.ID

	now := time.Now().UTC()
	if err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		update := tx.Model(&it)
		if requestScoped {
			update = tenantOrderItemScope(update, tenantID)
		}
		updated := update.Updates(map[string]any{
			"product_id":     &pid,
			"product_sku_id": &sid,
			"updated_at":     now,
		})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		it.ProductID = &pid
		it.ProductSKUID = &sid
		res := &MatchOrderItemResult{
			MatchType:        MatchTypeManual,
			MatchStatus:      MatchStatusManualBound,
			Confidence:       100,
			Reason:           "manual_bind",
			ProductID:        &pid,
			ProductSKUID:     &sid,
			UpdateOrderLines: false,
			RawData: map[string]any{
				"productSkuId": sid.String(),
				"productId":    pid.String(),
			},
		}
		row, err := matchRowFromResult(&o, &it, res, admin)
		if err != nil {
			return err
		}
		return upsertSKUMatchRowTx(tx, row)
	}); err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		msg := fmt.Sprintf("orderId=%s orderItemId=%s productSkuId=%s matchType=manual",
			it.OrderID.String(), it.ID.String(), sid.String())
		if in.CandidateConfidence != nil {
			msg += fmt.Sprintf(" candidateConfidence=%d", *in.CandidateConfidence)
		}
		if cs := strings.TrimSpace(in.CandidateSource); cs != "" {
			msg += " candidateSource=" + cs
		}
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: admin,
			Action:      "order.sku_match.manual_bind",
			Resource:    "order_item",
			ResourceID:  it.ID.String(),
			Status:      "success",
			Message:     msg,
		})
	}
	var out OrderItem
	if err := itemQuery.First(&out, "id = ?", in.OrderItemID).Error; err != nil {
		return nil, err
	}
	return &out, nil
}

// GetOrderItemByID loads a single order line.
func (s *Service) GetOrderItemByID(c *gin.Context, itemID uuid.UUID) (*OrderItem, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var it OrderItem
	if err := s.DB.WithContext(c.Request.Context()).Joins("JOIN orders ON orders.id = order_items.order_id AND orders.deleted_at IS NULL").Where("order_items.id = ? AND orders.tenant_id = ?", itemID, tid).First(&it).Error; err != nil {
		return nil, err
	}
	if _, err := s.findOrderBare(c, it.OrderID); err != nil {
		return nil, err
	}
	return &it, nil
}
