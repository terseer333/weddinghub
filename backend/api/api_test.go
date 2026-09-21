package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"weddinghub/delivery"
	"weddinghub/models"
	"weddinghub/repository"
)

func TestGuestDashboardFiltersContentAndDoesNotLeakHash(t *testing.T) {
	repo := repository.NewMemoryRepository()
	now := time.Now().UTC()
	wedding := models.Wedding{ID: "w1", Slug: "alex-sam", Title: "Alex & Sam", Status: models.StatusPublished, CreatedAt: now, UpdatedAt: now,
		Events: []models.Event{{ID: "published", Name: "Ceremony", Status: models.StatusPublished}, {ID: "draft", Name: "Secret", Status: models.StatusDraft}},
	}
	if _, err := repo.CreateWedding(wedding); err != nil {
		t.Fatal(err)
	}
	token, hash, err := models.NewOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.AddInvitation("w1", models.Invitation{ID: "i1", GuestName: "Taylor", MaxPartySize: 2, Status: models.InvitationPending, TokenHash: hash, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}

	handler := NewWithAllowedOrigin(repo, "")
	pendingResponse := httptest.NewRecorder()
	handler.ServeHTTP(pendingResponse, httptest.NewRequest(http.MethodGet, "/api/guest/"+token+"/dashboard", nil))
	if pendingResponse.Code != http.StatusForbidden {
		t.Fatalf("pending invitation status = %d, body = %s", pendingResponse.Code, pendingResponse.Body)
	}
	declinedToken, declinedHash, err := models.NewOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.AddInvitation("w1", models.Invitation{ID: "i2", GuestName: "Jordan", MaxPartySize: 1, Status: models.InvitationDeclined, TokenHash: declinedHash, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	declinedResponse := httptest.NewRecorder()
	handler.ServeHTTP(declinedResponse, httptest.NewRequest(http.MethodGet, "/api/guest/"+declinedToken+"/dashboard", nil))
	if declinedResponse.Code != http.StatusForbidden {
		t.Fatalf("declined invitation status = %d, body = %s", declinedResponse.Code, declinedResponse.Body)
	}
	if _, _, err = repo.RespondToInvitation(hash, models.InvitationAccepted, now); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/guest/"+token+"/dashboard", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("accepted invitation status = %d, body = %s", response.Code, response.Body)
	}
	if strings.Contains(response.Body.String(), hash) {
		t.Fatal("response leaked invitation token hash")
	}
	var dashboard dashboardView
	if err := json.Unmarshal(response.Body.Bytes(), &dashboard); err != nil {
		t.Fatal(err)
	}
	if len(dashboard.Wedding.Events) != 1 || dashboard.Wedding.Events[0].ID != "published" {
		t.Fatalf("unexpected events: %#v", dashboard.Wedding.Events)
	}
}

func TestInvitationReturnsPublicWeddingProjection(t *testing.T) {
	repo := repository.NewMemoryRepository()
	now := time.Now().UTC()
	wedding := models.Wedding{
		ID: "w1", Slug: "alex-sam", Title: "Alex & Sam", PartnerOne: "Alex", PartnerTwo: "Sam", Status: models.StatusDraft,
		Venue: "Garden", Address: "1 Main St", City: "Nairobi", State: "Nairobi County", Country: "Kenya",
		Message: "Celebrate with us", Verse: "Love never fails", DressCode: "Formal", HeroImage: "https://example.test/hero.jpg", TemplateID: "classic",
		Admins: []models.Admin{{UserID: "private-admin", Role: models.RoleOwner}},
		Guests: []models.Guest{{ID: "private-guest", InvitationID: "another-invitation", Name: "Other Guest"}},
		Events: []models.Event{
			{ID: "public-event", Name: "Ceremony", Status: models.StatusPublished},
			{ID: "draft-event", Name: "unreleased-event", Status: models.StatusDraft},
		},
		Photos: []models.Photo{
			{ID: "public-photo", URL: "https://example.test/public.jpg", Status: models.StatusPublished},
			{ID: "draft-photo", URL: "https://example.test/unreleased-photo.jpg", Status: models.StatusDraft},
		},
		StorySections: []models.StorySection{
			{ID: "public-story", Title: "Our Story", Body: "Published", Status: models.StatusPublished},
			{ID: "draft-story", Title: "unreleased-story", Body: "Private", Status: models.StatusDraft},
		},
		Announcements: []models.Announcement{
			{ID: "public-announcement", Title: "Welcome", Body: "Published", Status: models.StatusPublished},
			{ID: "draft-announcement", Title: "unreleased-announcement", Body: "Private", Status: models.StatusDraft},
		},
		RSVPs:         []models.RSVP{{InvitationID: "another-invitation", Status: models.RSVPAttending, DietaryNotes: "private-rsvp"}},
		GuestMessages: []models.GuestMessage{{ID: "private-message", Body: "private-message-body"}},
		CreatedAt:     now, UpdatedAt: now,
	}
	if _, err := repo.CreateWedding(wedding); err != nil {
		t.Fatal(err)
	}
	token, hash, err := models.NewOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.AddInvitation("w1", models.Invitation{ID: "i1", GuestName: "Taylor", MaxPartySize: 2, Status: models.InvitationPending, TokenHash: hash, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	NewWithAllowedOrigin(repo, "").ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/invitations/"+token, nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
	body := response.Body.String()
	for _, privateValue := range []string{hash, "private-admin", "private-guest", "private-rsvp", "private-message-body", "unreleased-event", "unreleased-photo", "unreleased-story", "unreleased-announcement"} {
		if strings.Contains(body, privateValue) {
			t.Fatalf("public invitation leaked %q: %s", privateValue, body)
		}
	}
	var rawView struct {
		Wedding map[string]json.RawMessage `json:"wedding"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &rawView); err != nil {
		t.Fatal(err)
	}
	for _, forbiddenKey := range []string{"status", "admins", "guests", "invitations", "rsvps", "guest_messages", "created_at", "updated_at"} {
		if _, exists := rawView.Wedding[forbiddenKey]; exists {
			t.Fatalf("public wedding contains forbidden key %q", forbiddenKey)
		}
	}
	var view invitationView
	if err := json.Unmarshal(response.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if view.Wedding.Venue != "Garden" || view.Wedding.Address != "1 Main St" || view.Wedding.City != "Nairobi" || view.Wedding.State != "Nairobi County" || view.Wedding.Country != "Kenya" {
		t.Fatalf("missing location fields: %#v", view.Wedding)
	}
	if view.Wedding.Message != "Celebrate with us" || view.Wedding.Verse != "Love never fails" || view.Wedding.DressCode != "Formal" || view.Wedding.HeroImage == "" || view.Wedding.TemplateID != "classic" {
		t.Fatalf("missing guest-facing fields: %#v", view.Wedding)
	}
	if len(view.Wedding.Events) != 1 || len(view.Wedding.Photos) != 1 || len(view.Wedding.StorySections) != 1 || len(view.Wedding.Announcements) != 1 {
		t.Fatalf("projection did not include only published content: %#v", view.Wedding)
	}
}

func TestGuestMessagesRequireAcceptedInvitationAndAreWeddingScoped(t *testing.T) {
	repo := repository.NewMemoryRepository()
	now := time.Now().UTC()
	if _, err := repo.CreateWedding(models.Wedding{ID: "w1", Slug: "one", Title: "One", Status: models.StatusPublished, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	token, hash, err := models.NewOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.AddInvitation("w1", models.Invitation{ID: "i1", GuestName: "Taylor", MaxPartySize: 1, Status: models.InvitationPending, TokenHash: hash, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	handler := NewWithAllowedOrigin(repo, "")
	path := "/api/guest/" + token + "/messages"

	pending := httptest.NewRecorder()
	handler.ServeHTTP(pending, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"message":"Hello"}`)))
	if pending.Code != http.StatusForbidden {
		t.Fatalf("pending status = %d, body = %s", pending.Code, pending.Body)
	}
	if _, _, err = repo.RespondToInvitation(hash, models.InvitationAccepted, now); err != nil {
		t.Fatal(err)
	}

	createdResponse := httptest.NewRecorder()
	handler.ServeHTTP(createdResponse, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"message":"  Best wishes!  "}`)))
	if createdResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createdResponse.Code, createdResponse.Body)
	}
	var created models.GuestMessage
	if err := json.Unmarshal(createdResponse.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.WeddingID != "w1" || created.InvitationID != "i1" || created.Body != "Best wishes!" || created.CreatedAt.IsZero() {
		t.Fatalf("unexpected message: %#v", created)
	}
	stored, err := repo.GetWedding("w1")
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.GuestMessages) != 1 || stored.GuestMessages[0] != created {
		t.Fatalf("message not stored in wedding aggregate: %#v", stored.GuestMessages)
	}

	bodyAlias := httptest.NewRecorder()
	handler.ServeHTTP(bodyAlias, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"body":"Thank you"}`)))
	if bodyAlias.Code != http.StatusCreated {
		t.Fatalf("body alias status = %d, body = %s", bodyAlias.Code, bodyAlias.Body)
	}
	for name, payload := range map[string]string{
		"both aliases":  `{"message":"one","body":"two"}`,
		"empty":         `{"message":"   "}`,
		"unknown field": `{"message":"hello","private":true}`,
		"too long":      `{"message":"` + strings.Repeat("x", maxGuestMessageLength+1) + `"}`,
	} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, strings.NewReader(payload)))
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body)
			}
		})
	}
}

