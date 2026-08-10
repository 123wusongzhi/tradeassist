package product

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// RecommendOzonCategories POST
// /api/v1/products/:id/platform-configs/ozon/category-recommendations
func (h *Handler) RecommendOzonCategories(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "products unavailable")
		return
	}
	if h.denyWrite(c) {
		return
	}
	if !adminperm.CanOperateStore(c, h.Svc.DB) {
		response.JSON(c, http.StatusForbidden, response.CodeStorePermissionDenied, "当前账号无店铺操作权限", gin.H{
			"errorCode": OzonCategoryRecommendationInvalid,
		})
		return
	}
	productID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil || productID == uuid.Nil {
		response.JSON(c, http.StatusBadRequest, response.CodeBadRequest, "invalid product id", gin.H{
			"errorCode": OzonCategoryRecommendationInvalid,
		})
		return
	}
	var body OzonCategoryRecommendationBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.JSON(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body", gin.H{
			"errorCode": OzonCategoryRecommendationInvalid,
		})
		return
	}
	result, err := h.Svc.RecommendOzonCategories(c, productID, body, adminUUID(c))
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, result)
}
