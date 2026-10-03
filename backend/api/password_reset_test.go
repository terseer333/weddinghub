package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"weddinghub/mailer"
	"weddinghub/models"
	"weddinghub/repository"
)

type capturedReset struct{ to, name, token string }
type fakePasswordMailer struct {
	resets   chan capturedReset
	changed  chan string
	resetErr error
}

func newFakePasswordMailer() *fakePasswordMailer {
	return &fakePasswordMailer{resets: make(chan capturedReset, 10), changed: make(chan string, 10)}
}
func (f *fakePasswordMailer) SendPasswordReset(_ context.Context, to, name, token string) error {
	f.resets <- capturedReset{to: to, name: name, token: token}
	return f.resetErr
}
func (f *fakePasswordMailer) SendPasswordChanged(_ context.Context, to, _ string) error {
	f.changed <- to
	return nil
}

var _ mailer.PasswordMailer = (*fakePasswordMailer)(nil)

func TestForgotPasswordGenericAndResetRevokesSessions(t *testing.T) {
	repo := repository.NewMemoryRepository()
	passwordHash, err := models.HashPassword("old-password")
	if err != nil {
		t.Fatal(err)
	}
	user, err := repo.CreateUser(models.User{ID: "reset-user", Email: "ada@example.com", DisplayName: "Ada", Role: models.RoleOwner, Status: "active", PasswordHash: passwordHash, CreatedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	sessionToken, sessionHash, err := models.NewOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.AddSession(models.Session{TokenHash: sessionHash, UserID: user.ID, CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	fake := newFakePasswordMailer()
	handler := NewWithPasswordMailer(repo, fake)

	request := postJSON(handler, "/api/auth/forgot-password", `{"email":"missing@example.com"}`)
	known := postJSON(handler, "/api/auth/forgot-password", `{"email":" ADA@example.com "}`)
	if request.Code != http.StatusAccepted || known.Code != request.Code || known.Body.String() != request.Body.String() {
		t.Fatalf("forgot responses differ: unknown=%d %s known=%d %s", request.Code, request.Body, known.Code, known.Body)
	}
	var reset capturedReset
	select {
	case reset = <-fake.resets:
	case <-time.After(time.Second):
		t.Fatal("registered address did not schedule a reset email")
	}
	if reset.to != user.Email || !validToken(reset.token) {
		t.Fatalf("unexpected reset email: %#v", reset)
	}
	select {
	case <-fake.resets:
		t.Fatal("unregistered address received a reset email")
	case <-time.After(30 * time.Millisecond):
	}
	_ = postJSON(handler, "/api/auth/forgot-password", `{"email":"ada@example.com"}`)
	var replacement capturedReset
	select {
	case replacement = <-fake.resets:
	case <-time.After(time.Second):
		t.Fatal("replacement reset email was not scheduled")
	}
	firstTokenBody, _ := json.Marshal(map[string]string{"token": reset.token, "password": "another-password"})
	firstUse := httptest.NewRecorder()
	handler.ServeHTTP(firstUse, httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewReader(firstTokenBody)))
	if firstUse.Code != http.StatusBadRequest {
		t.Fatalf("superseded token status %d: %s", firstUse.Code, firstUse.Body)
	}

	body, _ := json.Marshal(map[string]string{"token": replacement.token, "password": "new-password"})
	changed := httptest.NewRecorder()
	handler.ServeHTTP(changed, httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewReader(body)))
	if changed.Code != http.StatusOK {
		t.Fatalf("reset status %d: %s", changed.Code, changed.Body)
	}
	select {
	case recipient := <-fake.changed:
		if recipient != user.Email {
			t.Fatalf("changed notice recipient %q", recipient)
		}
	case <-time.After(time.Second):
		t.Fatal("password-changed notice not scheduled")
	}
	if old := postJSON(handler, "/api/auth/login", `{"email":"ada@example.com","password":"old-password"}`); old.Code != http.StatusUnauthorized {
		t.Fatalf("old password status %d: %s", old.Code, old.Body)
	}
	if current := postJSON(handler, "/api/auth/login", `{"email":"ada@example.com","password":"new-password"}`); current.Code != http.StatusOK {
		t.Fatalf("new password status %d: %s", current.Code, current.Body)
	}
	oldSession := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+sessionToken)
	handler.ServeHTTP(oldSession, req)
	if oldSession.Code != http.StatusUnauthorized {
		t.Fatalf("old session remained valid: %d", oldSession.Code)
	}
	reused := httptest.NewRecorder()
	handler.ServeHTTP(reused, httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewReader(body)))
	if reused.Code != http.StatusBadRequest {
		t.Fatalf("used token status %d: %s", reused.Code, reused.Body)
	}
}

