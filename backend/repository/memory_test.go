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