func TestDevelopmentCORS(t *testing.T) {
	handler := NewWithAllowedOrigin(repository.NewMemoryRepository(), "")
	preflight := httptest.NewRequest(http.MethodOptions, "/api/weddings", nil)
	preflight.Header.Set("Origin", "http://localhost:5173")
	preflight.Header.Set("Access-Control-Request-Method", http.MethodPost)
	preflight.Header.Set("Access-Control-Request-Headers", "Content-Type")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, preflight)
	if response.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, body = %s", response.Code, response.Body)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("allow origin = %q", got)
	}
	if got := response.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, http.MethodDelete) {
		t.Fatalf("allow methods = %q", got)
	}

	disallowed := httptest.NewRequest(http.MethodOptions, "/api/weddings", nil)
	disallowed.Header.Set("Origin", "https://evil.example")
	disallowed.Header.Set("Access-Control-Request-Method", http.MethodGet)
	disallowedResponse := httptest.NewRecorder()
	handler.ServeHTTP(disallowedResponse, disallowed)
	if disallowedResponse.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("disallowed origin received CORS headers")
	}

	badMethod := httptest.NewRequest(http.MethodOptions, "/api/weddings", nil)
	badMethod.Header.Set("Origin", "http://127.0.0.1:3000")
	badMethod.Header.Set("Access-Control-Request-Method", http.MethodPatch)
	badMethodResponse := httptest.NewRecorder()
	handler.ServeHTTP(badMethodResponse, badMethod)
	if badMethodResponse.Code != http.StatusForbidden {
		t.Fatalf("unsupported method status = %d", badMethodResponse.Code)
	}
}

