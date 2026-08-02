package pricing

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// Handler exposes pricing HTTP API.
type Handler struct {
	Svc *Service
}

func pricingTenantID(c *gin.Context) (int64, bool) {
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.HandleError(c, err)
		return 0, false
	}
	return tenantID, true
}

func handlePricingError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.Fail(c, 404, response.CodeNotFound, "resource not found")
		return
	}
	response.HandleError(c, err)
}

func adminUUID(c *gin.Context) *uuid.UUID {
	if v, ok := c.Get(ctxkey.AdminID); ok {
		if s, ok := v.(string); ok {
			if u, err := uuid.Parse(strings.TrimSpace(s)); err == nil {
				return &u
			}
		}
	}
	return nil
}

// Calculate POST /api/v1/pricing/calculate
func (h *Handler) Calculate(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "pricing unavailable")
		return
	}
	var body CalculateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	tenantID, ok := pricingTenantID(c)
	if !ok {
		return
	}
	out, err := h.Svc.Calculate(c, tenantID, body)
	if err != nil {
		handlePricingError(c, err)
		return
	}
	response.OK(c, out)
}

// ApplyProduct POST /api/v1/products/:id/pricing/apply
func (h *Handler) ApplyProduct(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "pricing unavailable")
		return
	}
	pid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid product id")
		return
	}
	var body ProductApplyBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	tenantID, ok := pricingTenantID(c)
	if !ok {
		return
	}
	if !body.Confirm {
		out, err := h.Svc.PreviewProduct(c, tenantID, pid, body)
		if err != nil {
			handlePricingError(c, err)
			return
		}
		response.OK(c, out)
		return
	}
	out, err := h.Svc.ApplyProduct(c, tenantID, pid, body, adminUUID(c))
	if err != nil {
		handlePricingError(c, err)
		return
	}
	response.OK(c, out)
}

// BatchApply POST /api/v1/products/pricing/batch-apply
func (h *Handler) BatchApply(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "pricing unavailable")
		return
	}
	var body BatchApplyBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	tenantID, ok := pricingTenantID(c)
	if !ok {
		return
	}
	if !body.Confirm {
		out, err := h.Svc.BatchPreview(c, tenantID, body)
		if err != nil {
			handlePricingError(c, err)
			return
		}
		response.OK(c, out)
		return
	}
	out, err := h.Svc.BatchApply(c, tenantID, body, adminUUID(c))
	if err != nil {
		handlePricingError(c, err)
		return
	}
	response.OK(c, out)
}
