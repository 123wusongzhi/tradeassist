package smtp

import (
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/providers/email"
)

type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	FromName string
	From     string
	UseTLS   bool
	UseSSL   bool
}

type Provider struct {
	cfg Config
}

func NewProvider(cfg Config) *Provider {
	return &Provider{cfg: cfg}
}

func (p *Provider) Name() string {
	return "smtp"
}

func (p *Provider) Send(ctx context.Context, req email.SendEmailRequest) error {
	if p == nil {
		return fmt.Errorf("smtp provider is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	host := strings.TrimSpace(p.cfg.Host)
	if host == "" || containsHeaderBreak(host) {
		return fmt.Errorf("smtp host is invalid")
	}
	if p.cfg.Port < 1 || p.cfg.Port > 65535 {
		return fmt.Errorf("smtp port is invalid")
	}
	from, to, message, err := buildMessage(p.cfg, req)
	if err != nil {
		return err
	}

	addr := net.JoinHostPort(host, strconv.Itoa(p.cfg.Port))
	conn, err := dialSMTP(ctx, addr, host, p.cfg.UseSSL)
	if err != nil {
		return fmt.Errorf("smtp connect: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp new client: %w", err)
	}
	defer client.Close()

	if !p.cfg.UseSSL {
		supportsSTARTTLS, _ := client.Extension("STARTTLS")
		if p.cfg.UseTLS && !supportsSTARTTLS {
			return fmt.Errorf("smtp STARTTLS is required but not supported")
		}
		// Preserve net/smtp.SendMail's opportunistic upgrade while making the
		// explicit STARTTLS setting fail closed when the server cannot upgrade.
		if supportsSTARTTLS {
			if err := client.StartTLS(smtpTLSConfig(host)); err != nil {
				return fmt.Errorf("smtp STARTTLS: %w", err)
			}
		}
	}

	if p.cfg.Username != "" || p.cfg.Password != "" {
		if p.cfg.Username == "" || p.cfg.Password == "" {
			return fmt.Errorf("smtp credentials are incomplete")
		}
		auth := smtp.PlainAuth("", p.cfg.Username, p.cfg.Password, host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(message); err != nil {
		_ = w.Close()
		return fmt.Errorf("smtp write data: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("smtp quit: %w", err)
	}
	return nil
}

func dialSMTP(ctx context.Context, addr, host string, implicitTLS bool) (net.Conn, error) {
	dialer := &net.Dialer{}
	if implicitTLS {
		return (&tls.Dialer{NetDialer: dialer, Config: smtpTLSConfig(host)}).DialContext(ctx, "tcp", addr)
	}
	return dialer.DialContext(ctx, "tcp", addr)
}

func smtpTLSConfig(host string) *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: strings.TrimSpace(host),
	}
}

func buildMessage(cfg Config, req email.SendEmailRequest) (string, string, []byte, error) {
	fromAddr, err := mail.ParseAddress(strings.TrimSpace(cfg.From))
	if err != nil || fromAddr.Address == "" {
		return "", "", nil, fmt.Errorf("smtp from address is invalid")
	}
	toAddr, err := mail.ParseAddress(strings.TrimSpace(req.To))
	if err != nil || toAddr.Address == "" {
		return "", "", nil, fmt.Errorf("smtp recipient address is invalid")
	}
	for _, value := range []string{cfg.FromName, req.Subject, fromAddr.Address, toAddr.Address} {
		if containsHeaderBreak(value) {
			return "", "", nil, fmt.Errorf("smtp message header is invalid")
		}
	}

	contentType := strings.TrimSpace(req.ContentType)
	if contentType == "" {
		contentType = "text/plain"
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || (mediaType != "text/plain" && mediaType != "text/html") {
		return "", "", nil, fmt.Errorf("smtp content type is invalid")
	}

	fromHeader := (&mail.Address{Name: strings.TrimSpace(cfg.FromName), Address: fromAddr.Address}).String()
	toHeader := (&mail.Address{Name: toAddr.Name, Address: toAddr.Address}).String()
	subject := mime.QEncoding.Encode("UTF-8", strings.TrimSpace(req.Subject))
	message := strings.Join([]string{
		"From: " + fromHeader,
		"To: " + toHeader,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: " + mediaType + `; charset="utf-8"`,
		"Content-Transfer-Encoding: 8bit",
		"",
		req.Content,
	}, "\r\n")
	return fromAddr.Address, toAddr.Address, []byte(message), nil
}

func containsHeaderBreak(value string) bool {
	return strings.ContainsAny(value, "\r\n\x00")
}
