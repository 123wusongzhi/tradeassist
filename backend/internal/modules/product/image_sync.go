package product

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

// SyncImagesBody selects which image groups to mirror into platform storage.
type SyncImagesBody struct {
	Scope string `json:"scope"` // all | main | detail
}

// SyncImagesResult summarizes mirror outcomes.
type SyncImagesResult struct {
	Synced  int      `json:"synced"`
	Skipped int      `json:"skipped"`
	Failed  int      `json:"failed"`
	Errors  []string `json:"errors,omitempty"`
}

var allowedCollectImageDomains = []string{
	"alicdn.com", "tbcdn.cn", "taobaocdn.com", "1688.com", "pinduoduo.com", "yangkeduo.com", "pddpic.com",
}

var resolveImageHost = func(ctx context.Context, host string) ([]net.IP, error) {
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	ips := make([]net.IP, 0, len(addrs))
	for _, addr := range addrs {
		ips = append(ips, addr.IP)
	}
	return ips, nil
}

var blockedImageIPPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001::/32"),
	netip.MustParsePrefix("2001:2::/48"),
	netip.MustParsePrefix("2001:10::/28"),
	netip.MustParsePrefix("2001:20::/28"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("ff00::/8"),
}

func allowedCollectImageHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	for _, domain := range allowedCollectImageDomains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

func isPublicImageIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() {
		return false
	}
	for _, prefix := range blockedImageIPPrefixes {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}

func validateExternalCollectImageURL(ctx context.Context, rawURL string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return nil, fmt.Errorf("unsafe image URL")
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") || !allowedCollectImageHost(host) {
		return nil, fmt.Errorf("unsafe image URL")
	}
	if ip := net.ParseIP(host); ip != nil {
		if !isPublicImageIP(ip) {
			return nil, fmt.Errorf("unsafe image URL")
		}
		return u, nil
	}
	ips, err := resolveImageHost(ctx, host)
	if err != nil || len(ips) == 0 {
		return nil, fmt.Errorf("unsafe image URL")
	}
	for _, ip := range ips {
		if !isPublicImageIP(ip) {
			return nil, fmt.Errorf("unsafe image URL")
		}
	}
	return u, nil
}

func isExternalCollectImageURL(rawURL string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := validateExternalCollectImageURL(ctx, rawURL)
	return err == nil
}

func safeImageTransport() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// A proxy may resolve or connect to a target outside this transport's
	// control, so image synchronization intentionally uses direct connections.
	transport.Proxy = nil
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := resolveImageHost(ctx, host)
		if err != nil || len(ips) == 0 {
			return nil, fmt.Errorf("unsafe image host")
		}
		var lastErr error
		for _, ip := range ips {
			if !isPublicImageIP(ip) {
				return nil, fmt.Errorf("unsafe image host")
			}
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		return nil, lastErr
	}
	return transport
}

func extFromImageBytes(data []byte, rawURL string) (string, string, error) {
	ct := http.DetectContentType(data)
	switch {
	case strings.Contains(ct, "jpeg"):
		return ".jpg", "image/jpeg", nil
	case strings.Contains(ct, "png"):
		return ".png", "image/png", nil
	case strings.Contains(ct, "webp"):
		return ".webp", "image/webp", nil
	case strings.Contains(ct, "gif"):
		return ".gif", "image/gif", nil
	}
	ext := strings.ToLower(filepath.Ext(strings.Split(rawURL, "?")[0]))
	switch ext {
	case ".jpg", ".jpeg":
		return ".jpg", "image/jpeg", nil
	case ".png":
		return ".png", "image/png", nil
	case ".webp":
		return ".webp", "image/webp", nil
	case ".gif":
		return ".gif", "image/gif", nil
	default:
		return "", "", fmt.Errorf("unsupported image type")
	}
}

