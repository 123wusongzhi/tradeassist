package alerting

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Severity levels.
const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityCritical = "critical"
)

// Status values.
const (
	StatusFiring       = "firing"
	StatusAcknowledged = "acknowledged"
	StatusSilenced     = "silenced"
	StatusResolved     = "resolved"
	StatusExpired      = "expired"
)

// AlertEvent is the unified alert event model.
type AlertEvent struct {
	ID              string `gorm:"primaryKey;size:36"`
	Fingerprint     string `gorm:"size:64;index"`
	RuleID          string `gorm:"size:64;index"`
	Severity        string `gorm:"size:16;index"`
	Status          string `gorm:"size:16;index"`
	Source          string `gorm:"size:32"`
	Module          string `gorm:"size:64"`
	Summary         string `gorm:"size:512"`
	SafeDetails     string `gorm:"type:text"`
	FirstSeenAt     time.Time
	LastSeenAt      time.Time
	OccurrenceCount int `gorm:"default:1"`
	ResolvedAt      *time.Time
	CooldownUntil   *time.Time
	TenantScope     string `gorm:"size:64"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (AlertEvent) TableName() string { return "alert_events" }

// AlertRule defines an alert rule.
type AlertRule struct {
	ID                string `gorm:"primaryKey;size:64"`
	Name              string `gorm:"size:128"`
	Metric            string `gorm:"size:128"`
	Condition         string `gorm:"size:32"`
	Window            string `gorm:"size:32"`
	Threshold         float64
	Severity          string `gorm:"size:16"`
	CooldownSeconds   int    `gorm:"default:300"`
	RecoveryCondition string `gorm:"size:128"`
	Enabled           bool   `gorm:"default:true"`
	RunbookURL        string `gorm:"size:512"`
	ChannelGroup      string `gorm:"size:64"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (AlertRule) TableName() string { return "alert_rules" }

// AlertSilence suppresses alerts temporarily.
type AlertSilence struct {
	ID         string `gorm:"primaryKey;size:36"`
	AlertID    string `gorm:"size:36;index"`
	RuleID     string `gorm:"size:64;index"`
	Reason     string `gorm:"size:256"`
	SilencedBy string `gorm:"size:36"`
	ExpiresAt  time.Time
	CreatedAt  time.Time
}

func (AlertSilence) TableName() string { return "alert_silences" }

// AlertNotification payload for channels.
type AlertNotification struct {
	AlertID     string
	RuleID      string
	Severity    string
	Status      string
	Summary     string
	SafeDetails string
	RunbookURL  string
}

// Channel sends alert notifications.
type Channel interface {
	Name() string
	Send(ctx context.Context, n AlertNotification) error
}

// InternalChannel stores delivery in DB only.
type InternalChannel struct{}

func (InternalChannel) Name() string { return "internal" }

func (InternalChannel) Send(ctx context.Context, n AlertNotification) error {
	_ = ctx
	_ = n
	return nil
}

// Service manages alert lifecycle.
type Service struct {
	DB              *gorm.DB
	DefaultCooldown time.Duration
	RecoveryEnabled bool
	Channels        []Channel
	mu              sync.Mutex
	cache           map[string]*AlertEvent
}

// NewService creates alerting service.
func NewService(db *gorm.DB, cooldown time.Duration, recovery bool) *Service {
	return &Service{
		DB:              db,
		DefaultCooldown: cooldown,
		RecoveryEnabled: recovery,
		Channels:        []Channel{InternalChannel{}},
		cache:           make(map[string]*AlertEvent),
	}
}

// Fingerprint builds dedup fingerprint.
func Fingerprint(ruleID, module, summary string) string {
	raw := strings.Join([]string{ruleID, module, summary}, "|")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// Fire creates or aggregates an alert event.
func (s *Service) Fire(ctx context.Context, ruleID, severity, module, summary, details string) (*AlertEvent, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("alerting unavailable")
	}
	fp := Fingerprint(ruleID, module, summary)
	now := time.Now().UTC()
	if s.isSilenced(ctx, ruleID, now) {
		return &AlertEvent{
			ID:              uuid.New().String(),
			Fingerprint:     fp,
			RuleID:          ruleID,
			Severity:        severity,
			Status:          StatusSilenced,
			Source:          "app",
			Module:          module,
			Summary:         summary,
			SafeDetails:     sanitizeDetails(details),
			FirstSeenAt:     now,
			LastSeenAt:      now,
			OccurrenceCount: 1,
		}, nil
	}
	s.mu.Lock()
	if cached, ok := s.cache[fp]; ok && cached.CooldownUntil != nil && now.Before(*cached.CooldownUntil) {
		cached.OccurrenceCount++
		cached.LastSeenAt = now
		s.mu.Unlock()
		_ = s.DB.WithContext(ctx).Save(cached).Error
		return cached, nil
	}
	s.mu.Unlock()

	var existing AlertEvent
	err := s.DB.WithContext(ctx).Where("fingerprint = ? AND status IN ?", fp, []string{StatusFiring, StatusAcknowledged}).First(&existing).Error
	if err == nil {
		existing.OccurrenceCount++
		existing.LastSeenAt = now
		cd := now.Add(s.cooldownDuration())
		existing.CooldownUntil = &cd
		if err := s.DB.WithContext(ctx).Save(&existing).Error; err != nil {
			return nil, err
		}
		s.mu.Lock()
		s.cache[fp] = &existing
		s.mu.Unlock()
		return &existing, nil
	}

	cd := now.Add(s.cooldownDuration())
	ev := AlertEvent{
		ID:              uuid.New().String(),
		Fingerprint:     fp,
		RuleID:          ruleID,
		Severity:        severity,
		Status:          StatusFiring,
		Source:          "app",
		Module:          module,
		Summary:         summary,
		SafeDetails:     sanitizeDetails(details),
		FirstSeenAt:     now,
		LastSeenAt:      now,
		OccurrenceCount: 1,
		CooldownUntil:   &cd,
	}
	if err := s.DB.WithContext(ctx).Create(&ev).Error; err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.cache[fp] = &ev
	s.mu.Unlock()
	s.dispatch(ctx, ev)
	return &ev, nil
}

// Resolve marks alert resolved with recovery notification.
func (s *Service) Resolve(ctx context.Context, alertID string) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("alerting unavailable")
	}
	var ev AlertEvent
	if err := s.DB.WithContext(ctx).Where("id = ?", alertID).First(&ev).Error; err != nil {
		return err
	}
	now := time.Now().UTC()
	ev.Status = StatusResolved
	ev.ResolvedAt = &now
	ev.UpdatedAt = now
	if err := s.DB.WithContext(ctx).Save(&ev).Error; err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.cache, ev.Fingerprint)
	s.mu.Unlock()
	if s.RecoveryEnabled {
		s.dispatch(ctx, ev)
	}
	return nil
}

