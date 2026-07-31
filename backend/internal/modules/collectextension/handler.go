package collectextension

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

const deviceContextKey = "collect.browser_extension.device"
const smallJSONBodyLimit int64 = 32 * 1024

type Handler struct {
	Svc *Service
}

func adminIdentity(c *gin.Context) (int64, uuid.UUID, error) {
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return 0, uuid.Nil, err
	}
	raw, ok := c.Get(ctxkey.AdminID)
	if !ok {
		return 0, uuid.Nil, errors.New("admin identity missing")
	}
	rawID, ok := raw.(string)
	if !ok {
		return 0, uuid.Nil, errors.New("admin identity invalid")
	}
	adminID, err := uuid.Parse(strings.TrimSpace(rawID))
	if err != nil || adminID == uuid.Nil {
		return 0, uuid.Nil, errors.New("admin identity invalid")
	}
	return tenantID, adminID, nil
}

func deviceFromContext(c *gin.Context) (*BrowserExtensionDevice, bool) {
	raw, ok := c.Get(deviceContextKey)
	if !ok {
		return nil, false
	}
	device, ok := raw.(*BrowserExtensionDevice)
	return device, ok && device != nil
}

func bearerToken(c *gin.Context) string {
	header := strings.TrimSpace(c.GetHeader("Authorization"))
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

// DeviceAuth authenticates only opaque, revocable browser-extension tokens.
func (h *Handler) DeviceAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h == nil || h.Svc == nil {
			response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "浏览器扩展采集暂不可用")
			c.Abort()
			return
		}
		device, err := h.Svc.AuthenticateDevice(c.Request.Context(), bearerToken(c))
		if err != nil {
			response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "扩展连接无效、已过期或已撤销")
			c.Abort()
			return
		}
		c.Set(deviceContextKey, device)
		c.Set(ctxkey.AdminID, device.AdminUserID.String())
		c.Set(ctxkey.AdminUsername, "browser-extension")
		c.Set(ctxkey.TenantID, device.TenantID)
		tc := security.BuildTenantContext(c, device.TenantID, device.AdminUserID, uuid.Nil, "", nil, nil)
		tc.AuthSource = security.AuthSourceBrowserExtension
		security.SetGin(c, tc)
		if !adminperm.RequireWrite(c, h.Svc.DB, adminperm.PermProductWrite) {
			c.Abort()
			return
		}
		c.Next()
	}
}

func (h *Handler) CreatePairing(c *gin.Context) {
	tenantID, adminID, err := adminIdentity(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, err.Error())
		return
	}
	if !adminperm.RequireWrite(c, h.Svc.DB, adminperm.PermProductWrite) {
		return
	}
	out, err := h.Svc.CreatePairing(c.Request.Context(), tenantID, adminID)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	if h.Svc.OpLog != nil {
		_ = h.Svc.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: &adminID,
			Action:      "collect.browser_extension.pairing.create",
			Resource:    "browser_extension_pairing",
			Status:      "success",
			Message:     "one-time pairing code created",
		})
	}
	response.OK(c, out)
}

func (h *Handler) ExchangePairing(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, smallJSONBodyLimit)
	var body ExchangePairingBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "连接信息格式无效")
		return
	}
	out, err := h.Svc.ExchangePairing(c.Request.Context(), body.Code, body.DeviceName)
	if err != nil {
		if errors.Is(err, ErrPairingInvalid) {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "配对码无效、已使用或已过期")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) ListDevices(c *gin.Context) {
	tenantID, _, err := adminIdentity(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, err.Error())
		return
	}
	if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermProductView) {
		return
	}
	out, err := h.Svc.ListDevices(c.Request.Context(), tenantID)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"list": out})
}

