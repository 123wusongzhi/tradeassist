package collect

import "fmt"

// providerProfileKey is generated solely from authenticated request or task tenant
// context. tenant 0 deliberately keeps its historical single-tenant directory.
func providerProfileKey(tenantID int64, provider string) string {
	if tenantID == 0 {
		return provider
	}
	return fmt.Sprintf("tenant_%d_%s", tenantID, provider)
}
