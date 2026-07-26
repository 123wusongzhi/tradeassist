package douyinshop

import (
	"context"
	"fmt"
	"strings"
)

// GetSKUStockFromDetail reads SKU stock using product.detail (official read path).
//
// Design decision: Douyin does not expose a dedicated stock-query-only API in the
// standard OpenAPI contract; product.detail spec_prices contains stock fields.
// This function reuses GetProductDetail to avoid inventing undocumented endpoints.
//
// If platformSKUID is non-empty, the matching SKU row's stock is returned.
// If platformSKUID is empty, the sum of all SKU stocks is returned.
func GetSKUStockFromDetail(ctx context.Context, c *Client, shopID, platformProductID, platformSKUID string) (int, error) {
	platformProductID = strings.TrimSpace(platformProductID)
	if platformProductID == "" {
		return 0, NewError(CodeDouyinValidationFailed, "platform_product_id is required for stock query", "", "", "")
	}

	detail, err := c.GetProductDetail(ctx, shopID, platformProductID)
	if err != nil {
		return 0, err
	}
	if detail == nil {
		return 0, NewError(CodeDouyinResourceNotFound,
			fmt.Sprintf("product %s not found on platform", platformProductID), "", "", "")
	}

	skuID := strings.TrimSpace(platformSKUID)
	if skuID != "" {
		for _, sku := range detail.SKUs {
			if strings.TrimSpace(sku.PlatformSKUID) == skuID {
				return sku.Stock, nil
			}
		}
		return 0, NewError(CodeDouyinResourceNotFound,
			fmt.Sprintf("sku %s not found in product.detail for product %s", skuID, platformProductID), "", "", "")
	}

	// Sum all SKUs
	total := 0
	for _, sku := range detail.SKUs {
		total += sku.Stock
	}
	return total, nil
}