func (s *Service) fetchRemoteImage(ctx context.Context, rawURL string) ([]byte, error) {
	u, err := validateExternalCollectImageURL(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "TradeMind-ImageSync/1.0")
	req.Header.Set("Accept", "image/*")
	cli := &http.Client{Timeout: 45 * time.Second, Transport: safeImageTransport()}
	cli.CheckRedirect = func(req *http.Request, _ []*http.Request) error {
		_, err := validateExternalCollectImageURL(req.Context(), req.URL.String())
		return err
	}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	const max = 15 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("image too large")
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("empty image")
	}
	return data, nil
}

// SyncImages mirrors external product images into configured storage.
func (s *Service) SyncImages(c *gin.Context, productID uuid.UUID, body SyncImagesBody, adminID *uuid.UUID, filesSvc *files.Service) (*SyncImagesResult, error) {
	if s == nil || s.DB == nil || filesSvc == nil {
		return nil, fmt.Errorf("product: misconfigured")
	}
	scope := strings.TrimSpace(strings.ToLower(body.Scope))
	if scope == "" {
		scope = "all"
	}

	prod, err := s.findTenantProduct(c, productID, "Images")
	if err != nil {
		return nil, err
	}
	if err := adminperm.EnsureProductOperate(c, s.DB, productID); err != nil {
		return nil, err
	}

	out := &SyncImagesResult{}
	for _, im := range prod.Images {
		imgType := strings.TrimSpace(strings.ToLower(im.ImageType))
		if imgType == ImageTypeDescription {
			imgType = ImageTypeDetail
		}
		if scope == "main" && imgType != ImageTypeMain {
			out.Skipped++
			continue
		}
		if scope == "detail" && imgType != ImageTypeDetail {
			out.Skipped++
			continue
		}
		if strings.TrimSpace(im.ObjectKey) != "" {
			out.Skipped++
			continue
		}
		src := strings.TrimSpace(im.OriginURL)
		if src == "" {
			src = strings.TrimSpace(im.PublicURL)
		}
		if src == "" || !isExternalCollectImageURL(src) {
			out.Skipped++
			continue
		}

		data, err := s.fetchRemoteImage(c.Request.Context(), src)
		if err != nil {
			out.Failed++
			out.Errors = append(out.Errors, fmt.Sprintf("%s: %v", im.ID.String(), err))
			continue
		}
		ext, ct, err := extFromImageBytes(data, src)
		if err != nil {
			out.Failed++
			out.Errors = append(out.Errors, fmt.Sprintf("%s: %v", im.ID.String(), err))
			continue
		}
		day := time.Now().UTC().Format("2006/01/02")
		objKey := fmt.Sprintf("%s/sync-%s%s", day, uuid.NewString(), ext)
		rec, err := filesSvc.SaveUntrustedProcessed(c.Request.Context(), files.SaveProcessedOpts{
			TenantID:    prod.TenantID,
			ObjectKey:   objKey,
			ContentType: ct,
			Data:        data,
			CreatedBy:   adminID,
		})
		if err != nil {
			out.Failed++
			out.Errors = append(out.Errors, fmt.Sprintf("%s: %v", im.ID.String(), err))
			continue
		}

		updates := map[string]interface{}{
			"object_key":  rec.ObjectKey,
			"storage_key": rec.ObjectKey,
			"public_url":  rec.PublicURL,
			"origin_url":  src,
			"source":      "sync",
		}
		if err := s.DB.WithContext(c.Request.Context()).Model(&ProductImage{}).
			Where("id = ? AND product_id = ?", im.ID, productID).
			Updates(updates).Error; err != nil {
			out.Failed++
			out.Errors = append(out.Errors, fmt.Sprintf("%s: %v", im.ID.String(), err))
			continue
		}
		out.Synced++
	}

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "product.image.sync",
			Resource:    "product",
			ResourceID:  productID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("scope=%s synced=%d skipped=%d failed=%d", scope, out.Synced, out.Skipped, out.Failed),
		})
	}
	return out, nil
}
