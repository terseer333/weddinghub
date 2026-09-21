// Package delivery sends wedding invitations over opt-in delivery channels.
//
// Email uses SMTP and WhatsApp uses the WhatsApp Cloud API. A channel only
// becomes active when its environment configuration is present, so the API can
// keep running the in-memory demo without any provider credentials.
package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultInvitationPath = "/pages/event.html"
	defaultSMTPPort       = "587"
	defaultWhatsAppAPI    = "v21.0"
	defaultWhatsAppBase   = "https://graph.facebook.com"
	whatsAppTimeout       = 10 * time.Second
)

// Channel identifiers used by requests and responses.
const (
	ChannelEmail    = "email"
	ChannelWhatsApp = "whatsapp"
)

// ErrChannelUnavailable reports that a requested channel is not configured.
var ErrChannelUnavailable = errors.New("delivery channel is not configured")

// Invitation is one personalized invitation message for one recipient.
type Invitation struct {
	Channel string
	To      string
	Name    string
	Couple  string
	Link    string
}

// Sender routes invitation messages to the configured delivery channels.
type Sender interface {
	// Channels lists the active channels, for example ["email", "whatsapp"].
	Channels() []string
	// Link builds the personal invitation URL for a raw token, or "" when no
	// public base URL is configured.
	Link(token string) string
	// Send delivers one invitation over invitation.Channel.
	Send(ctx context.Context, invitation Invitation) error
}

// Notifier is the environment-configured Sender.
type Notifier struct {
	baseURL  string
	path     string
	email    *SMTPSender
	whatsapp *WhatsAppSender
}

// FromEnv builds a Notifier from environment configuration. A channel whose
// configuration is incomplete stays inactive.
func FromEnv() *Notifier {
	notifier := &Notifier{
		baseURL: strings.TrimSpace(os.Getenv("WEDDINGHUB_PUBLIC_BASE_URL")),
		path:    invitationPath(),
	}
	if cfg, ok := smtpFromEnv(); ok {
		notifier.email = NewSMTPSender(cfg)
	}
	if cfg, ok := whatsappFromEnv(); ok {
		notifier.whatsapp = NewWhatsAppSender(cfg)
	}
	return notifier
}

