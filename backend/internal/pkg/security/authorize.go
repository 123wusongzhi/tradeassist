package security

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// Authorization errors.
var (
	ErrAuthenticationRequired   = errors.New("AUTHENTICATION_REQUIRED")
	ErrPermissionDenied         = errors.New("PERMISSION_DENIED")
	ErrTenantAccessDenied       = errors.New("TENANT_ACCESS_DENIED")
	ErrShopAccessDenied         = errors.New("SHOP_ACCESS_DENIED")
	ErrSensitiveOperationDenied = errors.New("SENSITIVE_OPERATION_DENIED")
)

// AuthorizationService enforces RBAC, tenant and shop scope.
type AuthorizationService interface {
	RequirePermission(ctx context.Context, permission string) error
	RequireTenant(ctx context.Context, tenantID int64) error
	RequireShopAccess(ctx context.Context, shopID uuid.UUID) error
	RequireSensitivePermission(ctx context.Context, permission string) error
}

// GinAuthorizer implements AuthorizationService using gin + adminperm.
type GinAuthorizer struct {
	C    *gin.Context
	DB   *gorm.DB
	perm *adminperm.Principal
}

// NewGinAuthorizer builds an authorizer for the current request.
func NewGinAuthorizer(c *gin.Context, db *gorm.DB) (*GinAuthorizer, error) {
	if c == nil {
		return nil, ErrAuthenticationRequired
	}
	p, err := adminperm.LoadPrincipal(c, db)
	if err != nil {
		return nil, err
	}
	if p == nil || p.UserID == uuid.Nil {
		return nil, ErrAuthenticationRequired
	}
	return &GinAuthorizer{C: c, DB: db, perm: p}, nil
}

func (a *GinAuthorizer) RequirePermission(ctx context.Context, permission string) error {
	if a == nil || a.perm == nil {
		return ErrAuthenticationRequired
	}
	if a.perm.Can(permission) {
		return nil
	}
	return ErrPermissionDenied
}

func (a *GinAuthorizer) RequireTenant(ctx context.Context, tenantID int64) error {
	tc := FromContext(ctx)
	if tc == nil {
		tc = FromGin(a.C)
	}
	if tc == nil {
		return ErrAuthenticationRequired
	}
	if tc.TenantID != tenantID {
		return ErrTenantAccessDenied
	}
	return nil
}

func (a *GinAuthorizer) RequireShopAccess(ctx context.Context, shopID uuid.UUID) error {
	if a == nil || a.perm == nil {
		return ErrAuthenticationRequired
	}
	if shopID == uuid.Nil {
		return nil
	}
	if a.perm.IsTenantAdmin() {
		var count int64
		if a.DB != nil && a.DB.WithContext(ctx).Table("shops").Where("id = ? AND tenant_id = ?", shopID, a.perm.TenantID).Count(&count).Error == nil && count == 1 {
			return nil
		}
		return ErrShopAccessDenied
	}
	if a.perm.CanViewStore(shopID) {
		return nil
	}
	return ErrShopAccessDenied
}

func (a *GinAuthorizer) RequireSensitivePermission(ctx context.Context, permission string) error {
	if err := a.RequirePermission(ctx, permission); err != nil {
		return err
	}
	// Reauth token validation is performed at handler level for high-risk ops.
	return nil
}

// Deny writes standardized denial response.
func Deny(c *gin.Context, err error) {
	if c == nil || err == nil {
		return
	}
	switch {
	case errors.Is(err, ErrAuthenticationRequired):
		response.Fail(c, 401, response.CodeUnauthorized, err.Error())
	case errors.Is(err, ErrPermissionDenied), errors.Is(err, ErrSensitiveOperationDenied):
		response.Fail(c, 403, response.CodePermissionDenied, "无权限执行此操作")
	case errors.Is(err, ErrTenantAccessDenied):
		response.Fail(c, 403, response.CodeForbidden, "该资源不属于当前租户")
	case errors.Is(err, ErrShopAccessDenied):
		response.Fail(c, 403, response.CodeStorePermissionDenied, "当前账号无权访问此店铺")
	default:
		response.Fail(c, 403, response.CodeForbidden, err.Error())
	}
}

// RequirePermissionGin is a helper for handlers.
func RequirePermissionGin(c *gin.Context, db *gorm.DB, permission string) bool {
	authz, err := NewGinAuthorizer(c, db)
	if err != nil {
		Deny(c, err)
		return false
	}
	if err := authz.RequirePermission(c.Request.Context(), permission); err != nil {
		Deny(c, err)
		return false
	}
	return true
}

// TenantScopedQuery adds tenant_id filter when tenant > 0.
func TenantScopedQuery(tx *gorm.DB, tenantID int64) *gorm.DB {
	if tx == nil {
		return tx
	}
	return tx.Where("tenant_id = ?", tenantID)
}

// EnsureTenantMatch validates resource tenant matches context.
func EnsureTenantMatch(ctx context.Context, resourceTenantID int64) error {
	tc := FromContext(ctx)
	if tc == nil {
		return ErrTenantContextMissing
	}
	if tc.TenantID <= 0 {
		return ErrTenantContextMissing
	}
	if tc.TenantID != resourceTenantID {
		return ErrTenantAccessDenied
	}
	return nil
}
