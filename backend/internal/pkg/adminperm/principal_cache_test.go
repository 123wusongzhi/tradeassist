package adminperm

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPrincipalCacheVersionedKeyAndInvalidation(t *testing.T) {
	uid := uuid.New()
	keyV1 := permissionCacheKey(7, uid, 1, "active", RoleOperator, "")
	keyV2 := permissionCacheKey(7, uid, 2, "active", RoleReadonly, "")
	p := &Principal{UserID: uid, Role: RoleOperator, Permissions: PermissionsForRole(RoleOperator)}
	putCachedPrincipal(keyV1, p)
	if got, ok := getCachedPrincipal(keyV1); !ok || !got.Can(PermOrderOperate) {
		t.Fatalf("expected v1 cache hit, got %+v ok=%v", got, ok)
	}
	if got, ok := getCachedPrincipal(keyV2); ok || got != nil {
		t.Fatalf("versioned key should miss, got %+v ok=%v", got, ok)
	}
	InvalidateUserPermissionCache(uid)
	if got, ok := getCachedPrincipal(keyV1); ok || got != nil {
		t.Fatalf("invalidation should remove user cache, got %+v ok=%v", got, ok)
	}
}

func TestPrincipalCacheTTLAndDisabledPrincipal(t *testing.T) {
	old := globalPrincipalCache
	defer func() { globalPrincipalCache = old }()
	now := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)
	globalPrincipalCache = &principalCache{
		entries: map[string]principalCacheEntry{},
		now:     func() time.Time { return now },
		ttl:     time.Second,
	}
	uid := uuid.New()
	key := permissionCacheKey(7, uid, 1, "disabled", RoleAdmin, "")
	putCachedPrincipal(key, &Principal{UserID: uid, Role: RoleAdmin, Disabled: true})
	if got, ok := getCachedPrincipal(key); !ok || !got.Disabled || got.Can(PermOrderView) || got.IsAdmin() {
		t.Fatalf("disabled cached principal should deny all, got %+v ok=%v", got, ok)
	}
	now = now.Add(2 * time.Second)
	if got, ok := getCachedPrincipal(key); ok || got != nil {
		t.Fatalf("expired cache should miss, got %+v ok=%v", got, ok)
	}
}
