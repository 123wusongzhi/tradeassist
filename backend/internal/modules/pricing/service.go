package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

// Service applies publish pricing rules to local product_skus.price only.
type Service struct {
	DB       *gorm.DB
	Settings *settings.Service
	OpLog    *operationlog.Service
}

func (s *Service) defaultRule(ctx context.Context, platform string, override Rule) (Rule, string, error) {
	curr := "CNY"
	if s == nil || s.Settings == nil {
		r := RuleFromSettingsMap(nil)
		return MergeRule(r, override), curr, nil
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "pricing")
	if err != nil {
		return Rule{}, curr, err
	}
	if v := strings.TrimSpace(m["default_currency"]); v != "" {
		curr = v
	}
	base := RuleFromSettingsMap(settings.PricingRuleFromMap(m, platform))
	return MergeRule(base, override), curr, nil
}

func (s *Service) batchMax(ctx context.Context) int {
	if s == nil || s.Settings == nil {
		return 500
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "pricing")
	if err != nil {
		return 500
	}
	return settings.PricingBatchMaxFromMap(m)
}

func skuBasePrice(row product.ProductSKU) float64 {
	if row.CostPrice != nil && *row.CostPrice > 0 {
		return *row.CostPrice
	}
	if row.Price != nil && *row.Price > 0 {
		return *row.Price
	}
	return 0
}

func buildPreviewLine(row product.ProductSKU, prodID uuid.UUID, calc CalculateResult) PreviewLine {
	cur := row.Price
	delta := calc.CalculatedPrice
	if cur != nil {
		delta = calc.CalculatedPrice - *cur
	}
	return PreviewLine{
		ProductSkuID:        row.ID.String(),
		ProductID:           prodID.String(),
		SKUCode:             row.SKUCode,
		SKUName:             row.SKUName,
		CostPrice:           row.CostPrice,
		CurrentPrice:        cur,
		LandedCost:          calc.LandedCost,
		CommissionFee:       calc.CommissionFee,
		CalculatedPrice:     calc.CalculatedPrice,
		EstimatedProfit:     calc.EstimatedProfit,
		ProfitMarginPercent: calc.ProfitMarginPercent,
		Delta:               roundMoney(delta),
	}
}

// Calculate runs a single SKU or explicit price trial.
func (s *Service) Calculate(c *gin.Context, tenantID int64, body CalculateBody) (*CalculateResponse, error) {
	ctx := c.Request.Context()
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("pricing unavailable")
	}
	rule, curr, err := s.defaultRule(ctx, body.Platform, body.Rule)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(body.Currency) != "" {
		curr = strings.TrimSpace(body.Currency)
	}

	var cost, current *float64
	base := 0.0
	if body.ProductSkuID != nil && *body.ProductSkuID != uuid.Nil {
		var row product.ProductSKU
		if err := s.DB.WithContext(ctx).Table("product_skus").
			Select("product_skus.*").
			Joins("JOIN products ON products.id = product_skus.product_id").
			Where("product_skus.id = ? AND products.tenant_id = ?", *body.ProductSkuID, tenantID).
			First(&row).Error; err != nil {
			return nil, err
		}
		// ProductSKU has no tenant id; authorize its parent before returning
		// price/cost inputs to a scoped principal.
		if err := adminperm.EnsureProductVisible(c, s.DB, row.ProductID); err != nil {
			return nil, err
		}
		cost = row.CostPrice
		current = row.Price
		base = skuBasePrice(row)
		if body.BasePrice != nil {
			base = *body.BasePrice
		}
		if body.CostPrice != nil {
			cost = body.CostPrice
			if *body.CostPrice > 0 {
				base = *body.CostPrice
			}
		}
	} else {
		if body.CostPrice != nil && *body.CostPrice > 0 {
			cost = body.CostPrice
			base = *body.CostPrice
		} else if body.BasePrice != nil {
			base = *body.BasePrice
		}
	}
	if body.BasePrice != nil && body.ProductSkuID == nil {
		base = *body.BasePrice
	}

	calc := CalculatePublishPrice(CalculateInput{
		BasePrice:       base,
		CostPrice:       cost,
		CurrentPrice:    current,
		MinPublishPrice: rowMinPub(body, rule),
		Rule:            rule,
	}, curr)
	return &CalculateResponse{
		BasePrice:           calc.BasePrice,
		CostPrice:           calc.CostPrice,
		CurrentPrice:        calc.CurrentPrice,
		LandedCost:          calc.LandedCost,
		ShippingCost:        calc.ShippingCost,
		CommissionFee:       calc.CommissionFee,
		CalculatedPrice:     calc.CalculatedPrice,
		EstimatedProfit:     calc.EstimatedProfit,
		ProfitMarginPercent: calc.ProfitMarginPercent,
		Currency:            calc.Currency,
	}, nil
}

