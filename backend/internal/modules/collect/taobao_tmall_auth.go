package collect

import (
	"context"
	"strings"
)

// LatestFailedTaobaoTmallSourceURL returns the most recent failed collect task URL for taobao_tmall.
func (s *Service) LatestFailedTaobaoTmallSourceURL(ctx context.Context, tenantID int64) string {
	if s == nil || s.DB == nil || tenantID < 0 {
		return ""
	}
	var task CollectTask
	err := s.DB.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Where("LOWER(source) IN ?", []string{"taobao_tmall", "taobao"}).
		Where("status = ?", StatusFailed).
		Where("source_url <> ''").
		Order("updated_at DESC").
		Limit(1).
		Find(&task).Error
	if err != nil || strings.TrimSpace(task.SourceURL) == "" {
		return ""
	}
	return strings.TrimSpace(task.SourceURL)
}

// ResolveTaobaoTmallAuthCheckInputs picks context URL (body → latest failure) and settings test URL.
func (s *Service) ResolveTaobaoTmallAuthCheckInputs(ctx context.Context, tenantID int64, bodyURL string) (contextURL string, settingsTestURL string) {
	if tenantID < 0 {
		return "", ""
	}
	contextURL = strings.TrimSpace(bodyURL)
	if contextURL == "" {
		contextURL = s.LatestFailedTaobaoTmallSourceURL(ctx, tenantID)
	}
	if s != nil && s.Settings != nil {
		m, err := s.Settings.PlainByGroup(ctx, 0, "collector")
		if err == nil {
			settingsTestURL = strings.TrimSpace(m["collect_taobao_tmall_auth_check_url"])
		}
	}
	return contextURL, settingsTestURL
}