func invitationPath() string {
	path := strings.TrimSpace(os.Getenv("WEDDINGHUB_INVITATION_PATH"))
	if path == "" {
		return defaultInvitationPath
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

// Channels returns the active channels in a stable order.
func (n *Notifier) Channels() []string {
	if n == nil {
		return nil
	}
	channels := make([]string, 0, 2)
	if n.email != nil {
		channels = append(channels, ChannelEmail)
	}
	if n.whatsapp != nil {
		channels = append(channels, ChannelWhatsApp)
	}
	return channels
}

// Link builds the personal invitation URL for a raw token. It returns "" when no
// public base URL is configured, because a loopback or relative link is useless
// to a recipient.
func (n *Notifier) Link(token string) string {
	if n == nil || n.baseURL == "" || strings.TrimSpace(token) == "" {
		return ""
	}
	return strings.TrimRight(n.baseURL, "/") + n.path + "?token=" + url.QueryEscape(token)
}

// Send delivers one invitation over the requested channel.
func (n *Notifier) Send(ctx context.Context, invitation Invitation) error {
	if n == nil {
		return ErrChannelUnavailable
	}
	switch invitation.Channel {
	case ChannelEmail:
		if n.email == nil {
			return ErrChannelUnavailable
		}
		return n.email.Send(ctx, invitation)
	case ChannelWhatsApp:
		if n.whatsapp == nil {
			return ErrChannelUnavailable
		}
		return n.whatsapp.Send(ctx, invitation)
	default:
		return fmt.Errorf("unknown delivery channel %q", invitation.Channel)
	}
}

// SMTPConfig configures the email channel.
type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

// SMTPSender delivers invitations over SMTP using STARTTLS when the server offers it.
type SMTPSender struct {
	cfg SMTPConfig
	// send is overridable in tests.
	send func(addr string, auth smtp.Auth, from string, to []string, msg []byte) error
}

func NewSMTPSender(cfg SMTPConfig) *SMTPSender {
	if cfg.Port == "" {
		cfg.Port = defaultSMTPPort
	}
	return &SMTPSender{cfg: cfg}
}

func (s *SMTPSender) Send(ctx context.Context, invitation Invitation) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	from, err := mail.ParseAddress(s.cfg.From)
	if err != nil {
		return fmt.Errorf("invalid sender address: %w", err)
	}
	// ParseAddress rejects the header-injection characters a raw string would allow,
	// so an attacker-controlled guest address cannot add extra headers.
	to, err := mail.ParseAddress(invitation.To)
	if err != nil {
		return fmt.Errorf("invalid recipient address: %w", err)
	}
	subject, body := invitationText(invitation)
	message := buildEmail(from.Address, to.Address, subject, body)

	send := s.send
	if send == nil {
		send = smtp.SendMail
	}
	var auth smtp.Auth
	if s.cfg.Username != "" {
		auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}
	return send(net.JoinHostPort(s.cfg.Host, s.cfg.Port), auth, from.Address, []string{to.Address}, message)
}

// buildEmail renders a UTF-8 plain-text message with CRLF line endings.
func buildEmail(from, to, subject, body string) []byte {
	var message bytes.Buffer
	fmt.Fprintf(&message, "From: %s\r\n", from)
	fmt.Fprintf(&message, "To: %s\r\n", to)
	fmt.Fprintf(&message, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	message.WriteString("MIME-Version: 1.0\r\n")
	message.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	message.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	message.WriteString(strings.ReplaceAll(body, "\n", "\r\n"))
	return message.Bytes()
}

// WhatsAppConfig configures the WhatsApp Cloud API channel.
type WhatsAppConfig struct {
	Token         string
	PhoneNumberID string
	APIVersion    string
	BaseURL       string
}

// WhatsAppSender delivers invitations through the WhatsApp Cloud API.
type WhatsAppSender struct {
	cfg    WhatsAppConfig
	client *http.Client
}

func NewWhatsAppSender(cfg WhatsAppConfig) *WhatsAppSender {
	if cfg.APIVersion == "" {
		cfg.APIVersion = defaultWhatsAppAPI
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultWhatsAppBase
	}
	return &WhatsAppSender{cfg: cfg, client: &http.Client{Timeout: whatsAppTimeout}}
}

func (s *WhatsAppSender) Send(ctx context.Context, invitation Invitation) error {
	to := normalizePhone(invitation.To)
	if to == "" {
		return errors.New("whatsapp recipient number is empty")
	}
	_, body := invitationText(invitation)
	payload, err := json.Marshal(map[string]any{
		"messaging_product": "whatsapp",
		"to":                to,
		"type":              "text",
		"text":              map[string]string{"body": body},
	})
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/%s/%s/messages", strings.TrimRight(s.cfg.BaseURL, "/"), s.cfg.APIVersion, url.PathEscape(s.cfg.PhoneNumberID))
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+s.cfg.Token)
	request.Header.Set("Content-Type", "application/json")

	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return fmt.Errorf("whatsapp delivery failed (%d): %s", response.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return nil
}

// normalizePhone keeps only digits so the Cloud API receives a country-coded number.
func normalizePhone(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

// invitationText renders the shared invitation copy for every channel.
func invitationText(invitation Invitation) (subject, body string) {
	couple := strings.TrimSpace(invitation.Couple)
	if couple == "" {
		couple = "the couple"
	}
	greeting := "Hello"
	if name := strings.TrimSpace(invitation.Name); name != "" {
		greeting = "Dear " + name
	}
	subject = fmt.Sprintf("You're invited to the wedding of %s", couple)
	body = fmt.Sprintf("%s,\n\nYou're warmly invited to celebrate the wedding of %s.\n\nOpen your personal invitation and accept it here:\n%s\n\nWith love.",
		greeting, couple, invitation.Link)
	return subject, body
}

func smtpFromEnv() (SMTPConfig, bool) {
	host := strings.TrimSpace(os.Getenv("WEDDINGHUB_SMTP_HOST"))
	username := strings.TrimSpace(os.Getenv("WEDDINGHUB_SMTP_USERNAME"))
	from := strings.TrimSpace(os.Getenv("WEDDINGHUB_EMAIL_FROM"))
	if from == "" {
		from = username
	}
	if host == "" || from == "" {
		return SMTPConfig{}, false
	}
	port := strings.TrimSpace(os.Getenv("WEDDINGHUB_SMTP_PORT"))
	if port == "" {
		port = defaultSMTPPort
	}
	return SMTPConfig{Host: host, Port: port, Username: username, Password: os.Getenv("WEDDINGHUB_SMTP_PASSWORD"), From: from}, true
}

func whatsappFromEnv() (WhatsAppConfig, bool) {
	token := strings.TrimSpace(os.Getenv("WEDDINGHUB_WHATSAPP_TOKEN"))
	phoneNumberID := strings.TrimSpace(os.Getenv("WEDDINGHUB_WHATSAPP_PHONE_NUMBER_ID"))
	if token == "" || phoneNumberID == "" {
		return WhatsAppConfig{}, false
	}
	return WhatsAppConfig{
		Token:         token,
		PhoneNumberID: phoneNumberID,
		APIVersion:    strings.TrimSpace(os.Getenv("WEDDINGHUB_WHATSAPP_API_VERSION")),
		BaseURL:       strings.TrimSpace(os.Getenv("WEDDINGHUB_WHATSAPP_API_BASE")),
	}, true
}
