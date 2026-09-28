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

// The wedding admin capability token is only ever revealed once, at creation, and it lives
// in browser storage. Losing it (signing out, a new device) must not lock an owner out of
// their own wedding, so a signed-in account that administers the wedding is accepted too.

func signupSession(t *testing.T, handler http.Handler, email string) string {
	t.Helper()
	response := postJSON(handler, "/api/auth/signup", `{"email":"`+email+`","password":"supersecret","display_name":"Owner"}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("signup %s status = %d: %s", email, response.Code, response.Body)
	}
	var session sessionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	if session.SessionToken == "" {
		t.Fatalf("signup %s issued no session", email)
	}
	return session.SessionToken
}

func withToken(handler http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}
	request := httptest.NewRequest(method, path, reader)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestAccountOwnershipAdministersWeddingWithoutCapabilityToken(t *testing.T) {
	handler := New(repository.NewMemoryRepository())
	owner := signupSession(t, handler, "owner@example.com")

	// Creating while signed in records the creator as an owner.
	created := withToken(handler, http.MethodPost, "/api/weddings", owner,
		`{"slug":"ada-and-sam","title":"Ada & Sam","partner_one":"Ada","partner_two":"Sam","status":"published"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create with a session status = %d: %s", created.Code, created.Body)
	}
	var payload struct {
		Wedding    models.Wedding `json:"wedding"`
		AdminToken string         `json:"admin_token"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Wedding.ID == "" || payload.AdminToken == "" {
		t.Fatalf("unexpected create payload: %#v", payload)
	}
	if len(payload.Wedding.Admins) != 1 || payload.Wedding.Admins[0].UserID == "" || payload.Wedding.Admins[0].Role != models.RoleOwner {
		t.Fatalf("creator was not recorded as an owner: %#v", payload.Wedding.Admins)
	}
	weddingID := payload.Wedding.ID

	// The account finds the wedding it administers without any capability token.
	mine := withToken(handler, http.MethodGet, "/api/weddings/mine", owner, "")
	if mine.Code != http.StatusOK {
		t.Fatalf("mine status = %d: %s", mine.Code, mine.Body)
	}
	var listed []weddingSummary
	if err := json.Unmarshal(mine.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != weddingID {
		t.Fatalf("unexpected mine list: %#v", listed)
	}

	// Session-only administration works, including the action the user reported failing.
	update := withToken(handler, http.MethodPut, "/api/weddings/"+weddingID, owner,
		`{"slug":"ada-and-sam","title":"Ada & Sam","partner_one":"Ada","partner_two":"Sam","status":"published","venue":"Garden"}`)
	if update.Code != http.StatusOK {
		t.Fatalf("session-only update status = %d: %s", update.Code, update.Body)
	}
	invite := withToken(handler, http.MethodPost, "/api/weddings/"+weddingID+"/invitations", owner,
		`{"type":"guest","guest_name":"Taylor","guest_email":"taylor@example.com","max_party_size":2}`)
	if invite.Code != http.StatusCreated {
		t.Fatalf("invite with a session status = %d: %s", invite.Code, invite.Body)
	}

	// The one-time capability token still administers the wedding.
	capInvite := withToken(handler, http.MethodPost, "/api/weddings/"+weddingID+"/invitations", payload.AdminToken,
		`{"type":"guest","guest_name":"Capability","max_party_size":1}`)
	if capInvite.Code != http.StatusCreated {
		t.Fatalf("capability-token invite status = %d: %s", capInvite.Code, capInvite.Body)
	}

	// No credential at all is still refused, with the reported message.
	anonymous := withToken(handler, http.MethodPost, "/api/weddings/"+weddingID+"/invitations", "",
		`{"type":"guest","guest_name":"Nobody","max_party_size":1}`)
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous invite status = %d, want 401", anonymous.Code)
	}
	if !strings.Contains(anonymous.Body.String(), "a valid capability token is required") {
		t.Fatalf("unexpected anonymous message: %s", anonymous.Body)
	}

	// Another account cannot administer or even list this wedding.
	stranger := signupSession(t, handler, "stranger@example.com")
	foreign := withToken(handler, http.MethodPut, "/api/weddings/"+weddingID, stranger,
		`{"slug":"ada-and-sam","title":"Ada & Sam","status":"published"}`)
	if foreign.Code != http.StatusForbidden {
		t.Fatalf("stranger update status = %d, want 403: %s", foreign.Code, foreign.Body)
	}
	strangerMine := withToken(handler, http.MethodGet, "/api/weddings/mine", stranger, "")
	var strangerList []weddingSummary
	if err := json.Unmarshal(strangerMine.Body.Bytes(), &strangerList); err != nil {
		t.Fatal(err)
	}
	if len(strangerList) != 0 {
		t.Fatalf("a stranger sees weddings they do not administer: %#v", strangerList)
	}
}
