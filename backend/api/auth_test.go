package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"weddinghub/models"
	"weddinghub/repository"
)

const testPassword = "secret-passphrase-1"

// fakeCodeSender captures the code the API would email instead of delivering it.
type fakeCodeSender struct {
	to      string
	code    string
	calls   int
	failAll bool
}

func (f *fakeCodeSender) SendCode(_ context.Context, to, code string) error {
	if f.failAll {
		return context.DeadlineExceeded
	}
	f.to, f.code = to, code
	f.calls++
	return nil
}

// newAuthAPI builds a mux with a fake code sender and an adjustable clock.
func newAuthAPI(repo repository.Repository, sender codeSender, now func() time.Time) http.Handler {
	a := &API{repo: repo, now: now, sender: sender}
	return a.routes()
}

func postJSON(t *testing.T, handler http.Handler, path, body string, token string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func getWithToken(handler http.Handler, path, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func registerUser(t *testing.T, handler http.Handler, email, fullName, password string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(registerRequest{Email: email, FullName: fullName, Password: password})
	if err != nil {
		t.Fatal(err)
	}
	return postJSON(t, handler, "/api/auth/register", string(body), "")
}

// startLogin posts a password-backed login/start request.
func startLogin(t *testing.T, handler http.Handler, email, password string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(loginStartRequest{Email: email, Password: password})
	if err != nil {
		t.Fatal(err)
	}
	return postJSON(t, handler, "/api/auth/login/start", string(body), "")
}

func verifyCode(t *testing.T, handler http.Handler, email, code string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(loginVerifyRequest{Email: email, Code: code})
	if err != nil {
		t.Fatal(err)
	}
	return postJSON(t, handler, "/api/auth/login/verify", string(body), "")
}

func TestRegisterLoginSessionFlow(t *testing.T) {
	repo := repository.NewMemoryRepository()
	sender := &fakeCodeSender{}
	handler := newAuthAPI(repo, sender, func() time.Time { return time.Now().UTC() })

	created := registerUser(t, handler, " Alice@Example.com ", "Alice Nakato", testPassword)
	if created.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body = %s", created.Code, created.Body)
	}
	var user models.User
	if err := json.Unmarshal(created.Body.Bytes(), &user); err != nil {
		t.Fatal(err)
	}
	if user.Email != "alice@example.com" {
		t.Fatalf("email not normalized: %q", user.Email)
	}
	if user.PasswordHash != "" || user.PasswordAttempt != 0 {
		t.Fatalf("password state leaked in registration response: %+v", user)
	}

	if duplicate := registerUser(t, handler, "alice@example.com", "Someone Else", testPassword); duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate register status = %d, body = %s", duplicate.Code, duplicate.Body)
	}

	if wrongPassword := startLogin(t, handler, "alice@example.com", "not-the-password-1"); wrongPassword.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password status = %d, body = %s", wrongPassword.Code, wrongPassword.Body)
	}

	start := startLogin(t, handler, "alice@example.com", testPassword)
	if start.Code != http.StatusAccepted {
		t.Fatalf("login/start status = %d, body = %s", start.Code, start.Body)
	}
	if sender.to != "alice@example.com" || len(sender.code) != 6 {
		t.Fatalf("code not delivered: to=%q code=%q", sender.to, sender.code)
	}

	wrong := verifyCode(t, handler, "alice@example.com", "000000")
	if wrong.Code != http.StatusUnauthorized {
		t.Fatalf("wrong code status = %d, body = %s", wrong.Code, wrong.Body)
	}

	verified := verifyCode(t, handler, "alice@example.com", sender.code)
	if verified.Code != http.StatusOK {
		t.Fatalf("verify status = %d, body = %s", verified.Code, verified.Body)
	}
	var session sessionIssued
	if err := json.Unmarshal(verified.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	if session.SessionToken == "" || session.User.ID != user.ID {
		t.Fatalf("unexpected session payload: %+v", session)
	}

	me := getWithToken(handler, "/api/auth/me", session.SessionToken)
	if me.Code != http.StatusOK {
		t.Fatalf("me status = %d, body = %s", me.Code, me.Body)
	}
	var meUser models.User
	if err := json.Unmarshal(me.Body.Bytes(), &meUser); err != nil {
		t.Fatal(err)
	}
	if meUser.Email != "alice@example.com" || meUser.ID != user.ID {
		t.Fatalf("me returned wrong user: %+v", meUser)
	}

	// The code is single-use: a second verification must fail.
	if replayed := verifyCode(t, handler, "alice@example.com", sender.code); replayed.Code != http.StatusBadRequest {
		t.Fatalf("replayed code status = %d, body = %s", replayed.Code, replayed.Body)
	}

	if out := postJSON(t, handler, "/api/auth/logout", `{}`, session.SessionToken); out.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, body = %s", out.Code, out.Body)
	}
	if me = getWithToken(handler, "/api/auth/me", session.SessionToken); me.Code != http.StatusUnauthorized {
		t.Fatalf("me after logout status = %d, body = %s", me.Code, me.Body)
	}
}

