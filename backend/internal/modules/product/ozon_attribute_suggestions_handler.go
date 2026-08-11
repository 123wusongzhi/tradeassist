package product

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

const ozonAttributeSuggestionMaxBodyBytes = 256 << 10

// SuggestOzonAttributesHTTP POST
// /api/v1/products/:id/ai/ozon-attribute-suggestions
func (h *Handler) SuggestOzonAttributesHTTP(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "products unavailable")
		return
	}
	if h.denyWrite(c) {
		return
	}
	productID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil || productID == uuid.Nil {
		response.JSON(c, http.StatusBadRequest, response.CodeBadRequest, "invalid product id", gin.H{"errorCode": OzonAttributeSuggestionInvalid})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, ozonAttributeSuggestionMaxBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var body OzonAttributeSuggestionBody
	if err := decoder.Decode(&body); err != nil {
		response.JSON(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body", gin.H{"errorCode": OzonAttributeSuggestionInvalid})
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		response.JSON(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body", gin.H{"errorCode": OzonAttributeSuggestionInvalid})
		return
	}
	result, err := h.Svc.SuggestOzonAttributes(c, productID, body, adminUUID(c))
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, result)
}
