package douyinshop

import (
	"context"
	"time"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

// DouyinProvider is a thin facade exposing all P3 capability groups.
// All methods delegate to *Client — no business logic lives here.
type DouyinProvider interface {
	// Auth / token
	EnsureFreshToken(ctx context.Context) (string, error)
	RefreshToken(ctx context.Context) (*TokenBundle, error)

	// Shop info
	GetShopInfo(ctx context.Context, shopID string) (*ShopInfo, error)

	// Catalog
	GetCategoryList(ctx context.Context, parentID string) ([]Category, error)
	GetCategoryRequiredAttributes(ctx context.Context, categoryID string) ([]CategoryAttribute, error)

	// Image upload
	UploadImageFromURL(ctx context.Context, imageURL, shopID string) (*PlatformImage, error)
	UploadImageFromBytes(ctx context.Context, req UploadImageRequest) (*PlatformImage, error)

	// Product draft
	CreateProductDraft(ctx context.Context, shopID string, req CreateProductDraftRequest) (*PlatformProductResult, error)
	GetProductDetail(ctx context.Context, shopID, platformProductID string) (*PlatformProductDetail, error)

	// Order
	SyncOrdersPage(ctx context.Context, cursor string, limit int, start, end *time.Time) ([]platformp.PlatformOrder, string, bool, map[string]any, error)
	GetOrderDetail(ctx context.Context, shopOrderID string) (*OrderDetail, error)

	// Inventory
	SyncStock(ctx context.Context, req SkuSyncStockRequest) error
	GetSKUStock(ctx context.Context, shopID, platformProductID, platformSKUID string) (int, error)

	// Customer messaging — gated by contract verification
	CustomerCapability() CustomerCapability

	// Brand — explicitly unsupported in P3
	BrandStatus() BrandSupportStatus
}

// SkuSyncStockRequest is the normalized sku.syncStock input.
type SkuSyncStockRequest struct {
	ShopID            string
	PlatformProductID string
	PlatformSKUID     string
	Stock             int
}

// BrandSupportStatus describes brand list capability.
type BrandSupportStatus struct {
	Supported bool
	Reason    string // blocked_by_contract_verification or similar
}

// clientFacade wraps *Client as DouyinProvider.
type clientFacade struct {
	c *Client
}

// NewFacade wraps a *Client as DouyinProvider. Returns nil if c is nil.
func NewFacade(c *Client) DouyinProvider {
	if c == nil {
		return nil
	}
	return &clientFacade{c: c}
}

func (f *clientFacade) EnsureFreshToken(ctx context.Context) (string, error) {
	return f.c.EnsureFreshAccess(ctx)
}

func (f *clientFacade) RefreshToken(ctx context.Context) (*TokenBundle, error) {
	return f.c.RefreshAccessToken(ctx)
}

func (f *clientFacade) GetShopInfo(ctx context.Context, shopID string) (*ShopInfo, error) {
	return f.c.GetShopInfo(ctx, shopID)
}

func (f *clientFacade) GetCategoryList(ctx context.Context, parentID string) ([]Category, error) {
	return f.c.GetCategories(ctx, CategoryRequest{ParentID: parentID})
}

func (f *clientFacade) GetCategoryRequiredAttributes(ctx context.Context, categoryID string) ([]CategoryAttribute, error) {
	return f.c.GetCategoryAttributes(ctx, categoryID)
}

func (f *clientFacade) UploadImageFromURL(ctx context.Context, imageURL, shopID string) (*PlatformImage, error) {
	return f.c.UploadImage(ctx, shopID, UploadImageRequest{SourceURL: imageURL})
}

func (f *clientFacade) UploadImageFromBytes(ctx context.Context, req UploadImageRequest) (*PlatformImage, error) {
	return f.c.UploadImage(ctx, f.c.ShopID, req)
}

func (f *clientFacade) CreateProductDraft(ctx context.Context, shopID string, req CreateProductDraftRequest) (*PlatformProductResult, error) {
	return f.c.CreateProductDraft(ctx, shopID, req)
}

func (f *clientFacade) GetProductDetail(ctx context.Context, shopID, platformProductID string) (*PlatformProductDetail, error) {
	return f.c.GetProductDetail(ctx, shopID, platformProductID)
}

func (f *clientFacade) SyncOrdersPage(ctx context.Context, cursor string, limit int, start, end *time.Time) ([]platformp.PlatformOrder, string, bool, map[string]any, error) {
	return SyncOrdersPage(ctx, f.c, cursor, limit, start, end)
}

func (f *clientFacade) GetOrderDetail(ctx context.Context, shopOrderID string) (*OrderDetail, error) {
	return f.c.GetOrderDetail(ctx, shopOrderID)
}

func (f *clientFacade) SyncStock(ctx context.Context, req SkuSyncStockRequest) error {
	params := map[string]any{
		"product_id":  req.PlatformProductID,
		"sku_id":      req.PlatformSKUID,
		"stock_num":   req.Stock,
		"incremental": false,
	}
	var out map[string]any
	return f.c.Do(ctx, MethodSkuSyncStock, params, &out)
}

func (f *clientFacade) GetSKUStock(ctx context.Context, shopID, platformProductID, platformSKUID string) (int, error) {
	return GetSKUStockFromDetail(ctx, f.c, shopID, platformProductID, platformSKUID)
}

func (f *clientFacade) CustomerCapability() CustomerCapability {
	return newCustomerCapabilityFromClient(f.c)
}

func (f *clientFacade) BrandStatus() BrandSupportStatus {
	return BrandSupportStatus{
		Supported: false,
		Reason:    "blocked_by_contract_verification: Douyin brand list API requires additional contract verification; standard_brand_id from category mapping is used instead",
	}
}