func TestConfiguredCORSUsesExactOrigins(t *testing.T) {
	handler := NewWithAllowedOrigin(repository.NewMemoryRepository(), "https://app.example, http://localhost:9000")
	for _, test := range []struct {
		origin  string
		allowed bool
	}{
		{origin: "https://app.example", allowed: true},
		{origin: "http://localhost:9000", allowed: true},
		{origin: "http://localhost:9001", allowed: false},
		{origin: "https://app.example.evil.test", allowed: false},
	} {
		t.Run(test.origin, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			request.Header.Set("Origin", test.origin)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			got := response.Header().Get("Access-Control-Allow-Origin")
			if (got != "") != test.allowed {
				t.Fatalf("allow origin = %q, allowed = %v", got, test.allowed)
			}
		})
	}
}

func TestInvitationAcceptAndRSVP(t *testing.T) {
	repo := repository.NewMemoryRepository()
	handler := New(repo)
	createBody := bytes.NewBufferString(`{"slug":"a-b","title":"A & B","status":"draft"}`)
	createResponse := httptest.NewRecorder()
	handler.ServeHTTP(createResponse, httptest.NewRequest(http.MethodPost, "/api/weddings", createBody))
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", createResponse.Code, createResponse.Body)
	}
	var createdWedding weddingCreated
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createdWedding); err != nil {
		t.Fatal(err)
	}
	if createdWedding.AdminToken == "" {
		t.Fatal("create did not return a wedding admin token")
	}
	wedding := createdWedding.Wedding

	// Admin-authenticated invitation creation.
	editor := httptest.NewRecorder()
	handler.ServeHTTP(editor, httptest.NewRequest(http.MethodPost, "/api/weddings/"+wedding.ID+"/invitations", bytes.NewBufferString(`{"guest_name":"Taylor","max_party_size":2}`)))
	if editor.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated invite status = %d (expected 401)", editor.Code)
	}

	inviteBody := bytes.NewBufferString(`{"guest_name":"Taylor","max_party_size":2}`)
	inviteResponse := httptest.NewRecorder()
	inviteRequest := httptest.NewRequest(http.MethodPost, "/api/weddings/"+wedding.ID+"/invitations", inviteBody)
	inviteRequest.Header.Set("Authorization", "Bearer "+createdWedding.AdminToken)
	handler.ServeHTTP(inviteResponse, inviteRequest)
	if inviteResponse.Code != http.StatusCreated {
		t.Fatalf("invite status = %d: %s", inviteResponse.Code, inviteResponse.Body)
	}
	var created invitationCreated
	if err := json.Unmarshal(inviteResponse.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Token == "" {
		t.Fatal("creation did not return token")
	}

	// Role validation: a guest invitation cannot be accepted as a committee member.
	mismatch := httptest.NewRecorder()
	handler.ServeHTTP(mismatch, httptest.NewRequest(http.MethodPost, "/api/invitations/"+created.Token+"/accept", bytes.NewBufferString(`{"role":"committee"}`)))
	if mismatch.Code != http.StatusForbidden {
		t.Fatalf("role mismatch accept status = %d: %s", mismatch.Code, mismatch.Body)
	}

	acceptResponse := httptest.NewRecorder()
	handler.ServeHTTP(acceptResponse, httptest.NewRequest(http.MethodPost, "/api/invitations/"+created.Token+"/accept", nil))
	if acceptResponse.Code != http.StatusOK {
		t.Fatalf("accept status = %d: %s", acceptResponse.Code, acceptResponse.Body)
	}

	rsvpResponse := httptest.NewRecorder()
	handler.ServeHTTP(rsvpResponse, httptest.NewRequest(http.MethodPut, "/api/guest/"+created.Token+"/rsvp", bytes.NewBufferString(`{"status":"attending","party_size":2}`)))
	if rsvpResponse.Code != http.StatusOK {
		t.Fatalf("rsvp status = %d: %s", rsvpResponse.Code, rsvpResponse.Body)
	}
}

