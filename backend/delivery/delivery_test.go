package delivery

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/smtp"
	"strings"
	"testing"
)

func TestNotifierChannelsLinkAndRouting(t *testing.T) {
	notifier := &Notifier{
		baseURL:  "https://wedding.example/",
		path:     defaultInvitationPath,
		email:    NewSMTPSender(SMTPConfig{Host: "smtp.example", Port: "587", From: "couple@example.com"}),
		whatsapp: NewWhatsAppSender(WhatsAppConfig{Token: "token", PhoneNumberID: "123"}),
	}

	channels := notifier.Channels()
	if len(channels) != 2 || channels[0] != ChannelEmail || channels[1] != ChannelWhatsApp {
		t.Fatalf("channels = %#v", channels)
	}
	if link := notifier.Link("abc def"); link != "https://wedding.example/pages/event.html?token=abc+def" {
		t.Fatalf("link = %q", link)
	}
	if link := (&Notifier{}).Link("abc"); link != "" {
		t.Fatalf("link without base url = %q", link)
	}
	if err := notifier.Send(context.Background(), Invitation{Channel: "carrier-pigeon"}); err == nil {
		t.Fatal("expected an unknown channel error")
	}
	if err := (&Notifier{}).Send(context.Background(), Invitation{Channel: ChannelEmail}); err != ErrChannelUnavailable {
		t.Fatalf("unconfigured email error = %v", err)
	}
}

func TestSMTPSenderRejectsHeaderInjectionAndRendersMessage(t *testing.T) {
	var (
		gotAddr string
		gotFrom string
		gotTo   []string
		gotMsg  string
	)
	sender := NewSMTPSender(SMTPConfig{Host: "smtp.example", Port: "2525", From: "couple@example.com"})
	sender.send = func(addr string, _ smtp.Auth, from string, to []string, msg []byte) error {
		gotAddr, gotFrom, gotTo, gotMsg = addr, from, to, string(msg)
		return nil
	}

	invitation := Invitation{Channel: ChannelEmail, To: "guest@example.com", Name: "Taylor", Couple: "Alex & Sam", Link: "https://wedding.example/pages/event.html?token=t"}
	if err := sender.Send(context.Background(), invitation); err != nil {
		t.Fatal(err)
	}
	if gotAddr != "smtp.example:2525" || gotFrom != "couple@example.com" || len(gotTo) != 1 || gotTo[0] != "guest@example.com" {
		t.Fatalf("unexpected envelope: addr=%q from=%q to=%#v", gotAddr, gotFrom, gotTo)
	}
	for _, want := range []string{"To: guest@example.com", "Subject: ", "text/plain", "Dear Taylor", "the wedding of Alex & Sam", invitation.Link} {
		if !strings.Contains(gotMsg, want) {
			t.Fatalf("message missing %q:\n%s", want, gotMsg)
		}
	}
	if strings.Contains(gotMsg, "\n") && !strings.Contains(gotMsg, "\r\n") {
		t.Fatalf("message does not use CRLF line endings:\n%q", gotMsg)
	}

	injected := Invitation{Channel: ChannelEmail, To: "guest@example.com\r\nBcc: attacker@example.com", Couple: "Alex & Sam"}
	if err := sender.Send(context.Background(), injected); err == nil {
		t.Fatal("expected a header injection attempt to be rejected")
	}
}

func TestWhatsAppSenderPostsToCloudAPI(t *testing.T) {
	type received struct {
		path   string
		auth   string
		body   map[string]any
		status int
	}
	got := make(chan received, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		got <- received{path: r.URL.Path, auth: r.Header.Get("Authorization"), body: body, status: http.StatusOK}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"messages":[{"id":"wamid.1"}]}`))
	}))
	defer server.Close()

	sender := NewWhatsAppSender(WhatsAppConfig{Token: "secret", PhoneNumberID: "999", BaseURL: server.URL})
	invitation := Invitation{Channel: ChannelWhatsApp, To: "+1 (555) 123-4567", Name: "Taylor", Couple: "Alex & Sam", Link: "https://wedding.example/pages/event.html?token=t"}
	if err := sender.Send(context.Background(), invitation); err != nil {
		t.Fatal(err)
	}

	request := <-got
	if request.path != "/"+defaultWhatsAppAPI+"/999/messages" {
		t.Fatalf("path = %q", request.path)
	}
	if request.auth != "Bearer secret" {
		t.Fatalf("authorization = %q", request.auth)
	}
	if request.body["messaging_product"] != "whatsapp" || request.body["to"] != "15551234567" {
		t.Fatalf("payload = %#v", request.body)
	}
	text, _ := request.body["text"].(map[string]any)
	if body, _ := text["body"].(string); !strings.Contains(body, invitation.Link) {
		t.Fatalf("message body missing link: %#v", request.body)
	}
}

func TestWhatsAppSenderReportsProviderErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad token"}}`))
	}))
	defer server.Close()

	sender := NewWhatsAppSender(WhatsAppConfig{Token: "secret", PhoneNumberID: "999", BaseURL: server.URL})
	err := sender.Send(context.Background(), Invitation{Channel: ChannelWhatsApp, To: "15551234567", Link: "https://x/y"})
	if err == nil || !strings.Contains(err.Error(), "bad token") {
		t.Fatalf("error = %v", err)
	}
}

func TestFromEnvActivatesOnlyConfiguredChannels(t *testing.T) {
	t.Setenv("WEDDINGHUB_PUBLIC_BASE_URL", "https://wedding.example")
	t.Setenv("WEDDINGHUB_SMTP_HOST", "smtp.example")
	t.Setenv("WEDDINGHUB_EMAIL_FROM", "couple@example.com")
	t.Setenv("WEDDINGHUB_WHATSAPP_TOKEN", "")
	t.Setenv("WEDDINGHUB_WHATSAPP_PHONE_NUMBER_ID", "")

	notifier := FromEnv()
	channels := notifier.Channels()
	if len(channels) != 1 || channels[0] != ChannelEmail {
		t.Fatalf("channels = %#v", channels)
	}
	if link := notifier.Link("tok"); link != "https://wedding.example/pages/event.html?token=tok" {
		t.Fatalf("link = %q", link)
	}

	t.Setenv("WEDDINGHUB_WHATSAPP_TOKEN", "token")
	t.Setenv("WEDDINGHUB_WHATSAPP_PHONE_NUMBER_ID", "999")
	t.Setenv("WEDDINGHUB_INVITATION_PATH", "invite")
	if channels := FromEnv().Channels(); len(channels) != 2 {
		t.Fatalf("channels with whatsapp = %#v", channels)
	}
	if link := FromEnv().Link("tok"); link != "https://wedding.example/invite?token=tok" {
		t.Fatalf("custom path link = %q", link)
	}
}