func rowMinPub(body CalculateBody, rule Rule) *float64 {
	if rule.MinPublishPrice != nil {
		return rule.MinPublishPrice
	}
	return nil
}

type applyPlan struct {
	skuID       uuid.UUID
	productID   uuid.UUID
	newPrice    float64
	previewLine PreviewLine
}

func (s *Service) planForSKU(row product.ProductSKU, prodID uuid.UUID, rule Rule, currency string) applyPlan {
	base := skuBasePrice(row)
	calc := CalculatePublishPrice(CalculateInput{
		BasePrice:       base,
		CostPrice:       row.CostPrice,
		CurrentPrice:    row.Price,
		MinPublishPrice: coalesceMinPub(rule.MinPublishPrice, row.MinPublishPrice),
		Rule:            rule,
	}, currency)
	return applyPlan{
		skuID:       row.ID,
		productID:   prodID,
		newPrice:    calc.CalculatedPrice,
		previewLine: buildPreviewLine(row, prodID, calc),
	}
}

func coalesceMinPub(a, b *float64) *float64 {
	if a != nil && *a > 0 {
		return a
	}
	if b != nil && *b > 0 {
		return b
	}
	return nil
}

func (s *Service) loadProductSKUs(ctx context.Context, tenantID int64, productID uuid.UUID, skuIDs []uuid.UUID) ([]product.ProductSKU, error) {
	tx := s.DB.WithContext(ctx).Table("product_skus").
		Select("product_skus.*").
		Joins("JOIN products ON products.id = product_skus.product_id").
		Where("product_skus.product_id = ? AND products.tenant_id = ?", productID, tenantID)
	if len(skuIDs) > 0 {
		tx = tx.Where("product_skus.id IN ?", skuIDs)
	}
	var rows []product.ProductSKU
	if err := tx.Order("product_skus.created_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ApplyProduct updates SKU prices for one product when confirm=true.
func (s *Service) ApplyProduct(c *gin.Context, tenantID int64, productID uuid.UUID, body ProductApplyBody, adminID *uuid.UUID) (*ApplySummary, error) {
	ctx := c.Request.Context()
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("pricing unavailable")
	}
	if !body.Confirm {
		return nil, fmt.Errorf("confirm is required to apply pricing")
	}
	if err := adminperm.EnsureProductOperate(c, s.DB, productID); err != nil {
		return nil, err
	}
	var probe product.Product
	if err := s.DB.WithContext(ctx).Select("id", "currency").First(&probe, "id = ? AND tenant_id = ?", productID, tenantID).Error; err != nil {
		return nil, err
	}
	rule, curr, err := s.defaultRule(ctx, body.Platform, body.Rule)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(probe.Currency) != "" {
		curr = strings.TrimSpace(probe.Currency)
	}
	rows, err := s.loadProductSKUs(ctx, tenantID, productID, body.SkuIDs)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return &ApplySummary{ProductCount: 1}, nil
	}
	max := s.batchMax(ctx)
	if len(rows) > max {
		return nil, fmt.Errorf("too many SKUs in one apply (max %d)", max)
	}
	plans := make([]applyPlan, 0, len(rows))
	for _, row := range rows {
		plans = append(plans, s.planForSKU(row, productID, rule, curr))
	}
	updated, err := s.commitPlans(ctx, tenantID, plans)
	if err != nil {
		return nil, err
	}
	sum := &ApplySummary{
		ProductCount: 1,
		SkuCount:     len(rows),
		Updated:      updated,
		Skipped:      len(rows) - updated,
	}
	s.writeApplyOpLog(ctx, adminID, "pricing.product.apply", productID.String(), body.Platform, rule, sum)
	return sum, nil
}

