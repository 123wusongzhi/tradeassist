package ozon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

const maxResponseBytes = 4 << 20
const maxOzonAttempts = 3

// ozonClient is a thin, credential-scoped HTTP client for the Seller API.
// All endpoints are POST + JSON; auth is carried in Client-Id / Api-Key headers.
type ozonClient struct {
	cfg  RuntimeConfig
	http *http.Client
}

func newClient(cfg RuntimeConfig) *ozonClient {
	return &ozonClient{cfg: cfg, http: &http.Client{Timeout: cfg.Timeout}}
}

// postJSON sends one Seller API request and unmarshals the 200 body into out.
// It retries 429 / 5xx / transient transport errors with bounded exponential
// backoff and honors Retry-After (never more than maxOzonAttempts total).
func (c *ozonClient) postJSON(ctx context.Context, path string, body any, out any) error {
	if c == nil || c.http == nil {
		return fmt.Errorf("ozon client not configured")
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("ozon %s: marshal request: %w", path, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+path, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("ozon %s: build request: %w", path, err)
	}
	req.Header.Set("Client-Id", c.cfg.ClientID)
	req.Header.Set("Api-Key", c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	var lastErr error
	for attempt := 1; attempt <= maxOzonAttempts; attempt++ {
		resp, doErr := c.http.Do(req)
		if doErr != nil {
			lastErr = maybeRetryableTransportErr(doErr)
			if attempt >= maxOzonAttempts || !strings.Contains(lastErr.Error(), "retryable") {
				return fmt.Errorf("ozon %s: %w", path, lastErr)
			}
			if !sleepCtx(ctx, backoffDuration(attempt, 0)) {
				return fmt.Errorf("ozon %s: context canceled during retry: %w", path, lastErr)
			}
			continue
		}

		data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
		retryAfter := retryAfterSeconds(resp.Header.Get("Retry-After"))
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("ozon %s: read response: %w", path, readErr)
			if attempt >= maxOzonAttempts {
				return lastErr
			}
			if !sleepCtx(ctx, backoffDuration(attempt, retryAfter)) {
				return fmt.Errorf("ozon %s: context canceled during retry: %w", path, lastErr)
			}
			continue
		}

		if resp.StatusCode == http.StatusOK {
			if out == nil {
				return nil
			}
			if err := json.Unmarshal(data, out); err != nil {
				return fmt.Errorf("ozon %s: invalid json response: %w", path, err)
			}
			return nil
		}

		lastErr = classifyOzonError(resp.StatusCode, data)
		if attempt >= maxOzonAttempts || !strings.Contains(lastErr.Error(), "retryable") {
			return lastErr
		}
		if !sleepCtx(ctx, backoffDuration(attempt, retryAfter)) {
			return fmt.Errorf("ozon %s: context canceled during retry: %w", path, lastErr)
		}
	}
	return lastErr
}

func retryAfterSeconds(raw string) time.Duration {
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n <= 0 {
		return 0
	}
	d := time.Duration(n) * time.Second
	if d > 15*time.Second {
		return 15 * time.Second
	}
	return d
}

func backoffDuration(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		return retryAfter
	}
	d := time.Duration(300*(1<<(attempt-1))) * time.Millisecond
	if d > 4*time.Second {
		return 4 * time.Second
	}
	return d
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func parseOzonErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var env struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return strings.TrimSpace(string(body))
	}
	msg := strings.TrimSpace(env.Message)
	if msg == "" {
		return ""
	}
	if env.Code != 0 {
		return fmt.Sprintf("code=%d %s", env.Code, msg)
	}
	return msg
}

func classifyOzonError(status int, body []byte) error {
	msg := parseOzonErrorMessage(body)
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("%w: ozon http %d %s", platformp.ErrPlatformProductPublishPermissionDenied, status, msg)
	case http.StatusTooManyRequests:
		return fmt.Errorf("ozon: retryable: rate limited (http 429) %s", msg)
	default:
		if status >= 500 {
			return fmt.Errorf("ozon: retryable: upstream error (http %d) %s", status, msg)
		}
		if msg != "" {
			return fmt.Errorf("ozon: request failed (http %d): %s", status, msg)
		}
		return fmt.Errorf("ozon: request failed (http %d)", status)
	}
}

func maybeRetryableTransportErr(err error) error {
	if err == nil {
		return nil
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "timeout") || strings.Contains(s, "timed out") ||
		strings.Contains(s, "connection reset") || strings.Contains(s, "eof") ||
		strings.Contains(s, "connection refused") || strings.Contains(s, "no such host") {
		return fmt.Errorf("ozon: retryable transport error: %w", err)
	}
	return err
}
