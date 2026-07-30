package adminperm

import "strings"

// Permission keys for role matrix and profile export.
const (
	PermProductView        = "product.view"
	PermProductWrite       = "product.write"
	PermAITextApply        = "ai_text.apply"
	PermAIImageApply       = "ai_image.apply"
	PermPublishCreateDraft = "publish.create_draft"
	PermOrderView          = "order.view"
	PermOrderOperate       = "order.operate"
	PermSKUBind            = "sku.bind"
	PermInventoryView      = "inventory.view"
	PermInventoryOperate   = "inventory.operate"
	PermCustomerView       = "customer.view"
	PermCustomerOperate    = "customer.operate"
	PermTaskRetry          = "task.retry"
	PermSettingsManage     = "settings.manage"
	PermUserManage         = "user.manage"
	PermOperationLogView   = "operationlog.view"
	PermStoreView          = "store.view"
	PermStoreOperate       = "store.operate"
	// P4 security permissions
	PermSecuritySessionManage = "security.session.manage"
	PermSecurityKeyRotate     = "security.key.rotate"
	PermAuditRead             = "audit.read"
	PermAuditExport           = "audit.export"
	PermPIIReadMasked         = "pii.read_masked"
	PermPIIReadFull           = "pii.read_full"
	PermPIIExport             = "pii.export"
	PermConfigRead            = "config.read"
	PermConfigManage          = "config.manage"
	PermExportRead            = "export.read"
	PermExportCreate          = "export.create"
	// P5 observability permissions
	PermObservabilityRead   = "observability.read"
	PermObservabilityManage = "observability.manage"
	PermAlertsRead          = "alerts.read"
	PermAlertsAck           = "alerts.ack"
	PermAlertsSilence       = "alerts.silence"
	PermSLORead             = "slo.read"
	PermSLOManage           = "slo.manage"
	// P6 backup / restore / release / DR permissions
	PermBackupRead      = "backup.read"
	PermBackupCreate    = "backup.create"
	PermBackupVerify    = "backup.verify"
	PermBackupDownload  = "backup.download"
	PermBackupDelete    = "backup.delete"
	PermBackupHold      = "backup.hold"
	PermRestoreRead     = "restore.read"
	PermRestoreExecute  = "restore.execute"
	PermRestoreVerify   = "restore.verify"
	PermReleaseRead     = "release.read"
	PermReleaseCreate   = "release.create"
	PermReleaseExecute  = "release.execute"
	PermReleaseRollback = "release.rollback"
	PermDRRead          = "dr.read"
	PermDRExecute       = "dr.execute"
	// P8 operation task permissions
	PermOperationTaskEdit      = "operationtask.edit"
	PermOperationTaskReview    = "operationtask.review"
	PermOperationTaskExecute   = "operationtask.execute"
	PermOperationTaskRetry     = "operationtask.retry"
	PermOperationTaskAuditRead = "operationtask.audit.read"
	// P9 inventory sync permissions
	PermInventorySyncRead       = "inventory_sync.read"
	PermInventorySyncRun        = "inventory_sync.run"
	PermInventorySyncRerun      = "inventory_sync.rerun"
	PermInventorySnapshotRead   = "inventory_snapshot.read"
	PermSKUBindingRead          = "sku_binding.read"
	PermSKUBindingManage        = "sku_binding.manage"
	PermSKUBindingResolveManual = "sku_binding.resolve_manual"
	PermInventorySyncAuditRead  = "inventory_sync.audit.read"
)

