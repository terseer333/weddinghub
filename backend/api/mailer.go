package api

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"
)

// Mailer sends verification codes over SMTP using credentials from the environment:
// WEDDINGHUB_SMTP_HOST, _PORT, _USERNAME, _PASSWORD, _FROM. When no host is
// configured the mailer reports itself as unavailable and code delivery fails loudly
// rather than silently falling back to a console log.
type Mailer struct {
	host       string
	port       int
	username   string
	password   string
	from       string
	configured bool
}

// codeSender is the seam the API depends on; tests inject a fake implementation.
type codeSender interface {
	SendCode(ctx context.Context, to, code string) error
}

func NewMailerFromEnv() *Mailer {
	host := strings.TrimSpace(os.Getenv("WEDDINGHUB_SMTP_HOST"))
	port := 587
	if raw := strings.TrimSpace(os.Getenv("WEDDINGHUB_SMTP_PORT")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			port = parsed
		}
	}
	from := strings.TrimSpace(os.Getenv("WEDDINGHUB_SMTP_FROM"))
	if from == "" {
		from = strings.TrimSpace(os.Getenv("WEDDINGHUB_SMTP_USERNAME"))
	}
	return &Mailer{
		host:       host,
		port:       port,
		username:   strings.TrimSpace(os.Getenv("WEDDINGHUB_SMTP_USERNAME")),
		password:   os.Getenv("WEDDINGHUB_SMTP_PASSWORD"),
		from:       from,
		configured: host != "" && from != "",
	}
}

// SendCode delivers a login verification code. The plain-text body is intentional:
// no HTML is needed for a single six-digit value, and it stays readable everywhere.
func (m *Mailer) SendCode(ctx context.Context, to, code string) error {
	if !m.configured {
		return errors.New("email delivery is not configured (WEDDINGHUB_SMTP_HOST and WEDDINGHUB_SMTP_FROM are required)")
	}
	subject := "Your WeddingHub verification code"
	body := fmt.Sprintf("Your WeddingHub verification code is: %s\n\nThis code expires in 10 minutes. If you did not request it, you can ignore this email.\n", code)
	msg := strings.Join([]string{
		"From: " + m.from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"",
		body,
	}, "\r\n")

	addr := m.host + ":" + strconv.Itoa(m.port)
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("connect to mail server: %w", err)
	}
	// Bound the whole exchange, not just the dial: a stalling mail server must
	// not hold the login request open indefinitely.
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	client, err := smtp.NewClient(conn, m.host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("greet mail server: %w", err)
	}
	defer client.Close()

	// Opportunistic TLS: STARTTLS when offered, plain otherwise.
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: m.host}); err != nil {
			return fmt.Errorf("start TLS: %w", err)
		}
	}
	if m.username != "" {
		if err := client.Auth(smtp.PlainAuth("", m.username, m.password, m.host)); err != nil {
			return fmt.Errorf("authenticate with mail server: %w", err)
		}
	}
	if err := client.Mail(m.from); err != nil {
		return fmt.Errorf("set sender: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("set recipient: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("open message body: %w", err)
	}
	if _, err := writer.Write([]byte(msg)); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	return client.Quit()
}
