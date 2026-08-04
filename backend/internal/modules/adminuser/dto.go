package adminuser

// UserRow is a list/detail DTO.
type UserRow struct {
	ID               string      `json:"id"`
	TenantID         int64       `json:"tenantId"`
	Username         string      `json:"username"`
	Email            string      `json:"email,omitempty"`
	Phone            string      `json:"phone,omitempty"`
	DisplayName      string      `json:"displayName"`
	Role             string      `json:"role"`
	Status           string      `json:"status"`
	StorePermissions []StorePerm `json:"storePermissions,omitempty"`
	LastLoginAt      *string     `json:"lastLoginAt,omitempty"`
	LastOperationAt  *string     `json:"lastOperationAt,omitempty"`
	CreatedAt        string      `json:"createdAt"`
	UpdatedAt        string      `json:"updatedAt"`
}

// TenantOption is one assignable tenant shown in the global user manager.
// ShopNames keeps legacy tenants discoverable when they predate the tenants table.
type TenantOption struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name,omitempty"`
	ShopNames []string `json:"shopNames,omitempty"`
}

// StorePerm is a store authorization row.
type StorePerm struct {
	ID              string `json:"id"`
	StoreID         string `json:"storeId"`
	StoreName       string `json:"storeName,omitempty"`
	Platform        string `json:"platform,omitempty"`
	PermissionScope string `json:"permissionScope"`
}
