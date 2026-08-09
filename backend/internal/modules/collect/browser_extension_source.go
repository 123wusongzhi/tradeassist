package collect

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/pkg/collectdomain"
)

// validateBrowserExtensionSourceAndURL accepts sources the sidepanel extension may submit.
// Playwright / OpenCLI task creation is unchanged and remains outside this path.
func validateBrowserExtensionSourceAndURL(source, urlStr string) error {
	src := strings.ToLower(strings.TrimSpace(source))
	switch src {
	case "taobao_tmall", "taobao":
		if !isTaobaoTmallCollectSource(source) {
			return fmt.Errorf("browser extension unsupported source %q", source)
		}
		return validateTaobaoTmallCollectURL(urlStr)
	case "1688":
		return validate1688CollectURL(urlStr)
	default:
		return fmt.Errorf("browser extension only supports taobao_tmall or 1688 source")
	}
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
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("INVALID_URL:链接协议必须是 http 或 https")
	}
	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	if platform, ok := collectdomain.DetectPlatform(host); !ok || platform != collectdomain.Platform1688 {
		return fmt.Errorf("INVALID_URL:请输入 1688 商品详情页链接（detail.1688.com/offer/*.html）")
	}
	path := u.Path
	query := u.RawQuery
	offerPath := strings.Contains(strings.ToLower(path), "/offer/") ||
		strings.Contains(strings.ToLower(query), "offerid=") ||
		strings.HasSuffix(strings.ToLower(path), "offer.html") ||
		strings.HasSuffix(strings.ToLower(path), "offerid.html")
	if !offerPath {
		return fmt.Errorf("UNSUPPORTED_1688_URL:当前链接不是 1688 商品详情页，请打开 offer 详情后重试")
	}
	return nil
}

func isBrowserExtension1688Source(source string) bool {
	return strings.EqualFold(strings.TrimSpace(source), "1688")
}
