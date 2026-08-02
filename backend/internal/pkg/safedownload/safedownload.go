package safedownload

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"golang.org/x/image/webp"
)

const (
	ErrSchemeNotAllowed   = "SAFE_DOWNLOAD_SCHEME_NOT_ALLOWED"
	ErrCredentialsInURL   = "SAFE_DOWNLOAD_CREDENTIALS_IN_URL"
	ErrPrivateHost        = "SAFE_DOWNLOAD_PRIVATE_HOST"
	ErrPrivateIP          = "SAFE_DOWNLOAD_PRIVATE_IP"
	ErrMetadataEndpoint   = "SAFE_DOWNLOAD_METADATA_ENDPOINT"
	ErrTooManyRedirects   = "SAFE_DOWNLOAD_TOO_MANY_REDIRECTS"
	ErrResponseTooLarge   = "SAFE_DOWNLOAD_RESPONSE_TOO_LARGE"
	ErrInvalidContentType = "SAFE_DOWNLOAD_INVALID_CONTENT_TYPE"
	ErrImageDecodeFailed  = "SAFE_DOWNLOAD_IMAGE_DECODE_FAILED"
	ErrImageDimensions    = "SAFE_DOWNLOAD_IMAGE_DIMENSIONS_EXCEEDED"
	ErrDownloadFailed     = "SAFE_DOWNLOAD_FAILED"
)

// Options configures safe HTTP download behavior.
type Options struct {
	MaxBodyBytes    int64
	MaxRedirects    int
	ConnectTimeout  time.Duration
	ResponseTimeout time.Duration
	RequireImage    bool
	MaxImagePixels  int64
	MaxImageWidth   int
	MaxImageHeight  int
	UserAgent       string
}

// DefaultOptions returns conservative defaults for image download.
func DefaultOptions() Options {
	return Options{
		MaxBodyBytes:    10 << 20,
		MaxRedirects:    5,
		ConnectTimeout:  10 * time.Second,
		ResponseTimeout: 30 * time.Second,
		RequireImage:    true,
		MaxImagePixels:  50_000_000,
		MaxImageWidth:   8192,
		MaxImageHeight:  8192,
		UserAgent:       "TradeMind-SafeDownload/1.0",
	}
}

// Result holds downloaded bytes and metadata.
type Result struct {
	Data        []byte
	ContentType string
	FinalURL    string
}

// Download fetches rawURL with SSRF protections.
func Download(ctx context.Context, rawURL string, opts Options) (*Result, error) {
	if opts.MaxBodyBytes <= 0 {
		opts.MaxBodyBytes = 10 << 20
	}
	if opts.MaxRedirects <= 0 {
		opts.MaxRedirects = 5
	}
	if opts.ConnectTimeout <= 0 {
		opts.ConnectTimeout = 10 * time.Second
	}
	if opts.ResponseTimeout <= 0 {
		opts.ResponseTimeout = 30 * time.Second
	}
	if opts.MaxImagePixels <= 0 {
		opts.MaxImagePixels = 50_000_000
	}
	if opts.MaxImageWidth <= 0 {
		opts.MaxImageWidth = 8192
	}
	if opts.MaxImageHeight <= 0 {
		opts.MaxImageHeight = 8192
	}
	if strings.TrimSpace(opts.UserAgent) == "" {
		opts.UserAgent = "TradeMind-SafeDownload/1.0"
	}

	current := strings.TrimSpace(rawURL)
	redirects := 0
	for {
		if err := validateURL(ctx, current); err != nil {
			return nil, err
		}
		data, ct, loc, err := fetchOnce(ctx, current, opts)
		if err != nil {
			return nil, err
		}
		if loc != "" {
			redirects++
			if redirects > opts.MaxRedirects {
				return nil, fmt.Errorf("%s: exceeded %d redirects", ErrTooManyRedirects, opts.MaxRedirects)
			}
			current = loc
			continue
		}
		if opts.RequireImage {
			if err := validateImageBytes(data, ct, opts); err != nil {
				return nil, err
			}
		}
		return &Result{Data: data, ContentType: ct, FinalURL: current}, nil
	}
}

