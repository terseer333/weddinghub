package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"weddinghub/models"
	"weddinghub/repository"
)

// profileTestBed creates a wedding with one accepted committee member and returns the
// handler, the wedding id, the admin capability token, and the member's invitation token.
func profileTestBed(t *testing.T) (http.Handler, string, string, string) {
	t.Helper()
	repo := repository.NewMemoryRepository()
	handler := New(repo)
	create := httptest.NewRecorder()
	handler.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/api/weddings", bytes.NewBufferString(`{"slug":"profile-wed","title":"A & B","status":"published"}`)))
	if create.Code != http.StatusCreated {
		t.Fatalf("create wedding status = %d: %s", create.Code, create.Body)
	}
	var created weddingCreated
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	invite := httptest.NewRecorder()
	inviteRequest := httptest.NewRequest(http.MethodPost, "/api/weddings/"+created.Wedding.ID+"/invitations", bytes.NewBufferString(`{"type":"committee","guest_name":"Committed Member","committee_title":"Coordinator"}`))
	inviteRequest.Header.Set("Authorization", "Bearer "+created.AdminToken)
	handler.ServeHTTP(invite, inviteRequest)
	if invite.Code != http.StatusCreated {
		t.Fatalf("invite status = %d: %s", invite.Code, invite.Body)
	}
	var createdInvite invitationCreated
	if err := json.Unmarshal(invite.Body.Bytes(), &createdInvite); err != nil {
		t.Fatal(err)
	}
	accept := httptest.NewRecorder()
	handler.ServeHTTP(accept, httptest.NewRequest(http.MethodPost, "/api/invitations/"+createdInvite.Token+"/accept", bytes.NewBufferString(`{"role":"committee"}`)))
	if accept.Code != http.StatusOK {
		t.Fatalf("accept status = %d: %s", accept.Code, accept.Body)
	}
	return handler, created.Wedding.ID, created.AdminToken, createdInvite.Token
}