func TestAdminOverviewCountsGuestsAndCommitteeSeparately(t *testing.T) {
	repo := repository.NewMemoryRepository()
	handler := New(repo)
	createResponse := httptest.NewRecorder()
	handler.ServeHTTP(createResponse, httptest.NewRequest(http.MethodPost, "/api/weddings", bytes.NewBufferString(`{"slug":"overview-wed","title":"A & B","status":"published"}`)))
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", createResponse.Code, createResponse.Body)
	}
	var created weddingCreated
	if err := json.Unmarshal(createResponse.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	editor := func(method, path string, body string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		request.Header.Set("Authorization", "Bearer "+created.AdminToken)
		handler.ServeHTTP(response, request)
		return response
	}

	if response := editor(http.MethodPost, "/api/weddings/"+created.Wedding.ID+"/invitations", `{"type":"guest","guest_name":"Taylor","max_party_size":2}`); response.Code != http.StatusCreated {
		t.Fatalf("guest invite status = %d: %s", response.Code, response.Body)
	}
	if response := editor(http.MethodPost, "/api/weddings/"+created.Wedding.ID+"/invitations", `{"type":"committee","guest_name":"Sam","committee_title":"Coordinator"}`); response.Code != http.StatusCreated {
		t.Fatalf("committee invite status = %d: %s", response.Code, response.Body)
	}

	response := editor(http.MethodGet, "/api/weddings/"+created.Wedding.ID+"/admin/overview", "")
	if response.Code != http.StatusOK {
		t.Fatalf("overview status = %d: %s", response.Code, response.Body)
	}
	var view overviewView
	if err := json.Unmarshal(response.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if view.InvitationsTotal != 1 {
		t.Fatalf("invitations_total = %d, want 1 (guests only, not double-counted)", view.InvitationsTotal)
	}
	if view.InvitationsPending != 1 || view.CommitteeTotal != 1 || view.CommitteePending != 1 {
		t.Fatalf("unexpected overview counts: %+v", view)
	}
}

func TestCommitteeAuthorizesMembersAndRejectsGuests(t *testing.T) {
	repo := repository.NewMemoryRepository()
	handler := New(repo)
	createBody := bytes.NewBufferString(`{"slug":"committee-wed","title":"A & B","status":"published"}`)
	createResponse := httptest.NewRecorder()
	handler.ServeHTTP(createResponse, httptest.NewRequest(http.MethodPost, "/api/weddings", createBody))
	var created weddingCreated
	if err := json.Unmarshal(createResponse.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	wedding := created.Wedding
	adminToken := created.AdminToken

	createInvite := func(name, inviteType string) (string, string) {
		payload := `{"guest_name":"` + name + `","type":"` + inviteType + `","committee_title":"Coordinator","max_party_size":2}`
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/weddings/"+wedding.ID+"/invitations", bytes.NewBufferString(payload))
		request.Header.Set("Authorization", "Bearer "+adminToken)
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusCreated {
			t.Fatalf("create %s invitation status = %d: %s", inviteType, response.Code, response.Body)
		}
		var createdIn invitationCreated
		if err := json.Unmarshal(response.Body.Bytes(), &createdIn); err != nil {
			t.Fatal(err)
		}
		return createdIn.Token, createdIn.Invitation.ID
	}

	committeeToken, committeeInvitationID := createInvite("Committed Member", "committee")
	guestToken, _ := createInvite("Casual Guest", "guest")

	// Accepting a committee invitation records a committee member.
	accept := httptest.NewRecorder()
	handler.ServeHTTP(accept, httptest.NewRequest(http.MethodPost, "/api/invitations/"+committeeToken+"/accept", bytes.NewBufferString(`{"role":"committee"}`)))
	if accept.Code != http.StatusOK {
		t.Fatalf("committee accept status = %d: %s", accept.Code, accept.Body)
	}
	stored, err := repo.GetWedding(wedding.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.CommitteeMembers) != 1 || stored.CommitteeMembers[0].InvitationID != committeeInvitationID {
		t.Fatalf("committee member not materialized: %#v", stored.CommitteeMembers)
	}

	committeePath := "/api/weddings/" + wedding.ID + "/committee/dashboard"

	// A pending committee member cannot access the workspace.
	pendingToken, _ := createInvite("Pending Member", "committee")
	pending := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, committeePath, nil)
	request.Header.Set("Authorization", "Bearer "+pendingToken)
	handler.ServeHTTP(pending, request)
	if pending.Code != http.StatusForbidden {
		t.Fatalf("pending committee access status = %d", pending.Code)
	}

	// An accepted committee member can.
	member := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, committeePath, nil)
	request.Header.Set("Authorization", "Bearer "+committeeToken)
	handler.ServeHTTP(member, request)
	if member.Code != http.StatusOK {
		t.Fatalf("committee member access status = %d: %s", member.Code, member.Body)
	}

	// A guest token is rejected on the committee route.
	guest := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, committeePath, nil)
	request.Header.Set("Authorization", "Bearer "+guestToken)
	handler.ServeHTTP(guest, request)
	if guest.Code != http.StatusForbidden {
		t.Fatalf("guest committee access status = %d (expected 403)", guest.Code)
	}

	// The accepted committee member cannot act as an admin.
	roster := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/weddings/"+wedding.ID+"/admin/roster", nil)
	request.Header.Set("Authorization", "Bearer "+committeeToken)
	handler.ServeHTTP(roster, request)
	if roster.Code != http.StatusForbidden {
		t.Fatalf("committee member trying admin roster status = %d (expected 403)", roster.Code)
	}

	chat := httptest.NewRecorder()
	chatRequest := httptest.NewRequest(http.MethodGet, "/api/weddings/"+wedding.ID+"/committee/chat", nil)
	chatRequest.Header.Set("Authorization", "Bearer "+committeeToken)
	handler.ServeHTTP(chat, chatRequest)
	if chat.Code != http.StatusOK {
		t.Fatalf("committee chat status = %d", chat.Code)
	}
	var before []models.CommitteeMessage
	if err := json.Unmarshal(chat.Body.Bytes(), &before); err != nil {
		t.Fatal(err)
	}
	send := httptest.NewRecorder()
	sendRequest := httptest.NewRequest(http.MethodPost, "/api/weddings/"+wedding.ID+"/committee/chat", bytes.NewBufferString(`{"message":"Decor is confirmed."}`))
	sendRequest.Header.Set("Authorization", "Bearer "+committeeToken)
	handler.ServeHTTP(send, sendRequest)
	if send.Code != http.StatusCreated {
		t.Fatalf("send chat status = %d: %s", send.Code, send.Body)
	}
	chat2 := httptest.NewRecorder()
	chat2Request := httptest.NewRequest(http.MethodGet, "/api/weddings/"+wedding.ID+"/committee/chat", nil)
	chat2Request.Header.Set("Authorization", "Bearer "+committeeToken)
	handler.ServeHTTP(chat2, chat2Request)
	var after []models.CommitteeMessage
	if err := json.Unmarshal(chat2.Body.Bytes(), &after); err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before)+1 || after[len(after)-1].AuthorName != "Committed Member" || after[len(after)-1].Body != "Decor is confirmed." {
		t.Fatalf("unexpected chat history: %#v", after)
	}
}