func fetchOnce(ctx context.Context, rawURL string, opts Options) ([]byte, string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", "", fmt.Errorf("%s: %w", ErrDownloadFailed, err)
	}
	req.Header.Set("User-Agent", opts.UserAgent)
	req.Header.Set("Accept", "image/*,*/*;q=0.8")

	cli := safeHTTPClient(opts)
	resp, err := cli.Do(req)
	if err != nil {
		return nil, "", "", fmt.Errorf("%s: %w", ErrDownloadFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		loc := strings.TrimSpace(resp.Header.Get("Location"))
		if loc == "" {
			return nil, "", "", fmt.Errorf("%s: redirect without location", ErrDownloadFailed)
		}
		abs, err := resolveRedirect(rawURL, loc)
		if err != nil {
			return nil, "", "", err
		}
		return nil, "", abs, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", "", fmt.Errorf("%s: http %d", ErrDownloadFailed, resp.StatusCode)
	}

	ct := strings.TrimSpace(resp.Header.Get("Content-Type"))
	data, err := io.ReadAll(io.LimitReader(resp.Body, opts.MaxBodyBytes+1))
	if err != nil {
		return nil, "", "", fmt.Errorf("%s: %w", ErrDownloadFailed, err)
	}
	if int64(len(data)) > opts.MaxBodyBytes {
		return nil, "", "", fmt.Errorf("%s: body exceeds %d bytes", ErrResponseTooLarge, opts.MaxBodyBytes)
	}
	return data, ct, "", nil
}

func safeHTTPClient(opts Options) *http.Client {
	return &http.Client{
		Timeout: opts.ResponseTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			// Never delegate target resolution to an environment proxy: doing so
			// would let the proxy resolve a validated hostname to a private address.
			Proxy: nil,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				d := &net.Dialer{Timeout: opts.ConnectTimeout, KeepAlive: 30 * time.Second}
				return dialPublicContext(ctx, network, addr, net.DefaultResolver, d.DialContext)
			},
			TLSHandshakeTimeout: opts.ConnectTimeout,
			MaxIdleConns:        4,
		},
	}
}

func validateURL(ctx context.Context, raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return fmt.Errorf("%s: invalid url", ErrDownloadFailed)
	}
	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("%s: only http/https allowed", ErrSchemeNotAllowed)
	}
	if u.User != nil {
		return fmt.Errorf("%s: credentials in url forbidden", ErrCredentialsInURL)
	}
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return fmt.Errorf("%s: empty host", ErrDownloadFailed)
	}
	if isBlockedHost(host) {
		return fmt.Errorf("%s: host %s blocked", ErrPrivateHost, host)
	}
	if isMetadataHost(host) {
		return fmt.Errorf("%s: metadata endpoint blocked", ErrMetadataEndpoint)
	}
	return assertHostResolvedNotPrivate(ctx, host)
}

func isBlockedHost(host string) bool {
	hl := strings.ToLower(strings.TrimSpace(host))
	if hl == "localhost" || hl == "0.0.0.0" || strings.HasSuffix(hl, ".localhost") {
		return true
	}
	if ip := net.ParseIP(hl); ip != nil {
		return isPrivateIP(ip)
	}
	return false
}

func isMetadataHost(host string) bool {
	hl := strings.ToLower(strings.TrimSpace(host))
	return hl == "metadata.google.internal" ||
		hl == "169.254.169.254" ||
		strings.HasSuffix(hl, ".internal")
}

func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsMulticast() || addr.IsUnspecified() {
		return true
	}
	for _, prefix := range blockedNetworkPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

var blockedNetworkPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001::/32"),
	netip.MustParsePrefix("2001:2::/48"),
	netip.MustParsePrefix("2001:10::/28"),
	netip.MustParsePrefix("2001:20::/28"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"),
}

func assertIPNotPrivate(ip net.IP) error {
	if isPrivateIP(ip) {
		return fmt.Errorf("%s: connection to private address blocked", ErrPrivateIP)
	}
	return nil
}

func assertHostResolvedNotPrivate(ctx context.Context, host string) error {
	if ip := net.ParseIP(host); ip != nil {
		return assertIPNotPrivate(ip)
	}
	resolver := net.Resolver{}
	ips, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("%s: dns lookup failed: %w", ErrDownloadFailed, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("%s: no dns records", ErrDownloadFailed)
	}
	for _, ia := range ips {
		if err := assertIPNotPrivate(ia.IP); err != nil {
			return err
		}
	}
	return nil
}

type ipResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type contextDialer func(context.Context, string, string) (net.Conn, error)