// PreviewProduct returns calculated lines without persisting.
func (s *Service) PreviewProduct(c *gin.Context, tenantID int64, productID uuid.UUID, body ProductApplyBody) (*ApplySummary, error) {
	ctx := c.Request.Context()
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("pricing unavailable")
	}
	if err := adminperm.EnsureProductVisible(c, s.DB, productID); err != nil {
		return nil, err
	}
	var probe product.Product
	if err := s.DB.WithContext(ctx).Select("id", "currency").First(&probe, "id = ? AND tenant_id = ?", productID, tenantID).Error; err != nil {
		return nil, err
	}
	rule, curr, err := s.defaultRule(ctx, body.Platform, body.Rule)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(probe.Currency) != "" {
		curr = strings.TrimSpace(probe.Currency)
	}
	rows, err := s.loadProductSKUs(ctx, tenantID, productID, body.SkuIDs)
	if err != nil {
		return nil, err
	}
	preview := make([]PreviewLine, 0, len(rows))
	for _, row := range rows {
		p := s.planForSKU(row, productID, rule, curr)
		preview = append(preview, p.previewLine)
	}
	return &ApplySummary{
		ProductCount: 1,
		SkuCount:     len(rows),
		Preview:      preview,
	}, nil
}

func batchScopePresent(body BatchApplyBody) bool {
	if len(body.ProductIDs) > 0 {
		return true
	}
	f := body.Filters
	if strings.TrimSpace(f.Status) != "" {
		return true
	}
	if strings.TrimSpace(f.Source) != "" {
		return true
	}
	if strings.TrimSpace(f.Keyword) != "" {
		return true
	}
	return false
}

func (s *Service) resolveProductIDs(c *gin.Context, tenantID int64, body BatchApplyBody) ([]uuid.UUID, error) {
	ctx := c.Request.Context()
	tx := s.DB.WithContext(ctx).Model(&product.Product{}).Where("tenant_id = ?", tenantID)
	var err error
	if tx, err = adminperm.ApplyProductScopeAlias(c, s.DB, tx, "products"); err != nil {
		return nil, err
	}
	if len(body.ProductIDs) > 0 {
		ids := make([]uuid.UUID, 0, len(body.ProductIDs))
		for _, id := range body.ProductIDs {
			if id != uuid.Nil {
				ids = append(ids, id)
			}
		}
		tx = tx.Where("id IN ?", ids)
	}
	f := body.Filters
	if v := strings.TrimSpace(f.Status); v != "" {
		tx = tx.Where("status = ?", v)
	}
	if v := strings.TrimSpace(f.Source); v != "" {
		tx = tx.Where("source = ?", v)
	}
	if v := strings.TrimSpace(f.Keyword); v != "" {
		pat := "%" + strings.ToLower(v) + "%"
		tx = tx.Where("LOWER(title) LIKE ? OR LOWER(original_title) LIKE ?", pat, pat)
	}
	var ids []uuid.UUID
	if err := tx.Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// BatchApply updates SKU prices across multiple products.
func (s *Service) BatchApply(c *gin.Context, tenantID int64, body BatchApplyBody, adminID *uuid.UUID) (*ApplySummary, error) {
	ctx := c.Request.Context()
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("pricing unavailable")
	}
	if !batchScopePresent(body) && !body.ConfirmAll {
		return nil, fmt.Errorf("empty filters: set productIds/filters or confirmAll=true")
	}
	if !body.Confirm {
		return nil, fmt.Errorf("confirm is required to apply pricing")
	}
	productIDs, err := s.resolveProductIDs(c, tenantID, body)
	if err != nil {
		return nil, err
	}
	// Explicit selections are an all-or-nothing write request.  Do not quietly
	// apply the visible subset when another selected product is out of scope.
	if len(body.ProductIDs) > 0 {
		requested := make(map[uuid.UUID]struct{}, len(body.ProductIDs))
		for _, id := range body.ProductIDs {
			if id != uuid.Nil {
				requested[id] = struct{}{}
			}
		}
		if len(requested) != len(productIDs) {
			return nil, gorm.ErrRecordNotFound
		}
	}
	if len(productIDs) == 0 {
		return &ApplySummary{}, nil
	}
	for _, pid := range productIDs {
		if err := adminperm.EnsureProductOperate(c, s.DB, pid); err != nil {
			return nil, err
		}
	}
	rule, curr, err := s.defaultRule(ctx, body.Platform, body.Rule)
	if err != nil {
		return nil, err
	}
	max := s.batchMax(ctx)
	var allPlans []applyPlan
	productCount := 0
	for _, pid := range productIDs {
		var probe product.Product
		if err := s.DB.WithContext(ctx).Select("id", "currency").First(&probe, "id = ? AND tenant_id = ?", pid, tenantID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				continue
			}
			return nil, err
		}
		productCount++
		c := curr
		if strings.TrimSpace(probe.Currency) != "" {
			c = strings.TrimSpace(probe.Currency)
		}
		rows, err := s.loadProductSKUs(ctx, tenantID, pid, nil)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			allPlans = append(allPlans, s.planForSKU(row, pid, rule, c))
			if len(allPlans) > max {
				return nil, fmt.Errorf("batch exceeds max SKU count (%d)", max)
			}
		}
	}
	updated, err := s.commitPlans(ctx, tenantID, allPlans)
	if err != nil {
		return nil, err
	}
	sum := &ApplySummary{
		ProductCount: productCount,
		SkuCount:     len(allPlans),
		Updated:      updated,
		Skipped:      len(allPlans) - updated,
	}
	s.writeApplyOpLog(ctx, adminID, "pricing.batch_apply", "", body.Platform, rule, sum)
	return sum, nil
}

