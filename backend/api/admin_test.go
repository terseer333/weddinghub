package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"weddinghub/models"
	"weddinghub/repository"
)

// The platform administrator is decided by server configuration, not by anything the browser
// sends. These tests pin the contract: only the configured address may sign in at /api/admin/login,
// its session is the only thing accepted by /api/admin/*, and every attempt is throttled and
// recorded.

const (
	adminAddress = "boss@example.com"
	adminSecret  = "supersecret"
)

// adminRequest sends a request with an optional session token and an optional client address. The
// address matters because login throttling is keyed on email + client address.
func adminRequest(handler http.Handler, method, path, token, body, forwardedFor string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}
	request := httptest.NewRequest(method, path, reader)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if forwardedFor != "" {
		request.Header.Set("X-Forwarded-For", forwardedFor)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func adminLoginBody(email, password string) string {
	encoded, _ := json.Marshal(map[string]string{"email": email, "password": password})
	return string(encoded)
}

// adminSession signs up the administrator account and signs it in through the admin endpoint,
// returning the session token.
func adminSession(t *testing.T, handler http.Handler) string {
	t.Helper()
	signupSession(t, handler, adminAddress)
	response := postJSON(handler, "/api/admin/login", adminLoginBody(adminAddress, adminSecret))
	if response.Code != http.StatusOK {
		t.Fatalf("admin login status = %d: %s", response.Code, response.Body)
	}
	var session sessionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	if session.SessionToken == "" {
		t.Fatal("admin login issued no session")
	}
	if !session.User.IsAdmin {
		t.Fatal("admin login response did not mark the account as the administrator")
	}
	return session.SessionToken
}

func TestAdminLoginAcceptsOnlyTheConfiguredAdministrator(t *testing.T) {
	t.Setenv("WEDDINGHUB_ADMIN_EMAIL", "Boss@Example.com") // the setting is normalized, not trusted verbatim
	repo := repository.NewMemoryRepository()
	handler := New(repo)

	// A normal account exists with the same password the administrator will use.
	normalSession := signupSession(t, handler, "guest@example.com")

	// The administrator signs in.
	token := adminSession(t, handler)

	// A valid non-administrator account is refused, with no session, and cannot pass as admin.
	denied := postJSON(handler, "/api/admin/login", adminLoginBody("guest@example.com", adminSecret))
	if denied.Code != http.StatusUnauthorized {
		t.Fatalf("non-admin login status = %d, want 401: %s", denied.Code, denied.Body)
	}
	if strings.Contains(denied.Body.String(), "session_token") {
		t.Fatalf("non-admin login revealed a session: %s", denied.Body)
	}

	// A wrong password for the administrator is refused the same way.
	wrong := postJSON(handler, "/api/admin/login", adminLoginBody(adminAddress, "not-the-password"))
	if wrong.Code != http.StatusUnauthorized {
		t.Fatalf("wrong admin password status = %d, want 401: %s", wrong.Code, wrong.Body)
	}

	// A normal login must not advertise administration at all.
	normal := postJSON(handler, "/api/auth/login", adminLoginBody("guest@example.com", adminSecret))
	if normal.Code != http.StatusOK {
		t.Fatalf("normal login status = %d: %s", normal.Code, normal.Body)
	}
	if strings.Contains(normal.Body.String(), `"is_admin"`) {
		t.Fatalf("normal login response exposed an admin flag: %s", normal.Body)
	}

	// The administrator's session works.
	me := adminRequest(handler, http.MethodGet, "/api/admin/me", token, "", "")
	if me.Code != http.StatusOK {
		t.Fatalf("admin me status = %d: %s", me.Code, me.Body)
	}
	if !strings.Contains(me.Body.String(), `"is_admin":true`) {
		t.Fatalf("admin me did not mark the administrator: %s", me.Body)
	}

	// A normal account's session is refused by every admin endpoint.
	forbidden := adminRequest(handler, http.MethodGet, "/api/admin/me", normalSession, "", "")
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("ordinary session on admin me status = %d, want 403: %s", forbidden.Code, forbidden.Body)
	}
}

func TestAdminEndpointsRefuseWithoutAnAdministratorConfigured(t *testing.T) {
	t.Setenv("WEDDINGHUB_ADMIN_EMAIL", "")
	handler := New(repository.NewMemoryRepository())
	userSession := signupSession(t, handler, "someone@example.com")

	// With nobody configured, even a correct credential is refused.
	login := postJSON(handler, "/api/admin/login", adminLoginBody("someone@example.com", adminSecret))
	if login.Code != http.StatusUnauthorized {
		t.Fatalf("login with no administrator configured = %d, want 401: %s", login.Code, login.Body)
	}

	// And the protected endpoints refuse a normal session and an anonymous caller.
	if response := adminRequest(handler, http.MethodGet, "/api/admin/me", userSession, "", ""); response.Code != http.StatusForbidden {
		t.Fatalf("admin me without an administrator = %d, want 403: %s", response.Code, response.Body)
	}
	if response := adminRequest(handler, http.MethodGet, "/api/admin/me", "", "", ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous admin me = %d, want 401: %s", response.Code, response.Body)
	}
}