func TestLoginStartHidesUnknownEmailsAndDelaysResends(t *testing.T) {
	repo := repository.NewMemoryRepository()
	sender := &fakeCodeSender{}
	now := time.Now().UTC()
	handler := newAuthAPI(repo, sender, func() time.Time { return now })

	// Unknown emails must answer exactly like wrong passwords: same status and
	// message, so the endpoint cannot be used to enumerate accounts.
	unknown := startLogin(t, handler, "nobody@example.com", testPassword)
	if unknown.Code != http.StatusUnauthorized {
		t.Fatalf("unknown email status = %d, body = %s", unknown.Code, unknown.Body)
	}
	var unknownBody, wrongBody map[string]string
	if err := json.Unmarshal(unknown.Body.Bytes(), &unknownBody); err != nil {
		t.Fatal(err)
	}

	registerUser(t, handler, "host@example.com", "Host", testPassword)
	wrongPassword := startLogin(t, handler, "host@example.com", "still-not-the-password-1")
	if wrongPassword.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password status = %d, body = %s", wrongPassword.Code, wrongPassword.Body)
	}
	if err := json.Unmarshal(wrongPassword.Body.Bytes(), &wrongBody); err != nil {
		t.Fatal(err)
	}
	if unknownBody["error"] != wrongBody["error"] {
		t.Fatalf("unknown email and wrong password disagree: %q vs %q", unknownBody["error"], wrongBody["error"])
	}

	if first := startLogin(t, handler, "host@example.com", testPassword); first.Code != http.StatusAccepted {
		t.Fatalf("first start status = %d, body = %s", first.Code, first.Body)
	}
	if second := startLogin(t, handler, "host@example.com", testPassword); second.Code != http.StatusTooManyRequests {
		t.Fatalf("cooldown status = %d, body = %s", second.Code, second.Body)
	}
	now = now.Add(2 * loginCodeCooldown)
	if third := startLogin(t, handler, "host@example.com", testPassword); third.Code != http.StatusAccepted {
		t.Fatalf("post-cooldown start status = %d, body = %s", third.Code, third.Body)
	}
}

func TestPasswordLockout(t *testing.T) {
	repo := repository.NewMemoryRepository()
	sender := &fakeCodeSender{}
	now := time.Now().UTC()
	handler := newAuthAPI(repo, sender, func() time.Time { return now })

	registerUser(t, handler, "target@example.com", "Target", testPassword)
	for i := 1; i <= passwordMaxAttempts; i++ {
		if res := startLogin(t, handler, "target@example.com", "wrong-password-"+string(rune('0'+i))); res.Code != http.StatusUnauthorized {
			t.Fatalf("failed attempt %d status = %d, body = %s", i, res.Code, res.Body)
		}
	}
	// The lock window rejects even the correct password without verifying it.
	if locked := startLogin(t, handler, "target@example.com", testPassword); locked.Code != http.StatusTooManyRequests {
		t.Fatalf("locked status = %d, body = %s", locked.Code, locked.Body)
	}
	if sender.calls != 0 {
		t.Fatalf("code sent during lockout (%d calls)", sender.calls)
	}
	now = now.Add(passwordLockout + time.Second)
	if ok := startLogin(t, handler, "target@example.com", testPassword); ok.Code != http.StatusAccepted {
		t.Fatalf("post-lockout status = %d, body = %s", ok.Code, ok.Body)
	}
}

