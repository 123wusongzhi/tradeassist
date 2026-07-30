package storagepublic

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/pkg/httppublic"
)

// ValidationIssue is one public_base rule violation.
type ValidationIssue struct {
	Key     string `json:"key"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// ValidatePublicBaseResult summarizes public URL prefix validation.
type ValidatePublicBaseResult struct {
	Valid   bool              `json:"valid"`
	Base    string            `json:"base,omitempty"`
	Checks  []ValidationIssue `json:"checks"`
	Warning bool              `json:"warning"`
}

// ValidatePublicBase validates storage public_base for the given environment profile.
func ValidatePublicBase(raw string, appEnv string) ValidatePublicBaseResult {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	out := ValidatePublicBaseResult{Base: raw}
	if raw == "" {
		out.Checks = append(out.Checks, ValidationIssue{
			Key: "required", Status: "failed", Message: "未配置 public_base",
		})
		return out
	}
	if strings.HasPrefix(raw, "/") {
		out.Checks = append(out.Checks, ValidationIssue{
			Key: "relative_path", Status: "failed", Message: "不能使用相对路径",
		})
		return out
	}
	if strings.HasPrefix(strings.ToLower(raw), "file:") {
		out.Checks = append(out.Checks, ValidationIssue{
			Key: "file_scheme", Status: "failed", Message: "不能使用 file://",
		})
		return out
	}
	if !strings.Contains(raw, "://") {
		out.Checks = append(out.Checks, ValidationIssue{
			Key: "scheme", Status: "failed", Message: "必须是完整 http(s) URL",
		})
		return out
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		out.Checks = append(out.Checks, ValidationIssue{
			Key: "parse", Status: "failed", Message: "URL 格式无效",
		})
		return out
	}
	if u.User != nil {
		out.Checks = append(out.Checks, ValidationIssue{
			Key: "credentials", Status: "failed", Message: "URL 不能包含用户名密码",
		})
		return out
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		out.Checks = append(out.Checks, ValidationIssue{
			Key: "scheme", Status: "failed", Message: "仅允许 http 或 https",
		})
		return out
	}
	if config.IsStagingOrProduction(appEnv) && scheme != "https" {
		out.Checks = append(out.Checks, ValidationIssue{
			Key: "https", Status: "failed", Message: "staging/production 必须使用 HTTPS",
		})
		return out
	}
	if scheme == "https" {
		out.Checks = append(out.Checks, ValidationIssue{Key: "https", Status: "passed", Message: "已使用 HTTPS"})
	} else {
		out.Checks = append(out.Checks, ValidationIssue{Key: "https", Status: "warning", Message: "建议使用 HTTPS"})
		out.Warning = true
	}
	if !httppublic.IsPublicHTTPURL(raw) {
		out.Checks = append(out.Checks, ValidationIssue{
			Key: "private_host", Status: "failed", Message: "不能指向 localhost、内网或私网地址",
		})
		return out
	}
	out.Checks = append(out.Checks, ValidationIssue{Key: "public_host", Status: "passed", Message: "主机名通过公网校验"})
	if err := assertHostNotPrivate(context.Background(), u.Hostname()); err != nil {
		out.Checks = append(out.Checks, ValidationIssue{
			Key: "dns_private", Status: "failed", Message: "域名解析到私网地址",
		})
		return out
	}
	out.Valid = true
	return out
}

// ValidatePublicBaseError returns an error when validation fails.
func ValidatePublicBaseError(raw, appEnv string) error {
	res := ValidatePublicBase(raw, appEnv)
	if res.Valid {
		return nil
	}
	msg := "STORAGE_PUBLIC_BASE_INVALID"
	if len(res.Checks) > 0 {
		msg = fmt.Sprintf("%s: %s", msg, res.Checks[len(res.Checks)-1].Message)
	}
	return fmt.Errorf("%s", msg)
}

const storagePublicCheckPrefix = "system-tests/storage-public-check/"

// PublicCheckObjectKey returns a test object key with the required prefix.
func PublicCheckObjectKey(day, id string) string {
	day = strings.Trim(day, "/")
	id = strings.Trim(id, "/")
	return fmt.Sprintf("%s%s/%s.png", storagePublicCheckPrefix, day, id)
}