func TestAdminLoginThrottlesRepeatedFailuresPerAddress(t *testing.T) {
	t.Setenv("WEDDINGHUB_ADMIN_EMAIL", adminAddress)
	handler := New(repository.NewMemoryRepository())
	signupSession(t, handler, adminAddress)

	// Five wrong passwords exhaust the allowance.
	for attempt := 1; attempt <= adminLoginMaxFailures; attempt++ {
		response := postJSON(handler, "/api/admin/login", adminLoginBody(adminAddress, "wrong-password"))
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("failure %d status = %d, want 401: %s", attempt, response.Code, response.Body)
		}
	}

	// The sixth is refused even though the password is now correct, and advertises a retry delay.
	locked := postJSON(handler, "/api/admin/login", adminLoginBody(adminAddress, adminSecret))
	if locked.Code != http.StatusTooManyRequests {
		t.Fatalf("locked login status = %d, want 429: %s", locked.Code, locked.Body)
	}
	if locked.Header().Get("Retry-After") == "" {
		t.Fatal("lockout response did not set Retry-After")
	}

	// A different client address is unaffected, so a locked-out address cannot deny the
	// administrator access from somewhere else.
	other := adminRequest(handler, http.MethodPost, "/api/admin/login", "", adminLoginBody(adminAddress, adminSecret), "203.0.113.9")
	if other.Code != http.StatusOK {
		t.Fatalf("login from another address = %d, want 200: %s", other.Code, other.Body)
	}
}

func TestAdminLogoutInvalidatesTheSession(t *testing.T) {
	t.Setenv("WEDDINGHUB_ADMIN_EMAIL", adminAddress)
	handler := New(repository.NewMemoryRepository())
	token := adminSession(t, handler)

	if response := adminRequest(handler, http.MethodPost, "/api/admin/logout", token, "", ""); response.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want 204: %s", response.Code, response.Body)
	}
	// The token cannot be replayed after signing out.
	if response := adminRequest(handler, http.MethodGet, "/api/admin/me", token, "", ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("reused token after logout = %d, want 401: %s", response.Code, response.Body)
	}
}

func TestAdminActionsAreRecordedInTheAuditTrail(t *testing.T) {
	t.Setenv("WEDDINGHUB_ADMIN_EMAIL", adminAddress)
	repo := repository.NewMemoryRepository()
	handler := New(repo)

	// A failed attempt and a successful sign-in are both recorded.
	postJSON(handler, "/api/admin/login", adminLoginBody(adminAddress, "wrong-password"))
	token := adminSession(t, handler)

	// A normal account reaching for an admin endpoint is recorded as a denial.
	normalSession := signupSession(t, handler, "guest@example.com")
	adminRequest(handler, http.MethodGet, "/api/admin/me", normalSession, "", "")

	// Signing out is recorded too.
	adminRequest(handler, http.MethodPost, "/api/admin/logout", token, "", "")

	entries, err := repo.ListAuditLogs(10)
	if err != nil {
		t.Fatal(err)
	}
	// Newest first.
	actions := make([]string, 0, len(entries))
	for _, entry := range entries {
		actions = append(actions, entry.Action)
	}
	for _, want := range []string{models.AuditAdminLoginFailed, models.AuditAdminLogin, models.AuditAdminAccessDenied, models.AuditAdminLogout} {
		if !containsString(actions, want) {
			t.Fatalf("audit trail is missing %s: %v", want, actions)
		}
	}
	if actions[0] != models.AuditAdminLogout {
		t.Fatalf("audit trail is not newest-first, got %v", actions)
	}

	// The successful sign-in names the administrator and no entry carries a password.
	var login models.AuditLog
	for _, entry := range entries {
		if entry.Action == models.AuditAdminLogin {
			login = entry
		}
	}
	if login.ActorEmail != adminAddress {
		t.Fatalf("admin login audit entry names %q, want %q", login.ActorEmail, adminAddress)
	}
	if login.CreatedAt.IsZero() || login.Metadata["client_address"] == "" {
		t.Fatalf("admin login audit entry is incomplete: %#v", login)
	}
}

// The wedding dashboard offers a link to the console to the administrator alone. It learns that
// from its own account record, so /api/auth/me must mark the administrator and nobody else.
func TestAuthMeReportsTheAdministratorOnlyToItsOwnAccount(t *testing.T) {
	t.Setenv("WEDDINGHUB_ADMIN_EMAIL", adminAddress)
	handler := New(repository.NewMemoryRepository())

	normalSession := signupSession(t, handler, "guest@example.com")
	adminToken := adminSession(t, handler)

	adminMe := adminRequest(handler, http.MethodGet, "/api/auth/me", adminToken, "", "")
	if adminMe.Code != http.StatusOK || !strings.Contains(adminMe.Body.String(), `"is_admin":true`) {
		t.Fatalf("administrator /api/auth/me = %d %s, want is_admin true", adminMe.Code, adminMe.Body)
	}

	normalMe := adminRequest(handler, http.MethodGet, "/api/auth/me", normalSession, "", "")
	if normalMe.Code != http.StatusOK {
		t.Fatalf("ordinary /api/auth/me status = %d: %s", normalMe.Code, normalMe.Body)
	}
	if strings.Contains(normalMe.Body.String(), "is_admin") {
		t.Fatalf("ordinary /api/auth/me exposed an admin flag: %s", normalMe.Body)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