func TestGuestDashboardExcludesCommitteeAnnouncements(t *testing.T) {
	repo := repository.NewMemoryRepository()
	handler := New(repo)
	now := time.Now().UTC()
	token, hash, err := models.NewOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	_ = hash
	wedding := models.Wedding{ID: "w1", Slug: "public-only", Title: "Public Only", Status: models.StatusPublished,
		Announcements: []models.Announcement{
			{ID: "public-note", Title: "Welcome", Body: "Public", Status: models.StatusPublished, Audience: models.AudiencePublic},
			{ID: "committee-note", Title: "Rehearsal", Body: "Committee only", Status: models.StatusPublished, Audience: models.AudienceCommittee},
		}, CreatedAt: now, UpdatedAt: now}
	if _, err := repo.CreateWedding(wedding); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.AddInvitation("w1", models.Invitation{ID: "i1", GuestName: "Taylor", MaxPartySize: 2, Status: models.InvitationPending, TokenHash: hash, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if _, _, err = repo.RespondToInvitation(hash, models.InvitationAccepted, now); err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/guest/"+token+"/dashboard", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body)
	}
	body := response.Body.String()
	if strings.Contains(body, "committee-note") || strings.Contains(body, "Committee only") {
		t.Fatal("guest dashboard leaked committee-only announcement")
	}
	if !strings.Contains(body, "public-note") {
		t.Fatal("guest dashboard missing public announcement")
	}
}

func TestTemplatesAndFontsCatalogs(t *testing.T) {
	repo := repository.NewMemoryRepository()
	handler := New(repo)

	// Test GET /api/templates
	templatesResp := httptest.NewRecorder()
	handler.ServeHTTP(templatesResp, httptest.NewRequest(http.MethodGet, "/api/templates", nil))
	if templatesResp.Code != http.StatusOK {
		t.Fatalf("GET /api/templates status = %d: %s", templatesResp.Code, templatesResp.Body)
	}
	var templates []models.Template
	if err := json.Unmarshal(templatesResp.Body.Bytes(), &templates); err != nil {
		t.Fatalf("failed to decode templates: %v", err)
	}
	if len(templates) == 0 {
		t.Fatal("expected at least 1 template")
	}

	// Test GET /api/fonts
	fontsResp := httptest.NewRecorder()
	handler.ServeHTTP(fontsResp, httptest.NewRequest(http.MethodGet, "/api/fonts", nil))
	if fontsResp.Code != http.StatusOK {
		t.Fatalf("GET /api/fonts status = %d: %s", fontsResp.Code, fontsResp.Body)
	}
	var fonts []models.FontItem
	if err := json.Unmarshal(fontsResp.Body.Bytes(), &fonts); err != nil {
		t.Fatalf("failed to decode fonts: %v", err)
	}
	if len(fonts) < 5 {
		t.Fatalf("expected fonts catalog, got %d items", len(fonts))
	}
}

func TestCardConfigAndCommitteeRoles(t *testing.T) {
	repo := repository.NewMemoryRepository()
	handler := New(repo)
	now := time.Now().UTC()
	adminToken, adminHash, err := models.NewOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}

	wedding := models.Wedding{
		ID:             "w_card_test",
		Slug:           "card-test-wedding",
		Title:          "Card Test Wedding",
		Status:         models.StatusPublished,
		AdminTokenHash: adminHash,
		TemplateID:     "luxury-sage-download",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if _, err := repo.CreateWedding(wedding); err != nil {
		t.Fatal(err)
	}

	// 1. GET card config default
	getCardReq := httptest.NewRequest(http.MethodGet, "/api/weddings/w_card_test/card", nil)
	getCardResp := httptest.NewRecorder()
	handler.ServeHTTP(getCardResp, getCardReq)
	if getCardResp.Code != http.StatusOK {
		t.Fatalf("GET card config status = %d: %s", getCardResp.Code, getCardResp.Body)
	}

	// 2. PUT card config
	newCardConfig := models.CardConfig{
		TemplateID: "royal-black-gold",
		Fonts: models.CardFonts{
			Couple:  "Allura",
			Heading: "Cormorant Garamond",
			Body:    "Montserrat",
		},
		Colors: models.CardColors{
			Background: "#111111",
			Text:       "#ffffff",
			Accent:     "#ffd700",
			Border:     "#ffd700",
		},
		Decorations: models.CardDecorations{
			FloralStyle: "luxury-gold",
			BorderStyle: "double-gold",
			Layout:      "centered-classic",
		},
	}
	payload, _ := json.Marshal(newCardConfig)
	putCardReq := httptest.NewRequest(http.MethodPut, "/api/weddings/w_card_test/card", bytes.NewReader(payload))
	putCardReq.Header.Set("Authorization", "Bearer "+adminToken)
	putCardReq.Header.Set("Content-Type", "application/json")
	putCardResp := httptest.NewRecorder()
	handler.ServeHTTP(putCardResp, putCardReq)
	if putCardResp.Code != http.StatusOK {
		t.Fatalf("PUT card config status = %d: %s", putCardResp.Code, putCardResp.Body)
	}

	// 3. POST committee role
	newRole := models.CommitteeRole{
		Name:        "Decoration Lead",
		Description: "Coordinates flower arrangements and hall decoration",
	}
	rolePayload, _ := json.Marshal(newRole)
	createRoleReq := httptest.NewRequest(http.MethodPost, "/api/weddings/w_card_test/committee/roles", bytes.NewReader(rolePayload))
	createRoleReq.Header.Set("Authorization", "Bearer "+adminToken)
	createRoleReq.Header.Set("Content-Type", "application/json")
	createRoleResp := httptest.NewRecorder()
	handler.ServeHTTP(createRoleResp, createRoleReq)
	if createRoleResp.Code != http.StatusCreated {
		t.Fatalf("POST committee role status = %d: %s", createRoleResp.Code, createRoleResp.Body)
	}

	var createdRole models.CommitteeRole
	if err := json.Unmarshal(createRoleResp.Body.Bytes(), &createdRole); err != nil {
		t.Fatal(err)
	}
	if createdRole.Name != "Decoration Lead" || createdRole.ID == "" {
		t.Fatalf("unexpected created role: %+v", createdRole)
	}

	// 4. GET committee roles
	getRolesReq := httptest.NewRequest(http.MethodGet, "/api/weddings/w_card_test/committee/roles", nil)
	getRolesReq.Header.Set("Authorization", "Bearer "+adminToken)
	getRolesResp := httptest.NewRecorder()
	handler.ServeHTTP(getRolesResp, getRolesReq)
	if getRolesResp.Code != http.StatusOK {
		t.Fatalf("GET committee roles status = %d: %s", getRolesResp.Code, getRolesResp.Body)
	}

	// 5. DELETE committee role
	deleteRoleReq := httptest.NewRequest(http.MethodDelete, "/api/weddings/w_card_test/committee/roles/"+createdRole.ID, nil)
	deleteRoleReq.Header.Set("Authorization", "Bearer "+adminToken)
	deleteRoleResp := httptest.NewRecorder()
	handler.ServeHTTP(deleteRoleResp, deleteRoleReq)
	if deleteRoleResp.Code != http.StatusNoContent {
		t.Fatalf("DELETE committee role status = %d: %s", deleteRoleResp.Code, deleteRoleResp.Body)
	}
}

// recordingSender is a delivery.Sender that records what the API asked it to send.
type recordingSender struct {
	channels []string
	link     string
	err      error
	sent     []delivery.Invitation
}

func (s *recordingSender) Channels() []string { return s.channels }

func (s *recordingSender) Link(string) string { return s.link }

func (s *recordingSender) Send(_ context.Context, invitation delivery.Invitation) error {
	if s.err != nil {
		return s.err
	}
	s.sent = append(s.sent, invitation)
	return nil
}

// newSendTestWedding creates a wedding and one invitation through the admin API so the
// test has both an admin capability token and the invitation's one-time raw token.
func newSendTestWedding(t *testing.T, handler http.Handler, invitationPayload string) (weddingCreated, invitationCreated) {
	t.Helper()
	create := httptest.NewRecorder()
	handler.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/api/weddings", bytes.NewBufferString(`{"slug":"send-wed","title":"A & B","status":"published"}`)))
	if create.Code != http.StatusCreated {
		t.Fatalf("create wedding status = %d: %s", create.Code, create.Body)
	}
	var wedding weddingCreated
	if err := json.Unmarshal(create.Body.Bytes(), &wedding); err != nil {
		t.Fatal(err)
	}
	invite := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/weddings/"+wedding.Wedding.ID+"/invitations", bytes.NewBufferString(invitationPayload))
	request.Header.Set("Authorization", "Bearer "+wedding.AdminToken)
	handler.ServeHTTP(invite, request)
	if invite.Code != http.StatusCreated {
		t.Fatalf("create invitation status = %d: %s", invite.Code, invite.Body)
	}
	var invitation invitationCreated
	if err := json.Unmarshal(invite.Body.Bytes(), &invitation); err != nil {
		t.Fatal(err)
	}
	return wedding, invitation
}

func TestSendInvitationDeliversAndValidatesToken(t *testing.T) {
	repo := repository.NewMemoryRepository()
	sender := &recordingSender{channels: []string{delivery.ChannelEmail, delivery.ChannelWhatsApp}}
	handler := NewWithSender(repo, "", sender)
	wedding, invitation := newSendTestWedding(t, handler,
		`{"type":"guest","guest_name":"Taylor","guest_email":"taylor@example.com","guest_phone":"+15551234567","max_party_size":2}`)
	path := "/api/weddings/" + wedding.Wedding.ID + "/invitations/" + invitation.Invitation.ID + "/send"

	call := func(body string, admin bool) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
		if admin {
			request.Header.Set("Authorization", "Bearer "+wedding.AdminToken)
		}
		handler.ServeHTTP(response, request)
		return response
	}

	if response := call(`{"token":"`+invitation.Token+`"}`, false); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d: %s", response.Code, response.Body)
	}
	if response := call(`{"token":"`+invitation.Token+`","channels":["carrier-pigeon"]}`, true); response.Code != http.StatusBadRequest {
		t.Fatalf("unknown channel status = %d: %s", response.Code, response.Body)
	}
	if response := call(`{"token":"   ","channels":["email"]}`, true); response.Code != http.StatusBadRequest {
		t.Fatalf("blank token status = %d: %s", response.Code, response.Body)
	}
	otherToken, _, err := models.NewOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	if response := call(`{"token":"`+otherToken+`","channels":["email"]}`, true); response.Code != http.StatusForbidden {
		t.Fatalf("mismatched token status = %d: %s", response.Code, response.Body)
	}
	missing := httptest.NewRecorder()
	missingRequest := httptest.NewRequest(http.MethodPost, "/api/weddings/"+wedding.Wedding.ID+"/invitations/missing/send",
		bytes.NewBufferString(`{"token":"`+invitation.Token+`","channels":["email"]}`))
	missingRequest.Header.Set("Authorization", "Bearer "+wedding.AdminToken)
	handler.ServeHTTP(missing, missingRequest)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing invitation status = %d: %s", missing.Code, missing.Body)
	}

	if response := call(`{"token":"`+invitation.Token+`","channels":["email"]}`, true); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing base url status = %d: %s", response.Code, response.Body)
	}
	sender.link = "https://wedding.example/pages/event.html?token="

	response := call(`{"token":"`+invitation.Token+`","channels":["email","whatsapp"]}`, true)
	if response.Code != http.StatusOK {
		t.Fatalf("send status = %d: %s", response.Code, response.Body)
	}
	var view sendInvitationResponse
	if err := json.Unmarshal(response.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if len(view.Results) != 2 || view.Results[0].Status != "sent" || view.Results[1].Status != "sent" {
		t.Fatalf("results = %#v", view.Results)
	}
	if len(sender.sent) != 2 {
		t.Fatalf("sender delivered %d messages, want 2", len(sender.sent))
	}
	if sender.sent[0].Channel != delivery.ChannelEmail || sender.sent[0].To != "taylor@example.com" || sender.sent[0].Name != "Taylor" || sender.sent[0].Couple != "A & B" {
		t.Fatalf("email copy = %#v", sender.sent[0])
	}
	if sender.sent[1].Channel != delivery.ChannelWhatsApp || sender.sent[1].To != "+15551234567" {
		t.Fatalf("whatsapp copy = %#v", sender.sent[1])
	}
}

