package collect

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var browserExtension1688OfferPathPattern = regexp.MustCompile(`(?i)^/offer/([0-9]+)\.html/?$`)

func canonicalBrowserExtensionSource(source string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "taobao_tmall", "taobao":
		return "taobao_tmall", nil
	case "1688":
		return "1688", nil
	default:
		return "", fmt.Errorf("browser extension only supports taobao_tmall or 1688 source")
	}
}

// validateBrowserExtensionSourceAndURL accepts sources the sidepanel extension may submit.
// Playwright / OpenCLI task creation is unchanged and remains outside this path.
func validateBrowserExtensionSourceAndURL(source, urlStr string) error {
	source, err := canonicalBrowserExtensionSource(source)
	if err != nil {
		return err
	}
	switch source {
	case "taobao_tmall":
		return validateTaobaoTmallCollectURL(urlStr)
	case "1688":
		return validate1688CollectURL(urlStr)
	}
	return fmt.Errorf("browser extension unsupported source %q", source)
}

func validate1688CollectURL(urlStr string) error {
	raw := strings.TrimSpace(urlStr)
	if raw == "" {
		return fmt.Errorf("INVALID_URL:请输入 1688 商品详情页链接")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("INVALID_URL:链接格式无效")
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return fmt.Errorf("INVALID_URL:1688 商品链接必须使用 https")
	}
	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	if host != "detail.1688.com" && host != "m.1688.com" {
		return fmt.Errorf("INVALID_URL:请输入 1688 商品详情页链接（detail.1688.com/offer/*.html）")
	}
	if browserExtension1688OfferPathPattern.MatchString(u.EscapedPath()) {
		return nil
	}
	for _, key := range []string{"offerId", "offerid", "object_id"} {
		if offerID := strings.TrimSpace(u.Query().Get(key)); offerID != "" && isASCIIDigits(offerID) {
			return nil
		}
	}
	return fmt.Errorf("UNSUPPORTED_1688_URL:当前链接不是 1688 商品详情页，请打开 offer 详情后重试")
}

func isASCIIDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func isBrowserExtension1688Source(source string) bool {
	return strings.EqualFold(strings.TrimSpace(source), "1688")
}
