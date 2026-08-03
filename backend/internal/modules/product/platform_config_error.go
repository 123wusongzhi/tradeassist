package product

import "net/http"

type platformConfigError struct {
	code    string
	message string
}

func (e *platformConfigError) Error() string {
	if e == nil {
		return ""
	}
	return e.message
}

func (e *platformConfigError) HTTPStatus() int { return http.StatusBadRequest }

func (e *platformConfigError) SafeMessage() string {
	if e == nil || e.message == "" {
		return "商品平台配置不完整"
	}
	return e.message
}

func (e *platformConfigError) SafeData() any {
	return map[string]any{"errorCode": e.code}
}

func newPlatformConfigError(message string) error {
	return &platformConfigError{code: "PRODUCT_PLATFORM_CONFIG_INVALID", message: message}
}

func newOzonPlatformConfigError(message string) error {
	return &platformConfigError{code: "OZON_PRODUCT_CONFIG_INVALID", message: message}
}