func TestSendInvitationReportsSkippedAndFailedChannels(t *testing.T) {
	repo := repository.NewMemoryRepository()
	sender := &recordingSender{channels: []string{delivery.ChannelEmail}, link: "https://wedding.example/e"}
	handler := NewWithSender(repo, "", sender)
	// No guest_email, so the email channel has no recipient on file.
	wedding, invitation := newSendTestWedding(t, handler,
		`{"type":"guest","guest_name":"Taylor","guest_phone":"+15551234567","max_party_size":2}`)
	path := "/api/weddings/" + wedding.Wedding.ID + "/invitations/" + invitation.Invitation.ID + "/send"
	call := func() *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, path,
			bytes.NewBufferString(`{"token":"`+invitation.Token+`","channels":["email","whatsapp"]}`))
		request.Header.Set("Authorization", "Bearer "+wedding.AdminToken)
		handler.ServeHTTP(response, request)
		return response
	}

	response := call()
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body)
	}
	var view sendInvitationResponse
	if err := json.Unmarshal(response.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if view.Results[0].Status != "skipped" || view.Results[0].Error != "no recipient on file" {
		t.Fatalf("email result = %#v", view.Results[0])
	}
	if view.Results[1].Status != "skipped" || view.Results[1].Error != "channel is not configured" {
		t.Fatalf("whatsapp result = %#v", view.Results[1])
	}
	if len(sender.sent) != 0 {
		t.Fatalf("sender should not receive skipped messages: %#v", sender.sent)
	}

	sender.err = errors.New("smtp is down")
	sender.channels = []string{delivery.ChannelEmail}
	withEmailRepo := repository.NewMemoryRepository()
	withEmailHandler := NewWithSender(withEmailRepo, "", sender)
	wedding2, invitation2 := newSendTestWedding(t, withEmailHandler,
		`{"type":"guest","guest_name":"Jordan","guest_email":"jordan@example.com","max_party_size":1}`)
	request := httptest.NewRequest(http.MethodPost, "/api/weddings/"+wedding2.Wedding.ID+"/invitations/"+invitation2.Invitation.ID+"/send",
		bytes.NewBufferString(`{"token":"`+invitation2.Token+`","channels":["email"]}`))
	request.Header.Set("Authorization", "Bearer "+wedding2.AdminToken)
	failed := httptest.NewRecorder()
	withEmailHandler.ServeHTTP(failed, request)
	if failed.Code != http.StatusOK {
		t.Fatalf("failed send status = %d: %s", failed.Code, failed.Body)
	}
	if err := json.Unmarshal(failed.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if view.Results[0].Status != "failed" || !strings.Contains(view.Results[0].Error, "smtp is down") {
		t.Fatalf("failed result = %#v", view.Results[0])
	}
}

func TestSendInvitationWithoutSenderIsUnavailable(t *testing.T) {
	repo := repository.NewMemoryRepository()
	seeded := NewWithSender(repo, "", &recordingSender{channels: []string{delivery.ChannelEmail}, link: "https://wedding.example/e"})
	wedding, invitation := newSendTestWedding(t, seeded,
		`{"type":"guest","guest_name":"Taylor","guest_email":"taylor@example.com","max_party_size":1}`)

	handler := NewWithSender(repo, "", nil)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/weddings/"+wedding.Wedding.ID+"/invitations/"+invitation.Invitation.ID+"/send",
		bytes.NewBufferString(`{"token":"`+invitation.Token+`","channels":["email"]}`))
	request.Header.Set("Authorization", "Bearer "+wedding.AdminToken)
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d: %s", response.Code, response.Body)
	}
}

