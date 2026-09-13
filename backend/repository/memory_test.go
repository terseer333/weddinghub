package repository

import (
	"sync"
	"testing"
	"time"

	"weddinghub/models"
)

func TestMemoryRepositoryDefensiveCopyAndConcurrentReads(t *testing.T) {
	repo := NewMemoryRepository()
	created, err := repo.CreateWedding(models.Wedding{ID: "w1", Slug: "one", Title: "One", Status: models.StatusDraft, Events: []models.Event{{ID: "e1", Name: "Ceremony", Status: models.StatusDraft}}})
	if err != nil {
		t.Fatal(err)
	}
	created.Events[0].Name = "changed outside repository"

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, getErr := repo.GetWedding("w1")
			if getErr != nil {
				t.Error(getErr)
				return
			}
			if got.Events[0].Name != "Ceremony" {
				t.Errorf("repository value mutated: %q", got.Events[0].Name)
			}
		}()
	}
	wg.Wait()
}

func TestInvitationLifecycle(t *testing.T) {
	repo := NewMemoryRepository()
	_, err := repo.CreateWedding(models.Wedding{ID: "w1", Slug: "one", Title: "One", Status: models.StatusDraft})
	if err != nil {
		t.Fatal(err)
	}
	token, hash, err := models.NewOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	if token == hash {
		t.Fatal("raw token must differ from stored hash")
	}
	inv := models.Invitation{ID: "i1", GuestName: "Guest", MaxPartySize: 2, Status: models.InvitationPending, TokenHash: hash, CreatedAt: time.Now()}
	if _, err = repo.AddInvitation("w1", inv); err != nil {
		t.Fatal(err)
	}
	if _, _, err = repo.RespondToInvitation(hash, models.InvitationAccepted, time.Now()); err != nil {
		t.Fatal(err)
	}
	_, response, err := repo.UpdateRSVP(hash, models.RSVP{Status: models.RSVPAttending, PartySize: 2, UpdatedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if response.InvitationID != inv.ID {
		t.Fatalf("got invitation ID %q", response.InvitationID)
	}
	createdAt := time.Now().UTC()
	wedding, message, err := repo.AddGuestMessage(hash, models.GuestMessage{ID: "m1", WeddingID: "wrong", InvitationID: "wrong", Body: "Best wishes", CreatedAt: createdAt})
	if err != nil {
		t.Fatal(err)
	}
	if message.WeddingID != "w1" || message.InvitationID != "i1" {
		t.Fatalf("message scope = wedding %q, invitation %q", message.WeddingID, message.InvitationID)
	}
	if len(wedding.GuestMessages) != 1 || wedding.GuestMessages[0] != message {
		t.Fatalf("message not appended to aggregate: %#v", wedding.GuestMessages)
	}
}

func TestAddGuestMessageRejectsUnacceptedInvitation(t *testing.T) {
	repo := NewMemoryRepository()
	if _, err := repo.CreateWedding(models.Wedding{ID: "w1", Slug: "one", Title: "One", Status: models.StatusPublished}); err != nil {
		t.Fatal(err)
	}
	_, hash, err := models.NewOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.AddInvitation("w1", models.Invitation{ID: "i1", GuestName: "Guest", MaxPartySize: 1, Status: models.InvitationPending, TokenHash: hash}); err != nil {
		t.Fatal(err)
	}
	if _, _, err = repo.AddGuestMessage(hash, models.GuestMessage{ID: "m1", Body: "No access", CreatedAt: time.Now().UTC()}); err != ErrInvalidStatus {
		t.Fatalf("error = %v, want %v", err, ErrInvalidStatus)
	}
	wedding, err := repo.GetWedding("w1")
	if err != nil {
		t.Fatal(err)
	}
	if len(wedding.GuestMessages) != 0 {
		t.Fatalf("unauthorized message was stored: %#v", wedding.GuestMessages)
	}
}

func TestCardConfigAndCommitteeRoleRepository(t *testing.T) {
	repo := NewMemoryRepository()
	_, err := repo.CreateWedding(models.Wedding{ID: "w2", Slug: "two", Title: "Two", Status: models.StatusPublished})
	if err != nil {
		t.Fatal(err)
	}

	// Test UpdateCardConfig
	cfg := models.CardConfig{
		TemplateID: "luxury-sage-download",
		Fonts:      models.CardFonts{Couple: "Great Vibes", Heading: "Playfair Display", Body: "Cormorant Garamond"},
		Colors:     models.CardColors{Background: "#2d4030", Text: "#f7f4ed", Accent: "#d4af37"},
	}
	savedCfg, err := repo.UpdateCardConfig("w2", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if savedCfg.TemplateID != "luxury-sage-download" {
		t.Fatalf("unexpected saved template id: %s", savedCfg.TemplateID)
	}

	// Verify wedding has card config and template ID updated
	w, err := repo.GetWedding("w2")
	if err != nil {
		t.Fatal(err)
	}
	if w.CardConfig == nil || w.CardConfig.TemplateID != "luxury-sage-download" || w.TemplateID != "luxury-sage-download" {
		t.Fatalf("wedding card config not persisted properly: %+v", w.CardConfig)
	}

	// Test AddCommitteeRole
	role := models.CommitteeRole{ID: "r1", Name: "Finance Director"}
	savedRole, err := repo.AddCommitteeRole("w2", role)
	if err != nil {
		t.Fatal(err)
	}
	if savedRole.Name != "Finance Director" {
		t.Fatalf("unexpected role name: %s", savedRole.Name)
	}

	// Duplicate role should conflict
	if _, err := repo.AddCommitteeRole("w2", role); err != ErrConflict {
		t.Fatalf("expected ErrConflict, got %v", err)
	}

	// Delete role
	if err := repo.DeleteCommitteeRole("w2", "r1"); err != nil {
		t.Fatal(err)
	}
	wAfter, _ := repo.GetWedding("w2")
	if len(wAfter.CommitteeRoles) != 0 {
		t.Fatalf("expected 0 committee roles, got %d", len(wAfter.CommitteeRoles))
	}
}