func TestCommitteeMemberManagesOwnProfile(t *testing.T) {
	handler, weddingID, adminToken, memberToken := profileTestBed(t)
	path := "/api/weddings/" + weddingID + "/profile"

	// The first read is synthesized from the membership rather than returning 404.
	fallbackResponse := httptest.NewRecorder()
	fallbackRequest := httptest.NewRequest(http.MethodGet, path, nil)
	fallbackRequest.Header.Set("Authorization", "Bearer "+memberToken)
	handler.ServeHTTP(fallbackResponse, fallbackRequest)
	if fallbackResponse.Code != http.StatusOK {
		t.Fatalf("default profile status = %d: %s", fallbackResponse.Code, fallbackResponse.Body)
	}
	var fallback models.Profile
	if err := json.Unmarshal(fallbackResponse.Body.Bytes(), &fallback); err != nil {
		t.Fatal(err)
	}
	if fallback.DisplayName != "Committed Member" || fallback.Role != models.RoleCommitteeMember || fallback.ID == models.ProfileKeyAdmin {
		t.Fatalf("unexpected default profile: %#v", fallback)
	}

	avatar := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUg=="
	update := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPut, path, bytes.NewBufferString(`{"display_name":"  Samantha  ","avatar":"`+avatar+`"}`))
	updateRequest.Header.Set("Authorization", "Bearer "+memberToken)
	handler.ServeHTTP(update, updateRequest)
	if update.Code != http.StatusOK {
		t.Fatalf("profile update status = %d: %s", update.Code, update.Body)
	}
	var saved models.Profile
	if err := json.Unmarshal(update.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.DisplayName != "Samantha" || saved.Avatar != avatar || saved.Role != models.RoleCommitteeMember || saved.UpdatedAt.IsZero() {
		t.Fatalf("unexpected saved profile: %#v", saved)
	}

	// A member's display name is mirrored onto their committee membership record.
	roster := httptest.NewRecorder()
	rosterRequest := httptest.NewRequest(http.MethodGet, "/api/weddings/"+weddingID+"/admin/roster", nil)
	rosterRequest.Header.Set("Authorization", "Bearer "+adminToken)
	handler.ServeHTTP(roster, rosterRequest)
	if roster.Code != http.StatusOK {
		t.Fatalf("roster status = %d: %s", roster.Code, roster.Body)
	}
	var rosterBody rosterView
	if err := json.Unmarshal(roster.Body.Bytes(), &rosterBody); err != nil {
		t.Fatal(err)
	}
	if len(rosterBody.CommitteeMembers) != 1 || rosterBody.CommitteeMembers[0].Name != "Samantha" {
		t.Fatalf("member name not synced: %#v", rosterBody.CommitteeMembers)
	}
	if len(rosterBody.Profiles) != 1 || rosterBody.Profiles[0].Avatar != avatar {
		t.Fatalf("roster missing stored profile: %#v", rosterBody.Profiles)
	}

	// The committee dashboard carries the caller's own profile for the header.
	dashboard := httptest.NewRecorder()
	dashboardRequest := httptest.NewRequest(http.MethodGet, "/api/weddings/"+weddingID+"/committee/dashboard", nil)
	dashboardRequest.Header.Set("Authorization", "Bearer "+memberToken)
	handler.ServeHTTP(dashboard, dashboardRequest)
	if dashboard.Code != http.StatusOK {
		t.Fatalf("dashboard status = %d: %s", dashboard.Code, dashboard.Body)
	}
	var view committeeDashboardView
	if err := json.Unmarshal(dashboard.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if view.Profile.DisplayName != "Samantha" || view.Profile.Avatar != avatar || len(view.Profiles) != 1 {
		t.Fatalf("dashboard profile = %#v, profiles = %#v", view.Profile, view.Profiles)
	}
}

func TestAdminProfileAndValidation(t *testing.T) {
	handler, weddingID, adminToken, memberToken := profileTestBed(t)
	path := "/api/weddings/" + weddingID + "/profile"

	// The admin owns the shared "admin" profile, and an unauthorized caller is rejected.
	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, path, nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated profile status = %d (want 401)", unauthorized.Code)
	}

	update := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPut, path, bytes.NewBufferString(`{"display_name":"Ada the Admin","avatar":""}`))
	updateRequest.Header.Set("Authorization", "Bearer "+adminToken)
	handler.ServeHTTP(update, updateRequest)
	if update.Code != http.StatusOK {
		t.Fatalf("admin profile update status = %d: %s", update.Code, update.Body)
	}
	var saved models.Profile
	if err := json.Unmarshal(update.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.ID != models.ProfileKeyAdmin || saved.Role != models.RoleAdmin || saved.DisplayName != "Ada the Admin" {
		t.Fatalf("unexpected admin profile: %#v", saved)
	}

	// An admin profile is separate from a committee member's profile.
	memberGet := httptest.NewRecorder()
	memberRequest := httptest.NewRequest(http.MethodGet, path, nil)
	memberRequest.Header.Set("Authorization", "Bearer "+memberToken)
	handler.ServeHTTP(memberGet, memberRequest)
	var memberProfile models.Profile
	if err := json.Unmarshal(memberGet.Body.Bytes(), &memberProfile); err != nil {
		t.Fatal(err)
	}
	if memberProfile.ID == models.ProfileKeyAdmin {
		t.Fatalf("committee member resolved the admin profile: %#v", memberProfile)
	}

	for name, payload := range map[string]string{
		"blank name":    `{"display_name":"   "}`,
		"remote avatar": `{"display_name":"Ada","avatar":"https://example.test/a.png"}`,
		"unknown field": `{"display_name":"Ada","role":"admin"}`,
		"name too long": `{"display_name":"` + string(bytes.Repeat([]byte("x"), models.MaxProfileDisplayNameLength+1)) + `"}`,
	} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPut, path, bytes.NewBufferString(payload))
			request.Header.Set("Authorization", "Bearer "+adminToken)
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body)
			}
		})
	}
}