// TestMain lowers the PBKDF2 work factor so the suite stays fast; the algorithm and its
// encoded format are covered by the models package tests at their own cost.
func TestMain(m *testing.M) {
	models.PasswordIterations = 1500
	os.Exit(m.Run())
}

func postJSON(handler http.Handler, path, body string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body)))
	return response
}

func TestSignupRejectsWeakCredentialsAndDuplicateEmail(t *testing.T) {
	handler := New(repository.NewMemoryRepository())

	if response := postJSON(handler, "/api/auth/signup", `{"email":"admin@example.com","password":"supersecret","display_name":"Ada"}); ;`); response.Code != http.StatusBadRequest {
		t.Fatalf("malformed body status = %d: %s", response.Code, response.Body)
	}
	for name, payload := range map[string]string{
		"short password":  `{"email":"short@example.com","password":"short"}`,
		"missing email":   `{"email":"","password":"supersecret"}`,
		"email no domain": `{"email":"admin@localhost","password":"supersecret"}`,
		"email spaces":    `{"email":"ad min@example.com","password":"supersecret"}`,
		"unknown field":   `{"email":"extra@example.com","password":"supersecret","role":"owner"}`,
	} {
		t.Run(name, func(t *testing.T) {
			response := postJSON(handler, "/api/auth/signup", payload)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d: %s", response.Code, response.Body)
			}
		})
	}

	if response := postJSON(handler, "/api/auth/signup", `{"email":"admin@example.com","password":"supersecret"}`); response.Code != http.StatusCreated {
		t.Fatalf("first signup status = %d: %s", response.Code, response.Body)
	}
	duplicate := postJSON(handler, "/api/auth/signup", `{"email":"ADMIN@example.com","password":"anothersecret"}`)
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate signup status = %d: %s", duplicate.Code, duplicate.Body)
	}
}

