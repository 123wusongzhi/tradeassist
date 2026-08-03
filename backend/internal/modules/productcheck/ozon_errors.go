package productcheck

import (
	"errors"
	"net/http"

	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	platformozon "github.com/trademind-ai/trademind/backend/internal/providers/platform/ozon"
)

const (
	ozonCredentialInvalidMessage = "Ozon 店铺授权已失效或 API Key 已停用，请前往店铺管理更新凭证后重试"
	ozonUnavailableMessage       = "Ozon 服务暂时不可用，发布前检查未完成，请稍后重试"
	ozonUpstreamRejectedMessage  = "Ozon 发布前检查请求失败，请核对店铺授权和商品配置后重试"
)

type ozonReadinessError struct {
	status  int
	code    string
	message string
	err     error
}

func (e *ozonReadinessError) Error() string {
	if e == nil {
		return ""
	}
	return e.message
}

func (e *ozonReadinessError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func (e *ozonReadinessError) HTTPStatus() int { return e.status }

func (e *ozonReadinessError) SafeMessage() string { return e.message }

func (e *ozonReadinessError) SafeData() any {
	return map[string]any{"errorCode": e.code}
}

func newOzonReadinessError(status int, code, message string, err error) error {
	return &ozonReadinessError{status: status, code: code, message: message, err: err}
}

func ozonConfigShopMismatchError(err error) error {
	return newOzonReadinessError(
		http.StatusBadRequest,
		"OZON_CONFIG_SHOP_MISMATCH",
		"已保存的 Ozon 商品配置不属于当前店铺，请重新选择店铺并保存配置",
		err,
	)
}

func invalidOzonConfigError(message string, err error) error {
	return newOzonReadinessError(http.StatusBadRequest, "OZON_PRODUCT_CONFIG_INVALID", message, err)
}

func mapOzonReadinessError(err error) error {
	if err == nil {
		return nil
	}
	var alreadyMapped *ozonReadinessError
	if errors.As(err, &alreadyMapped) {
		return err
	}
	if errors.Is(err, platformp.ErrPlatformProductPublishPermissionDenied) {
		return newOzonReadinessError(http.StatusBadGateway, "OZON_CREDENTIAL_INVALID", ozonCredentialInvalidMessage, err)
	}
	if errors.Is(err, platformozon.ErrTemporaryUnavailable) {
		return newOzonReadinessError(http.StatusServiceUnavailable, "OZON_UPSTREAM_UNAVAILABLE", ozonUnavailableMessage, err)
	}
	var categoryErr *shop.OzonCategoryError
	if errors.As(err, &categoryErr) {
		switch categoryErr.Code {
		case shop.OzonShopRequired:
			return newOzonReadinessError(http.StatusBadRequest, "OZON_SHOP_REQUIRED", "请选择已授权且启用的 Ozon 店铺", err)
		case shop.OzonCategoryNotLeaf:
			return invalidOzonConfigError("已保存的 Ozon 类目不可用，请重新选择有效叶子类目", err)
		default:
			return newOzonReadinessError(http.StatusBadGateway, "OZON_UPSTREAM_REJECTED", ozonUpstreamRejectedMessage, err)
		}
	}
	return err
}

func mapOzonProviderError(err error) error {
	if err == nil {
		return nil
	}
	mapped := mapOzonReadinessError(err)
	var publicErr *ozonReadinessError
	if errors.As(mapped, &publicErr) {
		return mapped
	}
	return newOzonReadinessError(http.StatusBadGateway, "OZON_UPSTREAM_REJECTED", ozonUpstreamRejectedMessage, err)
}