// dialPublicContext resolves once, validates every answer, then dials an exact
// validated IP. This closes the DNS-rebinding gap between validation and the
// socket connection while preserving the original request host for Host/SNI.
func dialPublicContext(ctx context.Context, network, addr string, resolver ipResolver, dial contextDialer) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil || strings.TrimSpace(port) == "" {
		return nil, fmt.Errorf("%s: invalid dial address", ErrDownloadFailed)
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if host == "" {
		return nil, fmt.Errorf("%s: empty dial host", ErrDownloadFailed)
	}
	if resolver == nil || dial == nil {
		return nil, fmt.Errorf("%s: resolver unavailable", ErrDownloadFailed)
	}
	if ip := net.ParseIP(host); ip != nil {
		if err := assertIPNotPrivate(ip); err != nil {
			return nil, err
		}
		return dial(ctx, network, net.JoinHostPort(ip.String(), port))
	}
	addresses, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("%s: dns lookup failed: %w", ErrDownloadFailed, err)
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("%s: no dns records", ErrDownloadFailed)
	}
	for _, candidate := range addresses {
		if err := assertIPNotPrivate(candidate.IP); err != nil {
			return nil, err
		}
	}
	var dialErr error
	for _, candidate := range addresses {
		conn, err := dial(ctx, network, net.JoinHostPort(candidate.IP.String(), port))
		if err == nil {
			return conn, nil
		}
		dialErr = errors.Join(dialErr, err)
	}
	return nil, fmt.Errorf("%s: connect failed: %w", ErrDownloadFailed, dialErr)
}

func resolveRedirect(base, loc string) (string, error) {
	loc = strings.TrimSpace(loc)
	if loc == "" {
		return "", fmt.Errorf("%s: empty redirect", ErrDownloadFailed)
	}
	if strings.HasPrefix(loc, "http://") || strings.HasPrefix(loc, "https://") {
		return loc, nil
	}
	bu, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	lu, err := url.Parse(loc)
	if err != nil {
		return "", err
	}
	return bu.ResolveReference(lu).String(), nil
}

func validateImageBytes(data []byte, contentType string, opts Options) error {
	if len(data) == 0 {
		return fmt.Errorf("%s: empty body", ErrImageDecodeFailed)
	}
	ct := strings.ToLower(strings.Split(strings.TrimSpace(contentType), ";")[0])
	if ct != "" && !strings.HasPrefix(ct, "image/") {
		return fmt.Errorf("%s: content-type %s", ErrInvalidContentType, contentType)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		webpCfg, werr := webp.DecodeConfig(bytes.NewReader(data))
		if werr == nil {
			cfg = webpCfg
		} else {
			return fmt.Errorf("%s: %w", ErrImageDecodeFailed, err)
		}
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return fmt.Errorf("%s: invalid dimensions", ErrImageDecodeFailed)
	}
	if cfg.Width > opts.MaxImageWidth || cfg.Height > opts.MaxImageHeight || int64(cfg.Width)*int64(cfg.Height) > opts.MaxImagePixels {
		return fmt.Errorf("%s: image is %dx%d", ErrImageDimensions, cfg.Width, cfg.Height)
	}
	return nil
}

// ValidateImageBytes applies the same byte, MIME, decode and dimension policy
// used by Download to bytes loaded from trusted storage or data URLs.
func ValidateImageBytes(data []byte, contentType string, opts Options) error {
	defaults := DefaultOptions()
	if opts.MaxBodyBytes <= 0 {
		opts.MaxBodyBytes = defaults.MaxBodyBytes
	}
	if opts.MaxImagePixels <= 0 {
		opts.MaxImagePixels = defaults.MaxImagePixels
	}
	if opts.MaxImageWidth <= 0 {
		opts.MaxImageWidth = defaults.MaxImageWidth
	}
	if opts.MaxImageHeight <= 0 {
		opts.MaxImageHeight = defaults.MaxImageHeight
	}
	if int64(len(data)) > opts.MaxBodyBytes {
		return fmt.Errorf("%s: body exceeds %d bytes", ErrResponseTooLarge, opts.MaxBodyBytes)
	}
	return validateImageBytes(data, contentType, opts)
}

// IsPrivateIP reports whether ip is in a blocked range (exported for tests).
func IsPrivateIP(ip net.IP) bool {
	return isPrivateIP(ip)
}

// ValidateURL checks URL without downloading (exported for tests).
func ValidateURL(ctx context.Context, raw string) error {
	return validateURL(ctx, raw)
}

// ErrCode extracts a stable error code from download errors.
func ErrCode(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	for _, code := range []string{
		ErrSchemeNotAllowed, ErrCredentialsInURL, ErrPrivateHost, ErrPrivateIP,
		ErrMetadataEndpoint, ErrTooManyRedirects, ErrResponseTooLarge,
		ErrInvalidContentType, ErrImageDecodeFailed, ErrImageDimensions, ErrDownloadFailed,
	} {
		if strings.Contains(msg, code) {
			return code
		}
	}
	return ErrDownloadFailed
}

// Wrap preserves code in error chain.
func Wrap(err error, code string) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), code) {
		return err
	}
	return fmt.Errorf("%s: %w", code, err)
}

var errPrivate = errors.New("private")

func init() {
	_ = errPrivate
}
