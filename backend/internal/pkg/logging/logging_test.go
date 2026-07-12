package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

const (
	testAccessToken  = "TEST_ACCESS_TOKEN_UNIQUE"
	testRefreshToken = "TEST_REFRESH_TOKEN_UNIQUE"
	testAppSecret    = "TEST_APP_SECRET_UNIQUE"
	testPhone        = "TEST_PHONE_UNIQUE"
	testEmail        = "TEST_EMAIL_UNIQUE"
)

func TestSanitizeLogFieldsRedactsSecrets(t *testing.T) {
	fields := map[string]any{
		"access_token": testAccessToken,
		"nested": map[string]any{
			"password": "secret123",
			"phone":    testPhone,
		},
		"items": []any{map[string]any{"email": testEmail}},
	}
	out := SanitizeLogFields(fields)
	raw, _ := json.Marshal(out)
	s := string(raw)
	for _, secret := range []string{testAccessToken, testPhone, testEmail, "secret123"} {
		if strings.Contains(s, secret) {
			t.Fatalf("secret leaked: %s in %s", secret, s)
		}
	}
}

func TestStructuredLoggerJSON(t *testing.T) {
	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	base := slog.New(h)
	l := &structuredLogger{
		base: base,
		cfg: Config{
			Format:         "json",
			Level:          "info",
			MaxFieldLength: 512,
			Service:        "test",
			Environment:    "test",
			FailSafe:       true,
		},
	}
	ctx := WithRequestID(context.Background(), "req-1")
	l.Info(ctx, "hello", F("result", "success"), F("authorization", testAccessToken))
	if !strings.Contains(buf.String(), "hello") {
		t.Fatalf("missing message: %s", buf.String())
	}
	if strings.Contains(buf.String(), testAccessToken) {
		t.Fatalf("token leaked in log: %s", buf.String())
	}
}

func TestTruncateString(t *testing.T) {
	if got := TruncateString("abcdef", 3); got != "abc..." {
		t.Fatalf("truncate got %q", got)
	}
}