func (h *Handler) RevokeDevice(c *gin.Context) {
	tenantID, adminID, err := adminIdentity(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, err.Error())
		return
	}
	if !adminperm.RequireWrite(c, h.Svc.DB, adminperm.PermProductWrite) {
		return
	}
	deviceID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "扩展设备 ID 无效")
		return
	}
	err = h.Svc.RevokeDevice(c.Request.Context(), tenantID, deviceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "扩展设备不存在")
			return
		}
		if errors.Is(err, ErrDeviceAlreadyRevoked) {
			response.Fail(c, http.StatusConflict, response.CodeBadRequest, "扩展设备已经撤销")
			return
		}
		response.HandleError(c, err)
		return
	}
	if h.Svc.OpLog != nil {
		_ = h.Svc.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: &adminID,
			Action:      "collect.browser_extension.device.revoke",
			Resource:    "browser_extension_device",
			ResourceID:  deviceID.String(),
			Status:      "success",
			Message:     "browser extension device revoked",
		})
	}
	response.OK(c, gin.H{"revoked": true})
}

func (h *Handler) Session(c *gin.Context) {
	device, ok := deviceFromContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "扩展连接无效、已过期或已撤销")
		return
	}
	response.OK(c, deviceToDTO(device))
}

func (h *Handler) CreateTask(c *gin.Context) {
	device, ok := deviceFromContext(c)
	if !ok || h.Svc == nil || h.Svc.Collect == nil {
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "浏览器扩展采集暂不可用")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, smallJSONBodyLimit)
	var body CreateInteractiveTaskBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "任务参数格式无效")
		return
	}
	out, err := h.Svc.Collect.CreateBrowserExtensionTask(c.Request.Context(), collect.BrowserExtensionTaskInput{
		TenantID: device.TenantID,
		AdminID:  device.AdminUserID,
		DeviceID: device.ID,
		Source:   body.Source,
		URL:      body.URL,
	})
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

func (h *Handler) SubmitResult(c *gin.Context) {
	device, ok := deviceFromContext(c)
	if !ok || h.Svc == nil || h.Svc.Collect == nil {
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "浏览器扩展采集暂不可用")
		return
	}
	taskID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "采集任务 ID 无效")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 3*1024*1024+64*1024)
	var body SubmitInteractiveResultBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "采集结果格式无效或超过大小限制")
		return
	}
	out, err := h.Svc.Collect.CompleteBrowserExtensionTask(c.Request.Context(), collect.BrowserExtensionResultInput{
		TenantID:    device.TenantID,
		AdminID:     device.AdminUserID,
		DeviceID:    device.ID,
		TaskID:      taskID,
		ProductJSON: body.Product,
	})
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "采集任务不存在")
		case errors.Is(err, collect.ErrBrowserExtensionTaskBusy):
			response.Fail(c, http.StatusConflict, response.CodeBadRequest, "采集任务正在接收结果，请稍后重试")
		case errors.Is(err, collect.ErrBrowserExtensionTaskInvalid):
			response.Fail(c, http.StatusForbidden, response.CodeForbidden, "采集任务不属于当前扩展设备")
		case errors.Is(err, collect.ErrBrowserExtensionTaskNotActive):
			response.Fail(c, http.StatusForbidden, response.CodeForbidden, "采集任务已结束，不能再次提交")
		default:
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		}
		return
	}
	response.OK(c, out)
}

func (h *Handler) SubmitFailure(c *gin.Context) {
	device, ok := deviceFromContext(c)
	if !ok || h.Svc == nil || h.Svc.Collect == nil {
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "浏览器扩展采集暂不可用")
		return
	}
	taskID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "采集任务 ID 无效")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, smallJSONBodyLimit)
	var body SubmitInteractiveFailureBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "失败信息格式无效")
		return
	}
	out, err := h.Svc.Collect.FailBrowserExtensionTask(c.Request.Context(), collect.BrowserExtensionFailureInput{
		TenantID: device.TenantID,
		AdminID:  device.AdminUserID,
		DeviceID: device.ID,
		TaskID:   taskID,
		Code:     body.ErrorCode,
		Message:  body.Message,
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "采集任务不存在")
			return
		}
		if errors.Is(err, collect.ErrBrowserExtensionTaskBusy) {
			response.Fail(c, http.StatusConflict, response.CodeBadRequest, "采集任务正在接收结果，请稍后重试")
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}
