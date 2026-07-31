package collectextension

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CreatePairingResult is the admin-facing one-time pairing code payload.
type CreatePairingResult struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// ExchangePairingBody is the device-side pairing exchange request.
type ExchangePairingBody struct {
	Code       string `json:"code"`
	DeviceName string `json:"deviceName"`
}

// ExchangePairingResult returns the long-lived, revocable device token once.
type ExchangePairingResult struct {
	Device      DeviceDTO `json:"device"`
	DeviceToken string    `json:"deviceToken"`
}

// DeviceDTO is the API-facing browser extension device shape (no token/hash).
type DeviceDTO struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	Status     string     `json:"status"`
	ExpiresAt  time.Time  `json:"expiresAt"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
}

func deviceToDTO(d *BrowserExtensionDevice) DeviceDTO {
	if d == nil {
		return DeviceDTO{}
	}
	status := strings.TrimSpace(d.Status)
	if status == DeviceStatusActive && !d.ExpiresAt.IsZero() && time.Now().UTC().After(d.ExpiresAt) {
		status = DeviceStatusExpired
	}
	var lastUsedAt *time.Time
	if d.LastUsedAt != nil {
		cp := *d.LastUsedAt
		lastUsedAt = &cp
	}
	return DeviceDTO{
		ID:         d.ID,
		Name:       d.Name,
		Status:     status,
		ExpiresAt:  d.ExpiresAt,
		LastUsedAt: lastUsedAt,
		CreatedAt:  d.CreatedAt,
	}
}

// CreateInteractiveTaskBody is the device-side interactive collect task request.
type CreateInteractiveTaskBody struct {
	Source string `json:"source"`
	URL    string `json:"url"`
}

// SubmitInteractiveResultBody is the device-side task completion payload.
// Product is the normalized product JSON produced by the browser adapter.
type SubmitInteractiveResultBody struct {
	Product json.RawMessage `json:"product"`
}

// SubmitInteractiveFailureBody is the device-side task failure payload.
type SubmitInteractiveFailureBody struct {
	ErrorCode string `json:"errorCode"`
	Message   string `json:"message"`
}