func TestLoginCodeExpiry(t *testing.T) {
	repo := repository.NewMemoryRepository()
	sender := &fakeCodeSender{}
	now := time.Now().UTC()
	handler := newAuthAPI(repo, sender, func() time.Time { return now })

	registerUser(t, handler, "late@example.com", "Late", testPassword)
	if res := startLogin(t, handler, "late@example.com", testPassword); res.Code != http.StatusAccepted {
		t.Fatalf("start status = %d, body = %s", res.Code, res.Body)
	}
	now = now.Add(loginCodeTTL + time.Second)
	expired := verifyCode(t, handler, "late@example.com", sender.code)
	if expired.Code != http.StatusGone {
		t.Fatalf("expired code status = %d, body = %s", expired.Code, expired.Body)
	}
}

func TestLoginCodeAttemptLimit(t *testing.T) {
	repo := repository.NewMemoryRepository()
	sender := &fakeCodeSender{}
	handler := newAuthAPI(repo, sender, func() time.Time { return time.Now().UTC() })

	registerUser(t, handler, "target@example.com", "Target", testPassword)
	if res := startLogin(t, handler, "target@example.com", testPassword); res.Code != http.StatusAccepted {
		t.Fatalf("start status = %d, body = %s", res.Code, res.Body)
	}
	for i := 0; i < loginCodeMaxAttempts; i++ {
		if res := verifyCode(t, handler, "target@example.com", "000000"); res.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, body = %s", i, res.Code, res.Body)
		}
	}
	// The correct code must be rejected once the attempt budget is spent.
	if res := verifyCode(t, handler, "target@example.com", sender.code); res.Code != http.StatusBadRequest {
		t.Fatalf("exhausted code status = %d, body = %s", res.Code, res.Body)
	}
}

func TestStartLoginReportsDeliveryFailure(t *testing.T) {
	repo := repository.NewMemoryRepository()
	handler := newAuthAPI(repo, &fakeCodeSender{failAll: true}, func() time.Time { return time.Now().UTC() })

	registerUser(t, handler, "mail@example.com", "Mail", testPassword)
	res := startLogin(t, handler, "mail@example.com", testPassword)
	if res.Code != http.StatusBadGateway {
		t.Fatalf("delivery failure status = %d, body = %s", res.Code, res.Body)
	}
}

func TestRegisterValidation(t *testing.T) {
	repo := repository.NewMemoryRepository()
	handler := newAuthAPI(repo, &fakeCodeSender{}, func() time.Time { return time.Now().UTC() })

	cases := []struct {
		name, body string
	}{
		{"invalid email", `{"email":"not-an-email","full_name":"A","password":"` + testPassword + `"}`},
		{"missing name", `{"email":"a@example.com","full_name":"  ","password":"` + testPassword + `"}`},
		{"short password", `{"email":"a@example.com","full_name":"A","password":"short1"}`},
		{"no digit", `{"email":"a@example.com","full_name":"A","password":"only-letters-here"}`},
		{"unknown field", `{"email":"a@example.com","full_name":"A","password":"` + testPassword + `","role":"admin"}`},
	}
	for _, tc := range cases {
		if res := postJSON(t, handler, "/api/auth/register", tc.body, ""); res.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, body = %s", tc.name, res.Code, res.Body)
		}
	}
}