// BatchPreview returns trial lines without persisting.
func (s *Service) BatchPreview(c *gin.Context, tenantID int64, body BatchApplyBody) (*ApplySummary, error) {
	ctx := c.Request.Context()
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("pricing unavailable")
	}
	if !batchScopePresent(body) && !body.ConfirmAll {
		return nil, fmt.Errorf("empty filters: set productIds/filters or confirmAll=true")
	}
	productIDs, err := s.resolveProductIDs(c, tenantID, body)
	if err != nil {
		return nil, err
	}
	rule, curr, err := s.defaultRule(ctx, body.Platform, body.Rule)
	if err != nil {
		return nil, err
	}
	max := s.batchMax(ctx)
	preview := make([]PreviewLine, 0)
	productCount := 0
	for _, pid := range productIDs {
		var probe product.Product
		if err := s.DB.WithContext(ctx).Select("id", "currency").First(&probe, "id = ? AND tenant_id = ?", pid, tenantID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				continue
			}
			return nil, err
		}
		productCount++
		c := curr
		if strings.TrimSpace(probe.Currency) != "" {
			c = strings.TrimSpace(probe.Currency)
		}
		rows, err := s.loadProductSKUs(ctx, tenantID, pid, nil)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			p := s.planForSKU(row, pid, rule, c)
			preview = append(preview, p.previewLine)
			if len(preview) > max {
				return nil, fmt.Errorf("batch exceeds max SKU count (%d)", max)
			}
		}
	}
	return &ApplySummary{
		ProductCount: productCount,
		SkuCount:     len(preview),
		Preview:      preview,
	}, nil
}

func (s *Service) commitPlans(ctx context.Context, tenantID int64, plans []applyPlan) (int, error) {
	updated := 0
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, p := range plans {
			if p.newPrice <= 0 {
				continue
			}
			res := tx.Model(&product.ProductSKU{}).
				Where("id = ? AND product_id = ?", p.skuID, p.productID).
				Where("EXISTS (SELECT 1 FROM products WHERE products.id = product_skus.product_id AND products.tenant_id = ?)", tenantID).
				Update("price", p.newPrice)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected > 0 {
				updated++
			}
		}
		return nil
	})
	return updated, err
}

func (s *Service) writeApplyOpLog(ctx context.Context, adminID *uuid.UUID, action, productID, platform string, rule Rule, sum *ApplySummary) {
	if s == nil || s.OpLog == nil || sum == nil {
		return
	}
	payload := map[string]any{
		"productId":                 productID,
		"skuCount":                  sum.SkuCount,
		"updated":                   sum.Updated,
		"platform":                  strings.TrimSpace(platform),
		"markupType":                rule.MarkupType,
		"markupPercent":             rule.MarkupPercent,
		"markupAmount":              rule.MarkupAmount,
		"markupMultiplier":          rule.MarkupMultiplier,
		"shippingCost":              rule.ShippingCost,
		"platformCommissionPercent": rule.PlatformCommissionPercent,
		"minProfit":                 rule.MinProfit,
		"roundingMode":              rule.RoundingMode,
	}
	b, _ := json.Marshal(payload)
	_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
		AdminUserID: adminID,
		Action:      action,
		Resource:    "pricing",
		ResourceID:  productID,
		Status:      "success",
		Message:     string(b),
	})
}