// Acknowledge marks alert acknowledged.
func (s *Service) Acknowledge(ctx context.Context, alertID string) error {
	return s.setStatus(ctx, alertID, StatusAcknowledged)
}

// Silence suppresses alert until expiry.
func (s *Service) Silence(ctx context.Context, alertID, reason, by string, until time.Time) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("alerting unavailable")
	}
	if err := s.setStatus(ctx, alertID, StatusSilenced); err != nil {
		return err
	}
	sil := AlertSilence{
		ID:         uuid.New().String(),
		AlertID:    alertID,
		RuleID:     alertRuleID(ctx, s.DB, alertID),
		Reason:     reason,
		SilencedBy: by,
		ExpiresAt:  until.UTC(),
	}
	return s.DB.WithContext(ctx).Create(&sil).Error
}

func (s *Service) setStatus(ctx context.Context, alertID, status string) error {
	return s.DB.WithContext(ctx).Model(&AlertEvent{}).Where("id = ?", alertID).Updates(map[string]any{
		"status":     status,
		"updated_at": time.Now().UTC(),
	}).Error
}

func (s *Service) cooldownDuration() time.Duration {
	if s.DefaultCooldown <= 0 {
		return 5 * time.Minute
	}
	return s.DefaultCooldown
}

func (s *Service) dispatch(ctx context.Context, ev AlertEvent) {
	n := AlertNotification{
		AlertID:     ev.ID,
		RuleID:      ev.RuleID,
		Severity:    ev.Severity,
		Status:      ev.Status,
		Summary:     ev.Summary,
		SafeDetails: ev.SafeDetails,
	}
	s.enqueueAndSend(ctx, ev, n)
}

func (s *Service) isSilenced(ctx context.Context, ruleID string, now time.Time) bool {
	var count int64
	err := s.DB.WithContext(ctx).Model(&AlertSilence{}).
		Where("rule_id = ? AND expires_at > ?", ruleID, now).
		Count(&count).Error
	return err == nil && count > 0
}

func alertRuleID(ctx context.Context, db *gorm.DB, alertID string) string {
	if db == nil || strings.TrimSpace(alertID) == "" {
		return ""
	}
	var ev AlertEvent
	if err := db.WithContext(ctx).Select("rule_id").Where("id = ?", alertID).First(&ev).Error; err != nil {
		return ""
	}
	return ev.RuleID
}

func sanitizeDetails(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) > 1024 {
		raw = raw[:1024]
	}
	for _, token := range []string{"TEST_ACCESS_TOKEN", "TEST_REFRESH_TOKEN", "TEST_APP_SECRET", "password", "secret"} {
		if strings.Contains(strings.ToLower(raw), strings.ToLower(token)) {
			return "[redacted]"
		}
	}
	return raw
}