func TestPasswordResetRateLimiterEnforcesIPAndEmailBudgets(t *testing.T) {
	limiter := newPasswordResetLimiter()
	now := time.Now().UTC()
	for i := 0; i < passwordResetEmailLimit; i++ {
		if !limiter.allow("192.0.2.10", "same@example.com", now) {
			t.Fatalf("email request %d was unexpectedly limited", i)
		}
	}
	if limiter.allow("192.0.2.11", "same@example.com", now) {
		t.Fatal("email-specific limit was not enforced across IPs")
	}
	limiter = newPasswordResetLimiter()
	for i := 0; i < passwordResetIPLimit; i++ {
		if !limiter.allow("192.0.2.20", "user"+string(rune('a'+i))+"@example.com", now) {
			t.Fatalf("IP request %d was unexpectedly limited", i)
		}
	}
	if limiter.allow("192.0.2.20", "another@example.com", now) {
		t.Fatal("IP-specific limit was not enforced across emails")
	}
}

func TestPasswordResetExpiredTokenAndRateLimit(t *testing.T) {
	repo := repository.NewMemoryRepository()
	passwordHash, _ := models.HashPassword("old-password")
	user, err := repo.CreateUser(models.User{ID: "limited-user", Email: "limited@example.com", Role: models.RoleOwner, Status: "active", PasswordHash: passwordHash, CreatedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	expiredToken, expiredHash, err := models.NewOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := repo.CreatePasswordResetToken(models.PasswordResetToken{UserID: user.ID, TokenHash: expiredHash, CreatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	fake := newFakePasswordMailer()
	handler := NewWithPasswordMailer(repo, fake)
	body, _ := json.Marshal(map[string]string{"token": expiredToken, "password": "new-password"})
	expired := httptest.NewRecorder()
	handler.ServeHTTP(expired, httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewReader(body)))
	if expired.Code != http.StatusGone || !strings.Contains(expired.Body.String(), "expired") {
		t.Fatalf("expired token response %d: %s", expired.Code, expired.Body)
	}
	for i := 0; i < 4; i++ {
		response := postJSON(handler, "/api/auth/forgot-password", `{"email":"limited@example.com"}`)
		if response.Code != http.StatusAccepted {
			t.Fatalf("limited request %d: %d", i, response.Code)
		}
	}
	for i := 0; i < 3; i++ {
		select {
		case <-fake.resets:
		case <-time.After(time.Second):
			t.Fatalf("only %d reset emails sent", i)
		}
	}
	select {
	case <-fake.resets:
		t.Fatal("rate limited request sent an email")
	case <-time.After(40 * time.Millisecond):
	}
}

func TestForgotPasswordRemainsGenericWhenMailerFails(t *testing.T) {
	repo := repository.NewMemoryRepository()
	hash, _ := models.HashPassword("old-password")
	_, _ = repo.CreateUser(models.User{ID: "mailer-failure", Email: "fail@example.com", Role: models.RoleOwner, Status: "active", PasswordHash: hash, CreatedAt: time.Now().UTC()})
	fake := newFakePasswordMailer()
	fake.resetErr = errors.New("provider offline")
	handler := NewWithPasswordMailer(repo, fake)
	response := postJSON(handler, "/api/auth/forgot-password", `{"email":"fail@example.com"}`)
	if response.Code != http.StatusAccepted || !strings.Contains(response.Body.String(), "If an account exists") {
		t.Fatalf("provider failure disclosed state: %d %s", response.Code, response.Body)
	}
	select {
	case <-fake.resets:
	case <-time.After(time.Second):
		t.Fatal("mailer was not called")
	}
}

func TestConfiguredAdministratorCanRecoverPassword(t *testing.T) {
	t.Setenv("WEDDINGHUB_ADMIN_EMAIL", "platform@example.com")
	repo := repository.NewMemoryRepository()
	oldHash, _ := models.HashPassword("admin-old-password")
	_, err := repo.CreateUser(models.User{ID: "platform-admin", Email: "platform@example.com", DisplayName: "Platform Admin", Role: models.RoleOwner, Status: "active", PasswordHash: oldHash, CreatedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	fake := newFakePasswordMailer()
	handler := NewWithPasswordMailer(repo, fake)
	if response := postJSON(handler, "/api/auth/forgot-password", `{"email":"platform@example.com"}`); response.Code != http.StatusAccepted {
		t.Fatalf("recovery status %d: %s", response.Code, response.Body)
	}
	var reset capturedReset
	select {
	case reset = <-fake.resets:
	case <-time.After(time.Second):
		t.Fatal("administrator reset email not sent")
	}
	body, _ := json.Marshal(map[string]string{"token": reset.token, "password": "admin-new-password"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewReader(body)))
	if response.Code != http.StatusOK {
		t.Fatalf("administrator reset status %d: %s", response.Code, response.Body)
	}
	login := postJSON(handler, "/api/admin/login", `{"email":"platform@example.com","password":"admin-new-password"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("administrator could not sign in after reset: %d %s", login.Code, login.Body)
	}
}