var allPermissions = []string{
	PermProductView,
	PermProductWrite,
	PermAITextApply,
	PermAIImageApply,
	PermPublishCreateDraft,
	PermOrderView,
	PermOrderOperate,
	PermSKUBind,
	PermInventoryView,
	PermInventoryOperate,
	PermCustomerView,
	PermCustomerOperate,
	PermTaskRetry,
	PermSettingsManage,
	PermUserManage,
	PermOperationLogView,
	PermStoreView,
	PermStoreOperate,
	PermSecuritySessionManage,
	PermSecurityKeyRotate,
	PermAuditRead,
	PermAuditExport,
	PermPIIReadMasked,
	PermPIIReadFull,
	PermPIIExport,
	PermConfigRead,
	PermConfigManage,
	PermExportRead,
	PermExportCreate,
	PermObservabilityRead,
	PermObservabilityManage,
	PermAlertsRead,
	PermAlertsAck,
	PermAlertsSilence,
	PermSLORead,
	PermSLOManage,
	PermBackupRead,
	PermBackupCreate,
	PermBackupVerify,
	PermBackupDownload,
	PermBackupDelete,
	PermBackupHold,
	PermRestoreRead,
	PermRestoreExecute,
	PermRestoreVerify,
	PermReleaseRead,
	PermReleaseCreate,
	PermReleaseExecute,
	PermReleaseRollback,
	PermDRRead,
	PermDRExecute,
	PermOperationTaskEdit,
	PermOperationTaskReview,
	PermOperationTaskExecute,
	PermOperationTaskRetry,
	PermOperationTaskAuditRead,
	PermInventorySyncRead,
	PermInventorySyncRun,
	PermInventorySyncRerun,
	PermInventorySnapshotRead,
	PermSKUBindingRead,
	PermSKUBindingManage,
	PermSKUBindingResolveManual,
	PermInventorySyncAuditRead,
}

var adminPermissions = append([]string(nil), allPermissions...)

var reviewerPermissions = []string{
	PermOperationLogView,
	PermAuditRead,
	PermOperationTaskReview,
	PermOperationTaskExecute,
	PermOperationTaskRetry,
	PermOperationTaskAuditRead,
	PermInventorySyncRead,
	PermInventorySnapshotRead,
	PermSKUBindingRead,
	PermSKUBindingResolveManual,
	PermInventorySyncAuditRead,
}

var operatorPermissions = []string{
	PermProductView,
	PermProductWrite,
	PermAITextApply,
	PermAIImageApply,
	PermPublishCreateDraft,
	PermOrderView,
	PermOrderOperate,
	PermSKUBind,
	PermInventoryView,
	PermInventoryOperate,
	PermCustomerView,
	PermCustomerOperate,
	PermTaskRetry,
	PermOperationLogView,
	PermStoreView,
	PermStoreOperate,
	PermSecuritySessionManage,
	PermPIIReadMasked,
	PermAuditRead,
	PermConfigRead,
	PermObservabilityRead,
	PermAlertsRead,
	PermSLORead,
	PermBackupRead,
	PermRestoreRead,
	PermReleaseRead,
	PermDRRead,
	PermOperationTaskEdit,
	PermOperationTaskAuditRead,
	PermInventorySyncRead,
	PermInventorySyncRun,
	PermInventorySyncRerun,
	PermInventorySnapshotRead,
	PermSKUBindingRead,
	PermSKUBindingManage,
}

var readonlyPermissions = []string{
	PermProductView,
	PermOrderView,
	PermInventoryView,
	PermCustomerView,
	PermOperationLogView,
	PermStoreView,
	PermPIIReadMasked,
	PermAuditRead,
	PermConfigRead,
	PermObservabilityRead,
	PermAlertsRead,
	PermSLORead,
	PermBackupRead,
	PermRestoreRead,
	PermReleaseRead,
	PermDRRead,
	PermOperationTaskAuditRead,
	PermInventorySyncRead,
	PermInventorySnapshotRead,
	PermSKUBindingRead,
}

// PermissionsForRole returns granted permission keys for a role.
func PermissionsForRole(role string) []string {
	switch normalizeRole(role) {
	case RoleReadonly:
		return copyPermissions(readonlyPermissions)
	case RoleOperator:
		return copyPermissions(operatorPermissions)
	case RoleReviewer:
		return copyPermissions(reviewerPermissions)
	default:
		return copyPermissions(adminPermissions)
	}
}

func StrictPermissionsForRole(role string) []string {
	switch strictRole(role) {
	case RoleAdmin:
		return copyPermissions(adminPermissions)
	case RoleOperator:
		return copyPermissions(operatorPermissions)
	case RoleReadonly:
		return copyPermissions(readonlyPermissions)
	case RoleReviewer:
		return copyPermissions(reviewerPermissions)
	default:
		return []string{}
	}
}

// HasPermission checks whether role grants a permission key.
func HasPermission(role, perm string) bool {
	return permissionIn(PermissionsForRole(role), perm)
}

func StrictHasPermission(role, perm string) bool {
	return permissionIn(StrictPermissionsForRole(role), perm)
}

func copyPermissions(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func permissionIn(perms []string, perm string) bool {
	perm = strings.TrimSpace(perm)
	if perm == "" {
		return false
	}
	for _, p := range perms {
		if p == perm {
			return true
		}
	}
	return false
}