func TestLoginRejectsWrongPasswordAndIssuesSessions(t *testing.T) {
	handler := New(repository.NewMemoryRepository())

	signup := postJSON(handler, "/api/auth/signup", `{"email":"Admin@Example.com","password":"supersecret","display_name":"Ada Admin"}`)
	if signup.Code != http.StatusCreated {
		t.Fatalf("signup status = %d: %s", signup.Code, signup.Body)
	}
	if body := signup.Body.String(); strings.Contains(body, "pbkdf2") || strings.Contains(body, "password_hash") || strings.Contains(body, "supersecret") {
		t.Fatalf("signup response leaked password material: %s", body)
	}
	var created sessionResponse
	if err := json.Unmarshal(signup.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.SessionToken == "" || created.ExpiresAt.IsZero() {
		t.Fatalf("signup did not issue a session: %#v", created)
	}
	if created.User.Email != "admin@example.com" || created.User.DisplayName != "Ada Admin" || created.User.Role != models.RoleOwner {
		t.Fatalf("unexpected account: %#v", created.User)
	}

	// Wrong password and unknown email must both be rejected identically.
	for name, payload := range map[string]string{
		"wrong password": `{"email":"admin@example.com","password":"not-the-password"}`,
		"unknown email":  `{"email":"nobody@example.com","password":"supersecret"}`,
		"empty password": `{"email":"admin@example.com","password":""}`,
	} {
		t.Run(name, func(t *testing.T) {
			response := postJSON(handler, "/api/auth/login", payload)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d: %s", response.Code, response.Body)
			}
			if strings.Contains(response.Body.String(), "supersecret") || strings.Contains(response.Body.String(), "pbkdf2") {
				t.Fatalf("login error leaked credential material: %s", response.Body)
			}
		})
	}

	// The gate must open for the right password, and the email is case-insensitive.
	login := postJSON(handler, "/api/auth/login", `{"email":"ADMIN@example.com","password":"supersecret"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d: %s", login.Code, login.Body)
	}
	var logged sessionResponse
	if err := json.Unmarshal(login.Body.Bytes(), &logged); err != nil {
		t.Fatal(err)
	}
	if logged.SessionToken == "" || logged.SessionToken == created.SessionToken {
		t.Fatal("each login must mint a new session token")
	}

	me := func(token string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		handler.ServeHTTP(response, request)
		return response
	}
	if response := me(logged.SessionToken); response.Code != http.StatusOK {
		t.Fatalf("me status = %d: %s", response.Code, response.Body)
	}
	if response := me(""); response.Code != http.StatusUnauthorized {
		t.Fatalf("me without a token status = %d", response.Code)
	}
	if response := me("not-a-valid-token"); response.Code != http.StatusUnauthorized {
		t.Fatalf("me with a malformed token status = %d", response.Code)
	}

	// Logout invalidates only the session it was called with.
	logout := httptest.NewRecorder()
	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutRequest.Header.Set("Authorization", "Bearer "+logged.SessionToken)
	handler.ServeHTTP(logout, logoutRequest)
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d: %s", logout.Code, logout.Body)
	}
	if response := me(logged.SessionToken); response.Code != http.StatusUnauthorized {
		t.Fatalf("logged-out session still works: %d", response.Code)
	}
	if response := me(created.SessionToken); response.Code != http.StatusOK {
		t.Fatalf("the other session should survive, status = %d", response.Code)
	}
}
