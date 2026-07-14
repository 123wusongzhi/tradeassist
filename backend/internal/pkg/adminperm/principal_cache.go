package adminperm

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const principalCacheTTL = 2 * time.Minute

var globalPrincipalCache = &principalCache{
	entries: map[string]principalCacheEntry{},
	now:     time.Now,
	ttl:     principalCacheTTL,
}

type principalCache struct {
	mu      sync.Mutex
	entries map[string]principalCacheEntry
	now     func() time.Time
	ttl     time.Duration
}

type principalCacheEntry struct {
	p         *Principal
	expiresAt time.Time
}

func permissionCacheKey(tenantID int64, userID uuid.UUID, tokenVersion int, status string, role string) string {
	return fmt.Sprintf("tenant:%d|user:%s|security:%d|status:%s|role:%s", tenantID, userID.String(), tokenVersion, strings.TrimSpace(strings.ToLower(status)), normalizeRole(role))
}

func getCachedPrincipal(key string) (*Principal, bool) {
	return globalPrincipalCache.get(key)
}

func putCachedPrincipal(key string, p *Principal) {
	globalPrincipalCache.put(key, p)
}

// InvalidateUserPermissionCache removes local permission cache entries for a user.
// Call this after role, store grant, security version, or user status changes commit.
func InvalidateUserPermissionCache(userID uuid.UUID) {
	globalPrincipalCache.invalidateUser(userID)
}

func (c *principalCache) get(key string) (*Principal, bool) {
	if c == nil || key == "" {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || c.now().After(e.expiresAt) {
		delete(c.entries, key)
		return nil, false
	}
	return clonePrincipal(e.p), true
}

func (c *principalCache) put(key string, p *Principal) {
	if c == nil || key == "" || p == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = principalCacheEntry{p: clonePrincipal(p), expiresAt: c.now().Add(c.ttl)}
}

func (c *principalCache) invalidateUser(userID uuid.UUID) {
	if c == nil || userID == uuid.Nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	needle := "|user:" + userID.String() + "|"
	for k := range c.entries {
		if strings.Contains(k, needle) {
			delete(c.entries, k)
		}
	}
}

func clonePrincipal(p *Principal) *Principal {
	if p == nil {
		return nil
	}
	cp := *p
	cp.Permissions = append([]string(nil), p.Permissions...)
	cp.StoreGrants = append([]StoreGrant(nil), p.StoreGrants...)
	return &cp
}
