package storagepublic

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// Handler serves storage public access diagnostics.
type Handler struct {
	Svc   *Service
	OpLog *operationlog.Service
	DB    *gorm.DB
}

func (h *Handler) requireSettingsManage(c *gin.Context) bool {
	if h == nil || h.DB == nil {
		response.Fail(c, 500, response.CodeInternalError, "storage public test unavailable")
		return false
	}
	return adminperm.RequirePermission(c, h.DB, adminperm.PermSettingsManage)
}

// TestPublicAccess POST /api/v1/storage/test-public-access
func (h *Handler) TestPublicAccess(c *gin.Context) {
	if !h.requireSettingsManage(c) {
		return
	}
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "storage public test unavailable")
		return
	}
	res, err := h.Svc.TestPublicAccess(c.Request.Context())
	if err != nil {
		if h.OpLog != nil {
			_ = h.OpLog.Write(c, operationlog.WriteOpts{
				Action:   "storage.public_access.test",
				Resource: "storage",
				Status:   "failed",
				Message:  err.Error(),
			})
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	st := "success"
	msg := res.Message
	if !res.OK {
		st = "failed"
		if msg == "" {
			msg = "图片地址无法被外部平台访问，请检查公开访问域名、HTTPS 证书和 Bucket 权限"
		}
		if h.OpLog != nil {
			_ = h.OpLog.Write(c, operationlog.WriteOpts{
				Action:   "storage.public_access.test",
				Resource: "storage",
				Status:   st,
				Message:  msg,
			})
		}
		saveLatest(res)
		response.JSON(c, 400, response.CodeBadRequest, msg, gin.H{
			"ok":               res.OK,
			"errorCode":        res.ErrorCode,
			"message":          msg,
			"technicalDetails": res.TechnicalDetails,
			"storageKind":      res.StorageKind,
			"testDeleted":      res.TestDeleted,
		})
		return
	}
	if h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			Action:   "storage.public_access.test",
			Resource: "storage",
			Status:   st,
			Message:  msg,
		})
	}
	saveLatest(res)
	response.OK(c, gin.H{
		"ok":               true,
		"message":          msg,
		"technicalDetails": res.TechnicalDetails,
		"storageKind":      res.StorageKind,
		"testDeleted":      res.TestDeleted,
	})
}

// GetLatestPublicCheck GET /api/v1/settings/storage/public-check/latest
func (h *Handler) GetLatestPublicCheck(c *gin.Context) {
	if !h.requireSettingsManage(c) {
		return
	}
	snap := getLatest()
	if snap == nil {
		response.OK(c, gin.H{"status": "not_run", "message": "尚未执行公网访问测试"})
		return
	}
	response.OK(c, snap)
}
