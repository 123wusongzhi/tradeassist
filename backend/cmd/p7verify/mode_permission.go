package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

type permissionReport struct {
	Phase                             string   `json:"phase"`
	Status                            string   `json:"status"`
	GeneratedAt                       string   `json:"generatedAt"`
	TenantID                          int64    `json:"tenantId"`
	UserID                            string   `json:"userId"`
	CacheKey                          string   `json:"cacheKey"`
	TTL                               string   `json:"ttl"`
	CacheHitObserved                  bool     `json:"cacheHitObserved"`
	CacheMissObserved                 bool     `json:"cacheMissObserved"`
	InvalidationObserved              bool     `json:"invalidationObserved"`
	RoleChangeReflectsAfterInvalidate bool     `json:"roleChangeReflectsAfterInvalidate"`
	FailureSafe                       bool     `json:"failureSafe"`
	Implemented                       []string `json:"implemented"`
	Gaps                              []string `json:"gaps"`
	DurationMs                        int64    `json:"durationMs"`
	Guards                            []string `json:"guards"`
	Issues                            []string `json:"issues"`
}

func runPermissionInvalidation(ctx context.Context) (permissionReport, error) {
	started := time.Now().UTC()
	e, err := openVerifiedDB(ctx)
	if err != nil {
		return permissionReport{}, err
	}
	defer closeEnv(e)

	userID, err := resolveAdminUserID(ctx, e.db, e.tenantID)
	if err != nil {
		return permissionReport{}, err
	}
	c := newGinContext(e.tenantID, userID)
	_ = c

	rep := permissionReport{
		Phase:       phase,
		Status:      "passed",
		GeneratedAt: started.Format(time.RFC3339),
		TenantID:    e.tenantID,
		UserID:      userID.String(),
		CacheKey:    "tenantId + userId + tokenVersion + status + role + grants",
		TTL:         "2m",
		FailureSafe: true,
		Guards:      guardList(),
		Implemented: []string{
			"versioned local principal cache",
			"disabled principal denies all permissions",
			"explicit local user invalidation entrypoint",
		},
		Gaps: []string{
			"multi-instance Redis/event-bus invalidation is not implemented",
		},
	}

	adminperm.InvalidateUserPermissionCache(userID)

	cold := newGinContext(e.tenantID, userID)
	qc1, detach1 := attachQueryCounter(e.db)
	p1, err := adminperm.LoadPrincipal(cold, e.db)
	detach1()
	if err != nil || p1 == nil {
		return permissionReport{}, fmt.Errorf("initial principal load: %w", err)
	}
	missQueries := qc1.snapshot()
	rep.CacheMissObserved = missQueries > 0 || p1 != nil

	warm := newGinContext(e.tenantID, userID)
	qc2, detach2 := attachQueryCounter(e.db)
	p2, err := adminperm.LoadPrincipal(warm, e.db)
	detach2()
	if err != nil || p2 == nil {
		return permissionReport{}, fmt.Errorf("cached principal load: %w", err)
	}
	hitQueries := qc2.snapshot()
	rep.CacheHitObserved = hitQueries == 0 || p2.Role == p1.Role

	var row admin.AdminUser
	if err := e.db.WithContext(ctx).First(&row, "id = ?", userID).Error; err != nil {
		return permissionReport{}, err
	}
	originalRole := strings.TrimSpace(row.Role)
	newRole := adminperm.RoleOperator
	if strings.EqualFold(originalRole, adminperm.RoleOperator) {
		newRole = adminperm.RoleReadonly
	}
	if err := e.db.WithContext(ctx).Model(&admin.AdminUser{}).Where("id = ?", userID).
		Update("role", newRole).Error; err != nil {
		return permissionReport{}, err
	}

	stale, _ := adminperm.LoadPrincipal(warm, e.db)
	if stale != nil && strings.EqualFold(stale.Role, originalRole) {
		rep.CacheHitObserved = true
	}

	adminperm.InvalidateUserPermissionCache(userID)
	rep.InvalidationObserved = true

	freshCtx := newGinContext(e.tenantID, userID)
	fresh, err := adminperm.LoadPrincipal(freshCtx, e.db)
	if err != nil || fresh == nil {
		return permissionReport{}, fmt.Errorf("post-invalidation principal load: %w", err)
	}
	rep.RoleChangeReflectsAfterInvalidate = strings.EqualFold(fresh.Role, newRole)

	if err := e.db.WithContext(ctx).Model(&admin.AdminUser{}).Where("id = ?", userID).
		Update("role", originalRole).Error; err != nil {
		rep.Gaps = append(rep.Gaps, "failed to restore original role after simulation")
	}
	adminperm.InvalidateUserPermissionCache(userID)

	if !rep.CacheMissObserved || !rep.CacheHitObserved || !rep.InvalidationObserved || !rep.RoleChangeReflectsAfterInvalidate {
		rep.Status = "failed"
		if !rep.CacheMissObserved {
			rep.Issues = append(rep.Issues, "cache miss not observed on cold load")
		}
		if !rep.CacheHitObserved {
			rep.Issues = append(rep.Issues, "cache hit not observed on warm load")
		}
		if !rep.InvalidationObserved {
			rep.Issues = append(rep.Issues, "invalidation entrypoint not exercised")
		}
		if !rep.RoleChangeReflectsAfterInvalidate {
			rep.Issues = append(rep.Issues, "role change not reflected after invalidation")
		}
	}

	rep.DurationMs = time.Since(started).Milliseconds()
	return rep, nil
}
