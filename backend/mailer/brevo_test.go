package mailer

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBrevoSendsBrandedResetEmail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v3/smtp/email" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}
		if got := r.Header.Get("api-key"); got != "test-secret-key" {
			t.Errorf("api-key = %q", got)
		}
		var body struct {
			Sender struct {
				Name  string `json:"name"`
				Email string `json:"email"`
			} `json:"sender"`
			To []struct {
				Email string `json:"email"`
			} `json:"to"`
			Subject string `json:"subject"`
			HTML    string `json:"htmlContent"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Sender.Email != "sender@example.com" || body.Sender.Name != "WeddingHub" || len(body.To) != 1 || body.To[0].Email != "ada@example.com" {
			t.Errorf("unexpected sender/recipient: %#v", body)
		}
		if body.Subject != "Reset your WeddingHub password" || !strings.Contains(body.HTML, "https://wedding.example/reset-password?token=secure-token") || !strings.Contains(body.HTML, "30 minutes") || !strings.Contains(body.HTML, "&lt;Ada&gt;") {
			t.Errorf("reset email content missing or unsafe: %#v", body)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	provider, err := New("test-secret-key", "sender@example.com", "WeddingHub", "https://wedding.example")
	if err != nil {
		t.Fatal(err)
	}
	provider.endpoint = server.URL + "/v3/smtp/email"
	if err := provider.SendPasswordReset(context.Background(), "ada@example.com", "<Ada>", "secure-token"); err != nil {
		t.Fatal(err)
	}
}

func TestBrevoFailureIsSanitizedAndLogsStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"message":"secret provider response"}`)
	}))
	defer server.Close()
	provider, err := New("test-secret-key", "sender@example.com", "WeddingHub", "https://wedding.example")
	if err != nil {
		t.Fatal(err)
	}
	provider.endpoint = server.URL
	var logs strings.Builder
	previous := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previous) })
	err = provider.SendPasswordChanged(context.Background(), "ada@example.com", "Ada")
	if err == nil || strings.Contains(err.Error(), "secret provider response") || strings.Contains(err.Error(), "test-secret-key") {
		t.Fatalf("unsanitized provider error: %v", err)
	}
	if strings.Contains(logs.String(), "secret provider response") || strings.Contains(logs.String(), "test-secret-key") || !strings.Contains(logs.String(), "401") {
		t.Fatalf("unsafe or incomplete provider log: %q", logs.String())
	}
}

func TestBrevoConfigValidatesSenderAndBaseURL(t *testing.T) {
	if _, err := New("key", "not an email", "WeddingHub", "https://wedding.example"); err == nil {
		t.Fatal("accepted invalid sender")
	}
	if _, err := New("key", "sender@example.com", "WeddingHub", "file:///etc/passwd"); err == nil {
		t.Fatal("accepted a non-HTTP application URL")
	}
	if _, err := New("key", "sender@example.com", "WeddingHub", "http://wedding.example"); err == nil {
		t.Fatal("accepted a public reset URL over HTTP")
	}
	if _, err := New("key", "sender@example.com", "WeddingHub", "http://localhost:8080"); err != nil {
		t.Fatalf("rejected loopback development URL: %v", err)
	}
}
