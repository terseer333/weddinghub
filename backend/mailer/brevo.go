// Package mailer sends account recovery messages through a replaceable provider interface.
package mailer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"strings"
	"time"
)

const brevoEndpoint = "https://api.brevo.com/v3/smtp/email"

type PasswordMailer interface {
	SendPasswordReset(context.Context, string, string, string) error
	SendPasswordChanged(context.Context, string, string) error
}

type Brevo struct {
	apiKey   string
	from     string
	fromName string
	baseURL  string
	client   *http.Client
	endpoint string
}

func FromEnv() (*Brevo, error) {
	keys := []string{"BREVO_API_KEY", "EMAIL_FROM", "APP_BASE_URL"}
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		values[key] = strings.TrimSpace(os.Getenv(key))
	}
	missing := make([]string, 0, len(values))
	for _, key := range keys {
		if values[key] == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing %s", strings.Join(missing, ", "))
	}
	name := strings.TrimSpace(os.Getenv("EMAIL_FROM_NAME"))
	if name == "" {
		name = "WeddingHub"
	}
	return New(values["BREVO_API_KEY"], values["EMAIL_FROM"], name, values["APP_BASE_URL"])
}

func New(apiKey, from, fromName, baseURL string) (*Brevo, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("Brevo API key is required")
	}
	parsedEmail, err := mail.ParseAddress(strings.TrimSpace(from))
	if err != nil || parsedEmail.Address != strings.TrimSpace(from) {
		return nil, errors.New("EMAIL_FROM must be a valid email address")
	}
	base, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || (base.Scheme != "https" && base.Scheme != "http") || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return nil, errors.New("APP_BASE_URL must be an absolute HTTP(S) URL")
	}
	if base.Scheme != "https" && !isLoopbackHost(base.Hostname()) {
		return nil, errors.New("APP_BASE_URL must use HTTPS except for local development")
	}
	if strings.TrimSpace(fromName) == "" {
		fromName = "WeddingHub"
	}
	return &Brevo{
		apiKey: apiKey, from: parsedEmail.Address, fromName: fromName, baseURL: strings.TrimRight(base.String(), "/"),
		client: &http.Client{Timeout: 10 * time.Second}, endpoint: brevoEndpoint,
	}, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	parsed := net.ParseIP(host)
	return parsed != nil && parsed.IsLoopback()
}

func (b *Brevo) SendPasswordReset(ctx context.Context, to, name, token string) error {
	resetURL, err := url.Parse(b.baseURL + "/reset-password")
	if err != nil {
		return errors.New("could not build reset URL")
	}
	query := resetURL.Query()
	query.Set("token", token)
	resetURL.RawQuery = query.Encode()
	greeting := "Hello"
	if strings.TrimSpace(name) != "" {
		greeting = "Hello " + html.EscapeString(name)
	}
	link := html.EscapeString(resetURL.String())
	body := fmt.Sprintf(`<div style="max-width:560px;margin:auto;padding:32px;font-family:Arial,sans-serif;color:#26362f"><h1 style="font-family:Georgia,serif;color:#315647">WeddingHub</h1><h2>Password reset requested</h2><p>%s,</p><p>We received a request to reset your WeddingHub password. Use the secure link below to choose a new password.</p><p style="margin:28px 0"><a href="%s" style="padding:13px 22px;border-radius:8px;background:#315647;color:#fff;text-decoration:none">Reset password</a></p><p>This link expires in 30 minutes and can only be used once.</p><p>If you did not request this, ignore this email. Your password will not change.</p></div>`, greeting, link)
	return b.send(ctx, to, "Reset your WeddingHub password", body)
}

func (b *Brevo) SendPasswordChanged(ctx context.Context, to, name string) error {
	greeting := "Hello"
	if strings.TrimSpace(name) != "" {
		greeting = "Hello " + html.EscapeString(name)
	}
	body := fmt.Sprintf(`<div style="max-width:560px;margin:auto;padding:32px;font-family:Arial,sans-serif;color:#26362f"><h1 style="font-family:Georgia,serif;color:#315647">WeddingHub</h1><h2>Password changed</h2><p>%s,</p><p>Your WeddingHub password was changed successfully.</p><p>If you did not make this change, contact WeddingHub support right away.</p></div>`, greeting)
	return b.send(ctx, to, "Your WeddingHub password was changed", body)
}

func (b *Brevo) send(ctx context.Context, to, subject, body string) error {
	parsed, err := mail.ParseAddress(strings.TrimSpace(to))
	if err != nil || parsed.Address != strings.TrimSpace(to) {
		return errors.New("recipient email is invalid")
	}
	payload := struct {
		Sender struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"sender"`
		To []struct {
			Email string `json:"email"`
		} `json:"to"`
		Subject string `json:"subject"`
		HTML    string `json:"htmlContent"`
	}{}
	payload.Sender.Name, payload.Sender.Email = b.fromName, b.from
	payload.To = append(payload.To, struct {
		Email string `json:"email"`
	}{Email: parsed.Address})
	payload.Subject, payload.HTML = subject, body
	encoded, err := json.Marshal(payload)
	if err != nil {
		return errors.New("could not encode email")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.endpoint, bytes.NewReader(encoded))
	if err != nil {
		return errors.New("could not create email request")
	}
	req.Header.Set("api-key", b.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.client.Do(req)
	if err != nil {
		log.Printf("Brevo email request failed (network error)")
		return errors.New("email provider request failed")
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("Brevo email request failed with HTTP status %d", resp.StatusCode)
		return fmt.Errorf("email provider returned HTTP %d", resp.StatusCode)
	}
	return nil
}
