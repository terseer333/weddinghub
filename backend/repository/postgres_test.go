package repository

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"weddinghub/models"
)

// newTestPostgres connects to the database named by WEDDINGHUB_TEST_DATABASE_URL and
// truncates the aggregate tables so each test starts from a clean slate. Tests are
// skipped when the variable is unset, so the default `go test ./...` stays hermetic.
func newTestPostgres(t *testing.T) *PostgresRepository {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("WEDDINGHUB_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("set WEDDINGHUB_TEST_DATABASE_URL to run PostgreSQL repository tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repo, err := NewPostgresRepository(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(func() { repo.Close() })
	_, err = repo.db.ExecContext(ctx, `TRUNCATE users, sessions, weddings, wedding_admins, committee_roles, guests,
		committee_members, invitations, events, photos, story_sections, announcements, planning_tasks,
		committee_messages, rsvps, guest_messages RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
	return repo
}

func mustWedding(t *testing.T, repo *PostgresRepository, wedding models.Wedding) models.Wedding {
	t.Helper()
	created, err := repo.CreateWedding(wedding)
	if err != nil {
		t.Fatalf("CreateWedding: %v", err)
	}
	return created
}

// TestNewPostgresRepositoryRejectsEmptyDSN keeps the fail-fast contract testable
// without a live database.
func TestNewPostgresRepositoryRejectsEmptyDSN(t *testing.T) {
	if _, err := NewPostgresRepository(context.Background(), "   "); err == nil {
		t.Fatal("expected an error for an empty connection string")
	}
}

func TestPostgresWeddingLifecycle(t *testing.T) {
	repo := newTestPostgres(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	created := mustWedding(t, repo, models.Wedding{
		ID: "w1", Slug: "alex-sam", Title: "Alex & Sam", PartnerOne: "Alex", PartnerTwo: "Sam",
		Status: models.StatusPublished, Venue: "Garden", Date: &now, AdminTokenHash: "admin-hash-1",
		CardConfig: &models.CardConfig{
			TemplateID:  "luxury-sage",
			Colors:      models.CardColors{Background: "#2d4030"},
			Decorations: models.CardDecorations{Layout: "centered-classic"},
		},
		Events:         []models.Event{{ID: "e1", Name: "Ceremony", StartsAt: now, Status: models.StatusPublished}},
		Photos:         []models.Photo{{ID: "p1", URL: "https://example.test/p.jpg", SortOrder: 1, Status: models.StatusPublished}},
		StorySections:  []models.StorySection{{ID: "s1", Title: "Our story", Body: "Once", SortOrder: 1, Status: models.StatusPublished}},
		Announcements:  []models.Announcement{{ID: "a1", Title: "Welcome", Body: "Hello", Audience: models.AudiencePublic, Status: models.StatusPublished, CreatedAt: now}},
		CommitteeRoles: []models.CommitteeRole{{ID: "r1", Name: "Coordinator", CreatedAt: now}},
		PlanningTasks:  []models.PlanningTask{{ID: "t1", Title: "Book venue", Status: models.TaskTodo, CreatedAt: now, UpdatedAt: now}},
		CommitteeChat:  []models.CommitteeMessage{{ID: "c1", AuthorName: "Ada", Body: "Hi", CreatedAt: now}},
		GuestMessages:  []models.GuestMessage{{ID: "gm1", InvitationID: "", Body: "Best wishes", CreatedAt: now}},
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	if created.Slug != "alex-sam" || created.CardConfig == nil || created.CardConfig.TemplateID != "luxury-sage" {
		t.Fatalf("wedding round trip mismatch: %+v", created)
	}
	if len(created.Events) != 1 || len(created.Photos) != 1 || len(created.StorySections) != 1 ||
		len(created.Announcements) != 1 || len(created.CommitteeRoles) != 1 || len(created.PlanningTasks) != 1 ||
		len(created.CommitteeChat) != 1 || len(created.GuestMessages) != 1 {
		t.Fatalf("child collections were not persisted: %+v", created)
	}

	// A second wedding cannot reuse the slug.
	duplicate := created
	duplicate.ID = "w2"
	duplicate.Slug = "alex-sam"
	if _, err := repo.CreateWedding(duplicate); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate slug error = %v, want ErrConflict", err)
	}

	// Admin capability hashes resolve to their wedding.
	byHash, err := repo.WeddingByAdminHash("admin-hash-1")
	if err != nil || byHash.ID != "w1" {
		t.Fatalf("WeddingByAdminHash = %+v, %v", byHash, err)
	}
	if _, err := repo.WeddingByAdminHash("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown admin hash error = %v, want ErrNotFound", err)
	}

	// UpdateWedding replaces content but keeps a card config the caller omitted.
	update := created
	update.Title = "Alex and Sam"
	update.CardConfig = nil
	update.Events = []models.Event{{ID: "e2", Name: "Reception", StartsAt: now, Status: models.StatusDraft}}
	update.UpdatedAt = now.Add(time.Minute)
	updated, err := repo.UpdateWedding(update)
	if err != nil {
		t.Fatalf("UpdateWedding: %v", err)
	}
	if updated.Title != "Alex and Sam" || len(updated.Events) != 1 || updated.Events[0].ID != "e2" {
		t.Fatalf("update did not replace content: %+v", updated)
	}
	if updated.CardConfig == nil || updated.CardConfig.TemplateID != "luxury-sage" {
		t.Fatalf("update did not preserve the existing card config: %+v", updated.CardConfig)
	}

	if list := repo.ListWeddings(); len(list) != 1 {
		t.Fatalf("ListWeddings returned %d weddings, want 1", len(list))
	}

	if err := repo.DeleteWedding("w1"); err != nil {
		t.Fatalf("DeleteWedding: %v", err)
	}
	if _, err := repo.GetWedding("w1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetWedding after delete = %v, want ErrNotFound", err)
	}
	if err := repo.DeleteWedding("w1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete error = %v, want ErrNotFound", err)
	}
}

func TestPostgresInvitationLifecycle(t *testing.T) {
	repo := newTestPostgres(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	mustWedding(t, repo, models.Wedding{ID: "w1", Slug: "one", Title: "One", Status: models.StatusPublished, AdminTokenHash: "h1", CreatedAt: now, UpdatedAt: now})

	token, hash, err := models.NewOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	if token == hash {
		t.Fatal("raw token must differ from stored hash")
	}
	if _, err := repo.AddInvitation("w1", models.Invitation{ID: "i1", Type: models.InvitationGuest, GuestName: "Taylor",
		GuestEmail: "taylor@example.com", MaxPartySize: 2, Status: models.InvitationPending, TokenHash: hash, CreatedAt: now}); err != nil {
		t.Fatalf("AddInvitation: %v", err)
	}
	if _, err := repo.AddInvitation("w1", models.Invitation{ID: "i2", GuestName: "Sam", MaxPartySize: 1,
		Status: models.InvitationPending, TokenHash: hash, CreatedAt: now}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate token hash error = %v, want ErrConflict", err)
	}
	if _, err := repo.AddInvitation("missing", models.Invitation{ID: "i3", TokenHash: "other", CreatedAt: now}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("invitation on unknown wedding error = %v, want ErrNotFound", err)
	}

	wedding, invitation, err := repo.InvitationByHash(hash)
	if err != nil || wedding.ID != "w1" || invitation.ID != "i1" {
		t.Fatalf("InvitationByHash = %+v / %+v / %v", wedding, invitation, err)
	}

	// The repository re-checks invitation state before accepting an RSVP.
	if _, _, err := repo.UpdateRSVP(hash, models.RSVP{Status: models.RSVPAttending, PartySize: 1, UpdatedAt: now}); !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("RSVP before acceptance error = %v, want ErrInvalidStatus", err)
	}
	if _, _, err := repo.AddGuestMessage(hash, models.GuestMessage{ID: "gm0", Body: "hi", CreatedAt: now}); !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("message before acceptance error = %v, want ErrInvalidStatus", err)
	}

	wedding, invitation, err = repo.RespondToInvitation(hash, models.InvitationAccepted, now)
	if err != nil {
		t.Fatalf("RespondToInvitation: %v", err)
	}
	if invitation.Status != models.InvitationAccepted || invitation.RespondedAt == nil || !invitation.RespondedAt.Equal(now) {
		t.Fatalf("accept did not record state: %+v", invitation)
	}
	if len(wedding.Guests) != 1 || wedding.Guests[0].InvitationID != "i1" || wedding.Guests[0].Name != "Taylor" {
		t.Fatalf("accept did not materialize the guest: %+v", wedding.Guests)
	}

	// RSVP upserts a single row per invitation.
	if _, rsvp, err := repo.UpdateRSVP(hash, models.RSVP{Status: models.RSVPAttending, PartySize: 2, UpdatedAt: now.Add(time.Minute)}); err != nil || rsvp.InvitationID != "i1" {
		t.Fatalf("UpdateRSVP = %+v, %v", rsvp, err)
	}
	wedding, _, err = repo.UpdateRSVP(hash, models.RSVP{Status: models.RSVPNotAttending, PartySize: 0, UpdatedAt: now.Add(2 * time.Minute)})
	if err != nil {
		t.Fatalf("second UpdateRSVP: %v", err)
	}
	if len(wedding.RSVPs) != 1 || wedding.RSVPs[0].Status != models.RSVPNotAttending {
		t.Fatalf("RSVP was not upserted: %+v", wedding.RSVPs)
	}

	wedding, message, err := repo.AddGuestMessage(hash, models.GuestMessage{ID: "gm1", Body: "Best wishes", CreatedAt: now.Add(3 * time.Minute)})
	if err != nil {
		t.Fatalf("AddGuestMessage: %v", err)
	}
	if message.WeddingID != "w1" || message.InvitationID != "i1" {
		t.Fatalf("message scope = %q / %q", message.WeddingID, message.InvitationID)
	}
	if len(wedding.GuestMessages) != 1 || wedding.GuestMessages[0].ID != "gm1" {
		t.Fatalf("message not part of the aggregate: %+v", wedding.GuestMessages)
	}

	// A content update must not wipe invitations, guests, or RSVPs.
	wedding.Title = "One & Only"
	wedding.Guests = nil
	wedding.Invitations = nil
	wedding.RSVPs = nil
	wedding.UpdatedAt = now.Add(4 * time.Minute)
	updated, err := repo.UpdateWedding(wedding)
	if err != nil {
		t.Fatalf("UpdateWedding after acceptance: %v", err)
	}
	if len(updated.Invitations) != 1 || len(updated.Guests) != 1 || len(updated.RSVPs) != 1 || len(updated.GuestMessages) != 1 {
		t.Fatalf("update did not preserve invitation state: %+v", updated)
	}

	if _, _, err := repo.InvitationByHash("not-a-real-hash"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown invitation error = %v, want ErrNotFound", err)
	}
}

func TestPostgresExpiredInvitation(t *testing.T) {
	repo := newTestPostgres(t)
	now := time.Now().UTC()
	mustWedding(t, repo, models.Wedding{ID: "w1", Slug: "exp", Title: "Exp", Status: models.StatusPublished, AdminTokenHash: "h1", CreatedAt: now, UpdatedAt: now})
	expired := now.Add(-time.Hour)
	_, hash, err := models.NewOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AddInvitation("w1", models.Invitation{ID: "i1", GuestName: "Late", MaxPartySize: 1,
		Status: models.InvitationPending, TokenHash: hash, ExpiresAt: &expired, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.InvitationByHash(hash); !errors.Is(err, ErrExpired) {
		t.Fatalf("InvitationByHash error = %v, want ErrExpired", err)
	}
	if _, _, err := repo.RespondToInvitation(hash, models.InvitationAccepted, now); !errors.Is(err, ErrExpired) {
		t.Fatalf("RespondToInvitation error = %v, want ErrExpired", err)
	}
}

func TestPostgresCommitteeWorkspace(t *testing.T) {
	repo := newTestPostgres(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	mustWedding(t, repo, models.Wedding{ID: "w1", Slug: "committee", Title: "Committee", Status: models.StatusPublished,
		AdminTokenHash: "h1", CommitteeRoles: []models.CommitteeRole{{ID: "r1", Name: "Finance", CreatedAt: now}},
		CreatedAt: now, UpdatedAt: now})

	if _, err := repo.AddCommitteeRole("w1", models.CommitteeRole{ID: "r2", Name: "Finance", CreatedAt: now}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate role name error = %v, want ErrConflict", err)
	}
	added, err := repo.AddCommitteeRole("w1", models.CommitteeRole{ID: "r3", Name: "Logistics", CreatedAt: now})
	if err != nil || added.WeddingID != "w1" {
		t.Fatalf("AddCommitteeRole = %+v, %v", added, err)
	}
	if err := repo.DeleteCommitteeRole("w1", "r3"); err != nil {
		t.Fatalf("DeleteCommitteeRole: %v", err)
	}
	if err := repo.DeleteCommitteeRole("w1", "r3"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second DeleteCommitteeRole error = %v, want ErrNotFound", err)
	}

	// A committee invitation materializes a committee member with its title.
	_, hash, err := models.NewOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AddInvitation("w1", models.Invitation{ID: "i1", Type: models.InvitationCommittee, GuestName: "Ada",
		CommitteeTitle: "Coordinator", MaxPartySize: 1, Status: models.InvitationPending, TokenHash: hash, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	wedding, _, err := repo.RespondToInvitation(hash, models.InvitationAccepted, now)
	if err != nil {
		t.Fatalf("RespondToInvitation: %v", err)
	}
	if len(wedding.CommitteeMembers) != 1 || wedding.CommitteeMembers[0].Title != "Coordinator" {
		t.Fatalf("committee member not materialized: %+v", wedding.CommitteeMembers)
	}
	memberID := wedding.CommitteeMembers[0].ID

	updatedMember, err := repo.UpdateCommitteeMember("w1", models.CommitteeMember{ID: memberID, Name: "Ada Lovelace", Title: "Lead"})
	if err != nil {
		t.Fatalf("UpdateCommitteeMember: %v", err)
	}
	if updatedMember.InvitationID != "i1" || updatedMember.JoinedAt.IsZero() || updatedMember.Name != "Ada Lovelace" {
		t.Fatalf("member update lost preserved fields: %+v", updatedMember)
	}
	if err := repo.DeleteCommitteeMember("w1", memberID); err != nil {
		t.Fatalf("DeleteCommitteeMember: %v", err)
	}
	if err := repo.DeleteCommitteeMember("w1", memberID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second DeleteCommitteeMember error = %v, want ErrNotFound", err)
	}

	// Committee chat honors the `since` filter and rejects unknown weddings.
	if _, err := repo.AddCommitteeMessage("w1", models.CommitteeMessage{ID: "c1", AuthorName: "Ada", AuthorRole: models.RoleCommitteeMember, Body: "Hi", CreatedAt: now}); err != nil {
		t.Fatalf("AddCommitteeMessage: %v", err)
	}
	if _, err := repo.AddCommitteeMessage("w1", models.CommitteeMessage{ID: "c2", AuthorName: "Ada", Body: "Update", CreatedAt: now.Add(time.Minute)}); err != nil {
		t.Fatalf("second AddCommitteeMessage: %v", err)
	}
	all, err := repo.CommitteeMessages("w1", time.Time{})
	if err != nil || len(all) != 2 || all[0].ID != "c1" {
		t.Fatalf("CommitteeMessages = %+v, %v", all, err)
	}
	recent, err := repo.CommitteeMessages("w1", now)
	if err != nil || len(recent) != 1 || recent[0].ID != "c2" {
		t.Fatalf("CommitteeMessages(since) = %+v, %v", recent, err)
	}
	if _, err := repo.CommitteeMessages("missing", time.Time{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("CommitteeMessages on unknown wedding error = %v, want ErrNotFound", err)
	}

	// Planning tasks preserve creation metadata across updates.
	task, err := repo.AddPlanningTask("w1", models.PlanningTask{ID: "t1", Title: "Book venue", Status: models.TaskTodo,
		CreatedBy: "Ada", CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatalf("AddPlanningTask: %v", err)
	}
	updatedTask, err := repo.UpdatePlanningTask("w1", models.PlanningTask{ID: task.ID, Title: "Book venue now", Status: models.TaskInProgress, UpdatedAt: now.Add(time.Minute)})
	if err != nil {
		t.Fatalf("UpdatePlanningTask: %v", err)
	}
	if updatedTask.CreatedBy != "Ada" || !updatedTask.CreatedAt.Equal(now.Truncate(time.Microsecond)) || updatedTask.Status != models.TaskInProgress {
		t.Fatalf("task update lost creation metadata: %+v", updatedTask)
	}
	if err := repo.DeletePlanningTask("w1", task.ID); err != nil {
		t.Fatalf("DeletePlanningTask: %v", err)
	}
	if err := repo.DeletePlanningTask("w1", task.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second DeletePlanningTask error = %v, want ErrNotFound", err)
	}

	// Announcements support create, update, and delete.
	announcement, err := repo.AddAnnouncement("w1", models.Announcement{ID: "a1", Title: "Rehearsal", Body: "Friday",
		Audience: models.AudienceCommittee, Status: models.StatusPublished, AuthorName: "Ada", CreatedAt: now})
	if err != nil {
		t.Fatalf("AddAnnouncement: %v", err)
	}
	if _, err := repo.UpdateAnnouncement("w1", models.Announcement{ID: announcement.ID, Title: "Rehearsal", Body: "Saturday",
		Audience: models.AudienceCommittee, Status: models.StatusPublished, AuthorName: "Ada"}); err != nil {
		t.Fatalf("UpdateAnnouncement: %v", err)
	}
	if err := repo.DeleteAnnouncement("w1", announcement.ID); err != nil {
		t.Fatalf("DeleteAnnouncement: %v", err)
	}
	if err := repo.DeleteAnnouncement("w1", announcement.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second DeleteAnnouncement error = %v, want ErrNotFound", err)
	}

	// Card config round trips and updates the wedding's template id.
	config, err := repo.UpdateCardConfig("w1", models.CardConfig{TemplateID: "sage", Colors: models.CardColors{Background: "#000"}})
	if err != nil || config.TemplateID != "sage" {
		t.Fatalf("UpdateCardConfig = %+v, %v", config, err)
	}
	stored, err := repo.GetWedding("w1")
	if err != nil || stored.CardConfig == nil || stored.TemplateID != "sage" {
		t.Fatalf("card config not persisted: %+v, %v", stored.CardConfig, err)
	}
	if _, err := repo.UpdateCardConfig("missing", models.CardConfig{TemplateID: "sage"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateCardConfig on unknown wedding error = %v, want ErrNotFound", err)
	}
}

func TestPostgresUsersAndSessions(t *testing.T) {
	repo := newTestPostgres(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	created, err := repo.CreateUser(models.User{ID: "u1", Email: "  Admin@Example.COM ", DisplayName: "Ada", PasswordHash: "stored", CreatedAt: now})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if created.Email != "admin@example.com" {
		t.Fatalf("stored email = %q, want a normalized address", created.Email)
	}
	if _, err := repo.CreateUser(models.User{ID: "u2", Email: "ADMIN@example.com"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate email error = %v, want ErrConflict", err)
	}
	if _, err := repo.CreateUser(models.User{ID: "u1", Email: "other@example.com"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate id error = %v, want ErrConflict", err)
	}
	if _, err := repo.CreateUser(models.User{ID: "", Email: "x@example.com"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("missing id error = %v, want ErrConflict", err)
	}

	lookedUp, err := repo.UserByEmail("admin@EXAMPLE.com")
	if err != nil || lookedUp.ID != "u1" {
		t.Fatalf("UserByEmail = %+v, %v", lookedUp, err)
	}
	if _, err := repo.UserByEmail("nobody@example.com"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown email error = %v, want ErrNotFound", err)
	}
	if _, err := repo.UserByID("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown id error = %v, want ErrNotFound", err)
	}

	// Expired sessions are treated as missing and dropped.
	expired := models.Session{TokenHash: "expired-hash", UserID: "u1", CreatedAt: now.Add(-2 * time.Hour), ExpiresAt: now.Add(-time.Hour)}
	if err := repo.AddSession(expired); err != nil {
		t.Fatalf("AddSession(expired): %v", err)
	}
	if _, err := repo.SessionByHash("expired-hash"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired session error = %v, want ErrNotFound", err)
	}

	live := models.Session{TokenHash: "live-hash", UserID: "u1", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	if err := repo.AddSession(live); err != nil {
		t.Fatalf("AddSession(live): %v", err)
	}
	session, err := repo.SessionByHash("live-hash")
	if err != nil || session.UserID != "u1" {
		t.Fatalf("SessionByHash = %+v, %v", session, err)
	}
	if err := repo.DeleteSession("live-hash"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if err := repo.DeleteSession("live-hash"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second DeleteSession error = %v, want ErrNotFound", err)
	}
	if err := repo.AddSession(models.Session{TokenHash: "", UserID: "u1"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("empty session hash error = %v, want ErrConflict", err)
	}
}
