package shop

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

// Shop is a unified storefront record (channels are not duplicated per table).
type Shop struct {
	model.Base
	TenantID        int64          `gorm:"default:0;index" json:"tenantId"`
	Platform        string         `gorm:"size:32;index;not null" json:"platform"`
	ShopName        string         `gorm:"size:255;not null" json:"shopName"`
	ShopCode        string         `gorm:"size:128;index" json:"shopCode,omitempty"`
	ExternalShopID  string         `gorm:"size:255;index" json:"externalShopId,omitempty"`
	Status          string         `gorm:"size:32;index;not null" json:"status"`
	AuthStatus      string         `gorm:"size:32;index;not null" json:"authStatus"`
	Region          string         `gorm:"size:64" json:"region,omitempty"`
	Currency        string         `gorm:"size:16" json:"currency,omitempty"`
	Timezone        string         `gorm:"size:128" json:"timezone,omitempty"`
	DefaultLanguage string         `gorm:"size:32" json:"defaultLanguage,omitempty"`
	Capabilities    datatypes.JSON `gorm:"type:jsonb" json:"capabilities,omitempty"`
	PlatformConfig  datatypes.JSON `gorm:"type:jsonb" json:"platformConfig,omitempty"`
	Remark          string         `gorm:"type:text" json:"remark,omitempty"`
	CreatedBy       *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

func (Shop) TableName() string { return "shops" }

// ShopAuthToken stores secrets for one shop (single row per shop in v1).
type ShopAuthToken struct {
	model.Base
	ShopID                  uuid.UUID      `gorm:"type:char(36);uniqueIndex;not null" json:"shopId"`
	Platform                string         `gorm:"size:32;index;not null" json:"platform"`
	AuthType                string         `gorm:"size:32;index;not null" json:"authType"`
	AppKey                  string         `gorm:"size:512" json:"appKey,omitempty"`
	AppSecretEnc            string         `gorm:"type:text" json:"-"`
	AccessTokenEnc          string         `gorm:"type:text" json:"-"`
	RefreshTokenEnc         string         `gorm:"type:text" json:"-"`
	SellerID                string         `gorm:"size:255" json:"sellerId,omitempty"`
	MerchantID              string         `gorm:"size:255" json:"merchantId,omitempty"`
	MarketplaceID           string         `gorm:"size:255" json:"marketplaceId,omitempty"`
	ExpiresAt               *time.Time     `json:"expiresAt,omitempty"`
	RefreshExpiresAt        *time.Time     `json:"refreshExpiresAt,omitempty"`
	Scopes                  datatypes.JSON `gorm:"type:jsonb" json:"scopes,omitempty"`
	AuthConfig              datatypes.JSON `gorm:"type:jsonb" json:"authConfig,omitempty"`
	RawData                 datatypes.JSON `gorm:"type:jsonb" json:"rawData,omitempty"`
	TokenVersion            int64          `gorm:"not null;default:0" json:"tokenVersion"`
	ReauthorizationRequired bool           `gorm:"not null;default:false" json:"reauthorizationRequired"`
	LastRefreshErrorCode    string         `gorm:"size:128" json:"lastRefreshErrorCode,omitempty"`
}

func (ShopAuthToken) TableName() string { return "shop_auth_tokens" }

// PlatformCategory caches marketplace categories for listing preparation.
type PlatformCategory struct {
	model.Base
	Platform   string         `gorm:"size:32;uniqueIndex:idx_platform_category;index;not null" json:"platform"`
	CategoryID string         `gorm:"size:128;uniqueIndex:idx_platform_category;not null" json:"categoryId"`
	ParentID   string         `gorm:"size:128;index" json:"parentId"`
	Name       string         `gorm:"size:512;index" json:"name"`
	Level      int            `gorm:"index" json:"level"`
	IsLeaf     bool           `gorm:"index;not null;default:false" json:"isLeaf"`
	Status     string         `gorm:"size:64;index" json:"status,omitempty"`
	Raw        datatypes.JSON `gorm:"type:jsonb" json:"raw,omitempty"`
	SyncedAt   *time.Time     `gorm:"index" json:"syncedAt,omitempty"`
}

func (PlatformCategory) TableName() string { return "platform_categories" }

// PlatformCategoryAttribute caches marketplace-required category attributes.
type PlatformCategoryAttribute struct {
	model.Base
	Platform    string         `gorm:"size:32;uniqueIndex:idx_platform_category_attr;index;not null" json:"platform"`
	CategoryID  string         `gorm:"size:128;uniqueIndex:idx_platform_category_attr;index;not null" json:"categoryId"`
	AttrID      string         `gorm:"size:128;uniqueIndex:idx_platform_category_attr;not null" json:"attrId"`
	Name        string         `gorm:"size:512;index" json:"name"`
	Required    bool           `gorm:"index;not null;default:false" json:"required"`
	ValueType   string         `gorm:"size:128" json:"valueType,omitempty"`
	Options     datatypes.JSON `gorm:"type:jsonb" json:"options,omitempty"`
	UnitOptions datatypes.JSON `gorm:"type:jsonb" json:"unitOptions,omitempty"`
	Raw         datatypes.JSON `gorm:"type:jsonb" json:"raw,omitempty"`
	SyncedAt    *time.Time     `gorm:"index" json:"syncedAt,omitempty"`
}

func (PlatformCategoryAttribute) TableName() string { return "platform_category_attributes" }

// PlatformCategoryAttributeMapping binds one Ozon (or other platform) category
// attribute to a local product field for automatic listing fill.
// The template itself lives in platform_category_attributes (24h+ cache);
// this table is the per-category "Ozon attribute name <-> local field" mapping.
type PlatformCategoryAttributeMapping struct {
	model.Base
	Platform      string `gorm:"size:32;uniqueIndex:idx_platform_cat_attr_mapping;not null" json:"platform"`
	CategoryID    string `gorm:"size:128;uniqueIndex:idx_platform_cat_attr_mapping;index;not null" json:"categoryId"`
	AttributeID   string `gorm:"size:128;uniqueIndex:idx_platform_cat_attr_mapping;not null" json:"attributeId"`
	AttributeName string `gorm:"size:512" json:"attributeName,omitempty"`
	LocalField    string `gorm:"size:128" json:"localField,omitempty"`
	Enabled       bool   `gorm:"not null;default:true" json:"enabled"`
	SortOrder     int    `gorm:"not null;default:0" json:"sortOrder"`
}

func (PlatformCategoryAttributeMapping) TableName() string {
	return "platform_category_attribute_mappings"
}

// OzonCategorySyncRun records an auditable, tenant-scoped refresh of the shared
// Ozon catalogue. The catalogue itself remains global; the initiating shop and
// resulting diff are never shared between tenants.
type OzonCategorySyncRun struct {
	model.Base
	TenantID     int64          `gorm:"not null;index" json:"tenantId"`
	ShopID       uuid.UUID      `gorm:"type:char(36);index;not null" json:"shopId"`
	Status       string         `gorm:"size:32;index;not null" json:"status"`
	StartedAt    *time.Time     `json:"startedAt,omitempty"`
	FinishedAt   *time.Time     `json:"finishedAt,omitempty"`
	Summary      datatypes.JSON `gorm:"type:jsonb" json:"summary,omitempty"`
	ErrorCode    string         `gorm:"size:96;index" json:"errorCode,omitempty"`
	ErrorMessage string         `gorm:"type:text" json:"errorMessage,omitempty"`
}

func (OzonCategorySyncRun) TableName() string { return "ozon_category_sync_runs" }

// OzonCategoryChange stores a bounded before/after representation for one
// catalogue change observed by a sync run.
type OzonCategoryChange struct {
	model.Base
	TenantID   int64          `gorm:"not null;index" json:"tenantId"`
	ShopID     uuid.UUID      `gorm:"type:char(36);index;not null" json:"shopId"`
	SyncRunID  uuid.UUID      `gorm:"type:char(36);index;not null" json:"syncRunId"`
	CategoryID string         `gorm:"size:128;index;not null" json:"categoryId"`
	ChangeType string         `gorm:"size:32;index;not null" json:"changeType"`
	Before     datatypes.JSON `gorm:"type:jsonb" json:"before,omitempty"`
	After      datatypes.JSON `gorm:"type:jsonb" json:"after,omitempty"`
}

func (OzonCategoryChange) TableName() string { return "ozon_category_changes" }

// OzonCategoryMapping is a tenant-owned mapping from a stable source category
// key to one active Ozon leaf category. A nil ShopID denotes a tenant default.
type OzonCategoryMapping struct {
	model.Base
	TenantID           int64      `gorm:"not null;index" json:"tenantId"`
	ShopID             *uuid.UUID `gorm:"type:char(36);index" json:"shopId,omitempty"`
	ScopeKey           string     `gorm:"size:64;not null;default:tenant" json:"-"`
	SourceCategoryKey  string     `gorm:"size:256;not null" json:"sourceCategoryKey"`
	SourceCategoryName string     `gorm:"size:512" json:"sourceCategoryName,omitempty"`
	CategoryID         string     `gorm:"size:128;index;not null" json:"categoryId"`
	CategoryPath       string     `gorm:"size:1024" json:"categoryPath,omitempty"`
	Status             string     `gorm:"size:32;index;not null" json:"status"`
	SchemaHash         string     `gorm:"size:128" json:"schemaHash,omitempty"`
	SelectionMethod    string     `gorm:"size:64" json:"selectionMethod,omitempty"`
	ConfirmationReason string     `gorm:"size:1000" json:"confirmationReason,omitempty"`
	TemplateSyncedAt   *time.Time `json:"templateSyncedAt,omitempty"`
	ConfirmedAt        *time.Time `json:"confirmedAt,omitempty"`
	ConfirmedBy        *uuid.UUID `gorm:"type:char(36);index" json:"confirmedBy,omitempty"`
}

func (OzonCategoryMapping) TableName() string { return "ozon_category_mappings" }
