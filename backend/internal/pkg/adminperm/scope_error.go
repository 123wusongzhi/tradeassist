package adminperm

import "net/http"

// ScopeOperationError represents a resource the caller may view but may not
// mutate. Resources outside view scope continue to use gorm.ErrRecordNotFound.
type ScopeOperationError struct {
	Code    string
	Message string
}

func (e *ScopeOperationError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func (e *ScopeOperationError) HTTPStatus() int { return http.StatusForbidden }

func (e *ScopeOperationError) SafeMessage() string {
	if e == nil || e.Message == "" {
		return "当前账号无权限执行此操作"
	}
	return e.Message
}

func (e *ScopeOperationError) SafeData() any {
	if e == nil || e.Code == "" {
		return nil
	}
	return map[string]any{"errorCode": e.Code}
}

func crossTenantOperationError() error {
	return &ScopeOperationError{
		Code:    "CROSS_TENANT_OPERATION_FORBIDDEN",
		Message: "全局管理员仅可跨租户查看，不能代表目标租户执行写操作；请使用目标租户管理员账号",
	}
}

func productOperationError() error {
	return &ScopeOperationError{
		Code:    "PRODUCT_OPERATION_FORBIDDEN",
		Message: "当前账号仅有查看权限，不能修改该商品关联店铺的数据",
	}
}

func storeOperationError() error {
	return &ScopeOperationError{
		Code:    "STORE_OPERATION_FORBIDDEN",
		Message: "当前账号仅有该店铺的查看权限，不能执行写操作",
	}
}
