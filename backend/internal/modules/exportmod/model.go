package exportmod

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/id"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	ExportStatusPending = "pending"
	ExportStatusRunning = "running"
	ExportStatusSuccess = "success"
	ExportStatusFailed  = "failed"
	ExportStatusExpired = "expired"
	ExportTypeOrders    = "orders"
	ExportTypeProducts  = "products"
	ExportTypeInventory = "inventory"
	ExportTypeCustomers = "customers"
	ExportTypeTasks     = "tasks"
	ExportTypeAuditLogs = "audit_logs"
)

// ExportJob is a tenant-scoped async export task.
type ExportJob struct {
	ID           uuid.UUID      `gorm:"type:char(36);primaryKey" json:"id"`
	TenantID     int64          `gorm:"not null;default:0;index" json:"tenantId"`
	ExportType   string         `gorm:"size:64;index;not null" json:"exportType"`
	Status       string         `gorm:"size:32;index;not null" json:"status"`
	ShopID       *uuid.UUID     `gorm:"type:char(36);index" json:"shopId,omitempty"`
	FileID       *uuid.UUID     `gorm:"type:char(36);index" json:"fileId,omitempty"`
	RowCount     int64          `gorm:"not null;default:0" json:"rowCount"`
	MaskedPII    bool           `gorm:"not null;default:true" json:"maskedPii"`
	Filters      datatypes.JSON `gorm:"type:jsonb" json:"filters,omitempty"`
	ErrorMessage string         `gorm:"type:text" json:"errorMessage,omitempty"`
	CreatedBy    *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	StartedAt    *time.Time     `json:"startedAt,omitempty"`
	FinishedAt   *time.Time     `json:"finishedAt,omitempty"`
	ExpiresAt    *time.Time     `json:"expiresAt,omitempty"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
}

func (ExportJob) TableName() string { return "export_jobs" }

func (j *ExportJob) BeforeCreate(tx *gorm.DB) error {
	id.Ensure(&j.ID)
	return nil
}
