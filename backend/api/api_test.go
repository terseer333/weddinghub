package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
	var wedding models.Wedding
	if err := json.Unmarshal(createResponse.Body.Bytes(), &wedding); err != nil {
		t.Fatal(err)
	}

	inviteBody := bytes.NewBufferString(`{"guest_name":"Taylor","max_party_size":2}`)
	inviteResponse := httptest.NewRecorder()
	handler.ServeHTTP(inviteResponse, httptest.NewRequest(http.MethodPost, "/api/weddings/"+wedding.ID+"/invitations", inviteBody))
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
