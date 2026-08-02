package smtp

import (
	"crypto/tls"
	"strings"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/providers/email"
)

func TestSMTPTLSConfigVerifiesPeer(t *testing.T) {
	cfg := smtpTLSConfig("smtp.example.com")
	if cfg.InsecureSkipVerify {
		t.Fatal("SMTP TLS must verify the server certificate")
	}
	if cfg.ServerName != "smtp.example.com" {
		t.Fatalf("unexpected server name %q", cfg.ServerName)
	}
	if cfg.MinVersion < tls.VersionTLS12 {
		t.Fatalf("SMTP TLS minimum version is too old: %d", cfg.MinVersion)
	}
}

func TestBuildMessageRejectsHeaderInjection(t *testing.T) {
	base := Config{From: "sender@example.com", FromName: "TradeMind"}
	tests := []struct {
		name string
		cfg  Config
		req  email.SendEmailRequest
	}{
		{name: "subject", cfg: base, req: email.SendEmailRequest{To: "to@example.com", Subject: "ok\r\nBcc: victim@example.com"}},
		{name: "from name", cfg: Config{From: base.From, FromName: "TradeMind\nBcc: victim@example.com"}, req: email.SendEmailRequest{To: "to@example.com", Subject: "ok"}},
		{name: "recipient", cfg: base, req: email.SendEmailRequest{To: "to@example.com\r\nBcc: victim@example.com", Subject: "ok"}},
		{name: "content type", cfg: base, req: email.SendEmailRequest{To: "to@example.com", Subject: "ok", ContentType: "text/plain\r\nBcc: victim@example.com"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, _, err := buildMessage(tc.cfg, tc.req); err == nil {
				t.Fatal("malicious SMTP header was accepted")
			}
		})
	}
}

func TestBuildMessageUsesParsedEnvelopeAddresses(t *testing.T) {
	from, to, message, err := buildMessage(Config{From: "sender@example.com", FromName: "贸灵"}, email.SendEmailRequest{
		To:          "Recipient <to@example.com>",
		Subject:     "安全通知",
		Content:     "hello",
		ContentType: "text/html; charset=utf-8",
	})
	if err != nil {
		t.Fatal(err)
	}
	if from != "sender@example.com" || to != "to@example.com" {
		t.Fatalf("unexpected envelope from=%q to=%q", from, to)
	}
	text := string(message)
	if !strings.Contains(text, "Content-Type: text/html; charset=\"utf-8\"") || strings.Contains(text, "Bcc:") {
		t.Fatalf("unexpected message headers: %s", text)
	}
}
