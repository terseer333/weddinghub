package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"weddinghub/models"
	"weddinghub/repository"
)

const (
	maxBodyBytes          = 1 << 20
	maxGuestMessageLength = 2000
)

type API struct {
	repo repository.Repository
	now  func() time.Time
}

func New(repo repository.Repository) http.Handler {
	configured := strings.TrimSpace(os.Getenv("WEDDINGHUB_ALLOWED_ORIGINS"))
	if configured == "" {
		configured = strings.TrimSpace(os.Getenv("WEDDINGHUB_ALLOWED_ORIGIN"))
	}
	return newHandler(repo, configured, configured == "")
}

// NewWithAllowedOrigin builds a handler with an exact comma-separated origin allowlist.
// An empty allowlist retains the development default of permitting loopback origins.
func NewWithAllowedOrigin(repo repository.Repository, allowedOrigins string) http.Handler {
	return newHandler(repo, allowedOrigins, strings.TrimSpace(allowedOrigins) == "")
}

func newHandler(repo repository.Repository, allowedOrigins string, allowLoopback bool) http.Handler {
	a := &API{repo: repo, now: func() time.Time { return time.Now().UTC() }}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("GET /api/weddings", a.listWeddings)
	mux.HandleFunc("POST /api/weddings", a.createWedding)
	// Admin-authenticated routes. requireAdmin resolves the wedding and verifies the
	// caller holds the wedding's admin capability token before any data is touched.
	mux.HandleFunc("GET /api/weddings/{weddingID}", a.getWedding)
	mux.HandleFunc("PUT /api/weddings/{weddingID}", a.updateWedding)
	mux.HandleFunc("DELETE /api/weddings/{weddingID}", a.deleteWedding)
	mux.HandleFunc("POST /api/weddings/{weddingID}/invitations", a.createInvitation)
	mux.HandleFunc("GET /api/weddings/{weddingID}/admin/overview", a.adminOverview)
	mux.HandleFunc("GET /api/weddings/{weddingID}/admin/roster", a.adminRoster)
	// Invitation capability-token routes, open to the invitee who holds the link.
	mux.HandleFunc("GET /api/invitations/{token}", a.getInvitation)
	mux.HandleFunc("POST /api/invitations/{token}/accept", a.acceptInvitation)
	mux.HandleFunc("POST /api/invitations/{token}/decline", a.declineInvitation)
	// Guest routes require an accepted guest-type invitation.
	mux.HandleFunc("GET /api/guest/{token}/dashboard", a.guestDashboard)
	mux.HandleFunc("PUT /api/guest/{token}/rsvp", a.updateRSVP)
	mux.HandleFunc("POST /api/guest/{token}/messages", a.createGuestMessage)
	// Committee routes require the admin token or an accepted committee invitation.
	mux.HandleFunc("GET /api/weddings/{weddingID}/committee/dashboard", a.committeeDashboard)
	mux.HandleFunc("GET /api/weddings/{weddingID}/committee/chat", a.committeeChat)
	mux.HandleFunc("POST /api/weddings/{weddingID}/committee/chat", a.sendCommitteeMessage)
	mux.HandleFunc("POST /api/weddings/{weddingID}/committee/tasks", a.createCommitteeTask)
	mux.HandleFunc("PUT /api/weddings/{weddingID}/committee/tasks/{taskID}", a.updateCommitteeTask)
	mux.HandleFunc("DELETE /api/weddings/{weddingID}/committee/tasks/{taskID}", a.deleteCommitteeTask)
	mux.HandleFunc("POST /api/weddings/{weddingID}/committee/announcements", a.createCommitteeAnnouncement)
	mux.HandleFunc("PUT /api/weddings/{weddingID}/committee/announcements/{announcementID}", a.updateCommitteeAnnouncement)
	mux.HandleFunc("DELETE /api/weddings/{weddingID}/committee/announcements/{announcementID}", a.deleteCommitteeAnnouncement)
	// Public templates and fonts catalog
	mux.HandleFunc("GET /api/templates", a.listTemplates)
	mux.HandleFunc("GET /api/fonts", a.listFonts)
	// Wedding card design customization
	mux.HandleFunc("GET /api/weddings/{weddingID}/card", a.getCardConfig)
	mux.HandleFunc("PUT /api/weddings/{weddingID}/card", a.updateCardConfig)
	// Dynamic committee roles and member management
	mux.HandleFunc("GET /api/weddings/{weddingID}/committee/roles", a.listCommitteeRoles)
	mux.HandleFunc("POST /api/weddings/{weddingID}/committee/roles", a.createCommitteeRole)
	mux.HandleFunc("DELETE /api/weddings/{weddingID}/committee/roles/{roleID}", a.deleteCommitteeRole)
	mux.HandleFunc("PUT /api/weddings/{weddingID}/committee/members/{memberID}", a.updateCommitteeMember)
	mux.HandleFunc("DELETE /api/weddings/{weddingID}/committee/members/{memberID}", a.deleteCommitteeMember)
	return securityHeaders(corsMiddleware(parseAllowedOrigins(allowedOrigins), allowLoopback, mux))
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// weddingCreated wraps a newly created wedding together with its one-time admin
// capability token. Only the token hash is stored; the raw token is returned here once.
type weddingCreated struct {
	Wedding    models.Wedding `json:"wedding"`
	AdminToken string         `json:"admin_token"`
}

func (a *API) createWedding(w http.ResponseWriter, r *http.Request) {
	var wedding models.Wedding
	if err := decodeJSON(w, r, &wedding); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	wedding.ID = mustID(w)
	if wedding.ID == "" {
		return
	}
	now := a.now()
	wedding.CreatedAt, wedding.UpdatedAt = now, now
	wedding.Slug = strings.TrimSpace(wedding.Slug)
	wedding.Title = strings.TrimSpace(wedding.Title)
	if err := prepareWedding(&wedding); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	adminToken, adminHash, err := models.NewOpaqueToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not secure this wedding")
		return
	}
	wedding.AdminTokenHash = adminHash
	created, err := a.repo.CreateWedding(wedding)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, weddingCreated{Wedding: created, AdminToken: adminToken})
}

type weddingSummary struct {
	ID         string                   `json:"id"`
	Slug       string                   `json:"slug"`
	Title      string                   `json:"title"`
	PartnerOne string                   `json:"partner_one"`
	PartnerTwo string                   `json:"partner_two"`
	Date       *time.Time               `json:"date,omitempty"`
	Status     models.PublicationStatus `json:"status"`
}

// listWeddings exposes only identifiers and names so strangers cannot read guest,
// committee, or planning data from a public directory.
func (a *API) listWeddings(w http.ResponseWriter, _ *http.Request) {
	weddings := a.repo.ListWeddings()
	out := make([]weddingSummary, 0, len(weddings))
	for _, wedding := range weddings {
		out = append(out, weddingSummary{ID: wedding.ID, Slug: wedding.Slug, Title: wedding.Title,
			PartnerOne: wedding.PartnerOne, PartnerTwo: wedding.PartnerTwo, Date: wedding.Date, Status: wedding.Status})
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) getWedding(w http.ResponseWriter, r *http.Request) {
	wedding, ok := a.requireAdmin(w, r, r.PathValue("weddingID"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, wedding)
}

func (a *API) updateWedding(w http.ResponseWriter, r *http.Request) {
	_, ok := a.requireAdmin(w, r, r.PathValue("weddingID"))
	if !ok {
		return
	}
	var wedding models.Wedding
	if err := decodeJSON(w, r, &wedding); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	wedding.ID = r.PathValue("weddingID")
	wedding.Slug = strings.TrimSpace(wedding.Slug)
	wedding.Title = strings.TrimSpace(wedding.Title)
	wedding.UpdatedAt = a.now()
	if err := prepareWedding(&wedding); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := a.repo.UpdateWedding(wedding)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *API) deleteWedding(w http.ResponseWriter, r *http.Request) {
	_, ok := a.requireAdmin(w, r, r.PathValue("weddingID"))
	if !ok {
		return
	}
	if err := a.repo.DeleteWedding(r.PathValue("weddingID")); err != nil {
		writeRepositoryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type invitationRequest struct {
	Type           models.InvitationType `json:"type"`
	GuestName      string                `json:"guest_name"`
	GuestEmail     string                `json:"guest_email"`
	GuestPhone     string                `json:"guest_phone"`
	CommitteeTitle string                `json:"committee_title"`
	MaxPartySize   int                   `json:"max_party_size"`
	ExpiresAt      *time.Time            `json:"expires_at"`
}

type invitationCreated struct {
	Invitation models.Invitation `json:"invitation"`
	Token      string            `json:"token"`
}

func (a *API) createInvitation(w http.ResponseWriter, r *http.Request) {
	wedding, ok := a.requireAdmin(w, r, r.PathValue("weddingID"))
	if !ok {
		return
	}
	var input invitationRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	invitationType := input.Type.Normalized()
	if !input.Type.Valid() {
		writeError(w, http.StatusBadRequest, "type must be guest or committee")
		return
	}
	input.GuestName = strings.TrimSpace(input.GuestName)
	if input.GuestName == "" {
		writeError(w, http.StatusBadRequest, "guest_name is required")
		return
	}
	if invitationType == models.InvitationGuest {
		if input.MaxPartySize < 1 || input.MaxPartySize > 20 {
			writeError(w, http.StatusBadRequest, "max_party_size between 1 and 20 is required for guest invitations")
			return
		}
	} else {
		// Committee invitations represent a single planner, so party size is fixed.
		input.MaxPartySize = 1
	}
	if input.ExpiresAt != nil && !input.ExpiresAt.After(a.now()) {
		writeError(w, http.StatusBadRequest, "expires_at must be in the future")
		return
	}
	token, hash, err := models.NewOpaqueToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not generate invitation")
		return
	}
	id, err := models.NewID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not generate invitation")
		return
	}
	inv := models.Invitation{ID: id, Type: invitationType, GuestName: input.GuestName, GuestEmail: strings.TrimSpace(input.GuestEmail),
		GuestPhone: strings.TrimSpace(input.GuestPhone), CommitteeTitle: strings.TrimSpace(input.CommitteeTitle),
		MaxPartySize: input.MaxPartySize, Status: models.InvitationPending, TokenHash: hash, ExpiresAt: input.ExpiresAt, CreatedAt: a.now()}
	created, err := a.repo.AddInvitation(wedding.ID, inv)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, invitationCreated{Invitation: created, Token: token})
}

type invitationView struct {
	Wedding    publicWedding     `json:"wedding"`
	Invitation models.Invitation `json:"invitation"`
}

func (a *API) getInvitation(w http.ResponseWriter, r *http.Request) {
	wedding, inv, err := a.invitation(r.PathValue("token"))
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, invitationView{Wedding: publishedWedding(wedding), Invitation: inv})
}

func (a *API) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	// The invitee states which role they are joining as. When provided, that role
	// must match the type the admin issued the invitation for; otherwise a guest
	// invitation could not be used to enter the committee workspace.
	var input struct {
		Role string `json:"role"`
	}
	if r.Body != nil {
		body, readErr := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
		if readErr == nil && len(strings.TrimSpace(string(body))) > 0 {
			if decodeErr := json.Unmarshal(body, &input); decodeErr != nil {
				writeError(w, http.StatusBadRequest, "invalid JSON body")
				return
			}
		}
	}
	if strings.TrimSpace(input.Role) != "" {
		requested := normalizeRole(strings.TrimSpace(input.Role))
		if requested == "" {
			writeError(w, http.StatusBadRequest, "role must be guest or committee")
			return
		}
		_, invitation, err := a.invitation(token)
		if err != nil {
			writeRepositoryError(w, err)
			return
		}
		if requested != invitation.Type.Role() {
			writeError(w, http.StatusForbidden, "this invitation is for the role of "+string(invitation.Type.Role())+", not "+string(requested))
			return
		}
	}
	a.respond(w, token, models.InvitationAccepted)
}

func (a *API) declineInvitation(w http.ResponseWriter, r *http.Request) {
	a.respond(w, r.PathValue("token"), models.InvitationDeclined)
}

// normalizeRole maps the human-facing labels used by invitation links onto backend roles.
func normalizeRole(value string) models.Role {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "committee", "committee_member":
		return models.RoleCommitteeMember
	case "guest":
		return models.RoleGuest
	}
	return ""
}

func (a *API) respond(w http.ResponseWriter, token string, status models.InvitationStatus) {
	if !validToken(token) {
		writeError(w, http.StatusNotFound, "resource not found")
		return
	}
	wedding, inv, err := a.repo.RespondToInvitation(models.HashToken(token), status, a.now())
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, invitationView{Wedding: publishedWedding(wedding), Invitation: inv})
}

type dashboardView struct {
	Wedding    publicWedding     `json:"wedding"`
	Invitation models.Invitation `json:"invitation"`
	RSVP       *models.RSVP      `json:"rsvp,omitempty"`
}

type publicWedding struct {
	ID            string                `json:"id"`
	Slug          string                `json:"slug"`
	Title         string                `json:"title"`
	PartnerOne    string                `json:"partner_one"`
	PartnerTwo    string                `json:"partner_two"`
	Date          *time.Time            `json:"date,omitempty"`
	Venue         string                `json:"venue,omitempty"`
	Address       string                `json:"address,omitempty"`
	City          string                `json:"city,omitempty"`
	State         string                `json:"state,omitempty"`
	Country       string                `json:"country,omitempty"`
	Message       string                `json:"message,omitempty"`
	Verse         string                `json:"verse,omitempty"`
	DressCode     string                `json:"dress_code,omitempty"`
	HeroImage     string                `json:"hero_image,omitempty"`
	TemplateID    string                `json:"template_id,omitempty"`
	CardConfig    *models.CardConfig    `json:"card_config,omitempty"`
	Events        []models.Event        `json:"events"`
	Photos        []models.Photo        `json:"photos"`
	StorySections []models.StorySection `json:"story_sections"`
	Announcements []models.Announcement `json:"announcements"`
}

func (a *API) guestDashboard(w http.ResponseWriter, r *http.Request) {
	wedding, inv, err := a.invitation(r.PathValue("token"))
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	if inv.Type.Normalized() != models.InvitationGuest {
		writeError(w, http.StatusForbidden, "committee invitations cannot open the guest experience")
		return
	}
	if inv.Status != models.InvitationAccepted {
		writeError(w, http.StatusForbidden, "accepted invitation required")
		return
	}
	view := dashboardView{Wedding: publishedWedding(wedding), Invitation: inv}
	for i := range wedding.RSVPs {
		if wedding.RSVPs[i].InvitationID == inv.ID {
			response := wedding.RSVPs[i]
			view.RSVP = &response
			break
		}
	}
	writeJSON(w, http.StatusOK, view)
}

type guestMessageRequest struct {
	Message string `json:"message"`
	Body    string `json:"body"`
}

func (a *API) createGuestMessage(w http.ResponseWriter, r *http.Request) {
	var input guestMessageRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	input.Message = strings.TrimSpace(input.Message)
	input.Body = strings.TrimSpace(input.Body)
	if input.Message != "" && input.Body != "" {
		writeError(w, http.StatusBadRequest, "provide either message or body, not both")
		return
	}
	body := input.Message
	if body == "" {
		body = input.Body
	}
	if body == "" {
		writeError(w, http.StatusBadRequest, "message or body is required")
		return
	}
	if utf8.RuneCountInString(body) > maxGuestMessageLength {
		writeError(w, http.StatusBadRequest, "message must be at most 2000 characters")
		return
	}
	token := r.PathValue("token")
	_, inv, err := a.invitation(token)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	if inv.Type.Normalized() != models.InvitationGuest {
		writeError(w, http.StatusForbidden, "committee invitations cannot use the guest RSVP flow")
		return
	}
	if inv.Status != models.InvitationAccepted {
		writeError(w, http.StatusForbidden, "accepted invitation required")
		return
	}
	message := models.GuestMessage{ID: mustID(w), Body: body, CreatedAt: a.now()}
	if message.ID == "" {
		return
	}
	_, created, err := a.repo.AddGuestMessage(models.HashToken(token), message)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (a *API) updateRSVP(w http.ResponseWriter, r *http.Request) {
	var response models.RSVP
	if err := decodeJSON(w, r, &response); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if response.Status != models.RSVPAttending && response.Status != models.RSVPNotAttending && response.Status != models.RSVPMaybe {
		writeError(w, http.StatusBadRequest, "status must be attending, not_attending, or maybe")
		return
	}
	_, inv, err := a.invitation(r.PathValue("token"))
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	if inv.Type.Normalized() != models.InvitationGuest {
		writeError(w, http.StatusForbidden, "committee invitations cannot use the guest RSVP flow")
		return
	}
	if response.PartySize < 0 || response.PartySize > inv.MaxPartySize || (response.Status == models.RSVPAttending && response.PartySize < 1) || (response.Status == models.RSVPNotAttending && response.PartySize != 0) {
		writeError(w, http.StatusBadRequest, "party_size is invalid for this invitation")
		return
	}
	response.UpdatedAt = a.now()
	_, updated, err := a.repo.UpdateRSVP(models.HashToken(r.PathValue("token")), response)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

type overviewView struct {
	WeddingID            string `json:"wedding_id"`
	InvitationsTotal     int    `json:"invitations_total"`
	InvitationsPending   int    `json:"invitations_pending"`
	Accepted             int    `json:"accepted"`
	Declined             int    `json:"declined"`
	AttendingPartySize   int    `json:"attending_party_size"`
	GuestMessages        int    `json:"guest_messages"`
	CommitteeTotal       int    `json:"committee_total"`
	CommitteePending     int    `json:"committee_pending"`
	CommitteeAccepted    int    `json:"committee_accepted"`
	CommitteeDeclined    int    `json:"committee_declined"`
}

func (a *API) adminOverview(w http.ResponseWriter, r *http.Request) {
	wedding, ok := a.requireAdmin(w, r, r.PathValue("weddingID"))
	if !ok {
		return
	}
	view := overviewView{WeddingID: wedding.ID, InvitationsTotal: len(wedding.Invitations), GuestMessages: len(wedding.GuestMessages)}
	for _, inv := range wedding.Invitations {
		if inv.Type.Normalized() == models.InvitationCommittee {
			view.CommitteeTotal++
			switch inv.Status {
			case models.InvitationPending:
				view.CommitteePending++
			case models.InvitationAccepted:
				view.CommitteeAccepted++
			case models.InvitationDeclined:
				view.CommitteeDeclined++
			}
			continue
		}
		view.InvitationsTotal++
		switch inv.Status {
		case models.InvitationPending:
			view.InvitationsPending++
		case models.InvitationAccepted:
			view.Accepted++
		case models.InvitationDeclined:
			view.Declined++
		}
	}
	for _, response := range wedding.RSVPs {
		if response.Status == models.RSVPAttending {
			view.AttendingPartySize += response.PartySize
		}
	}
	writeJSON(w, http.StatusOK, view)
}

type rosterView struct {
	WeddingID        string                     `json:"wedding_id"`
	Invitations      []models.Invitation        `json:"invitations"`
	Guests           []models.Guest             `json:"guests"`
	CommitteeMembers []models.CommitteeMember   `json:"committee_members"`
	CommitteeRoles   []models.CommitteeRole     `json:"committee_roles"`
}

// adminRoster returns the full invitation status of both groups so the admin can
// separate committee members from guests. Admin scope only.
func (a *API) adminRoster(w http.ResponseWriter, r *http.Request) {
	wedding, ok := a.requireAdmin(w, r, r.PathValue("weddingID"))
	if !ok {
		return
	}
	roles := wedding.CommitteeRoles
	if roles == nil {
		roles = []models.CommitteeRole{}
	}
	writeJSON(w, http.StatusOK, rosterView{
		WeddingID:        wedding.ID,
		Invitations:      wedding.Invitations,
		Guests:           wedding.Guests,
		CommitteeMembers: wedding.CommitteeMembers,
		CommitteeRoles:   roles,
	})
}

type committeeActor struct {
	Role         models.Role `json:"role"`
	Name         string      `json:"name"`
	MemberID     string      `json:"member_id,omitempty"`
	InvitationID string      `json:"invitation_id,omitempty"`
}

type guestStats struct {
	Invited   int `json:"invited"`
	Accepted  int `json:"accepted"`
	Pending   int `json:"pending"`
	Declined  int `json:"declined"`
	Attending int `json:"attending_party_size"`
}

// committeeDashboardView merges the shared published wedding content with the
// committee-only planning data. Announcements carry their audience so guests are
// never sent the committee-only copy of the same wedding.
type committeeDashboardView struct {
	Wedding       publicWedding            `json:"wedding"`
	Actor         committeeActor           `json:"actor"`
	Members       []models.CommitteeMember `json:"committee_members"`
	Roles         []models.CommitteeRole   `json:"committee_roles"`
	Tasks         []models.PlanningTask    `json:"planning_tasks"`
	Announcements []models.Announcement    `json:"announcements"`
	GuestStats    guestStats               `json:"guest_stats"`
}

func (a *API) committeeDashboard(w http.ResponseWriter, r *http.Request) {
	wedding, actor, ok := a.requireCommittee(w, r, r.PathValue("weddingID"))
	if !ok {
		return
	}
	roles := wedding.CommitteeRoles
	if roles == nil {
		roles = []models.CommitteeRole{}
	}
	view := committeeDashboardView{
		Wedding: publishedWedding(wedding),
		Actor:   committeeActor{Role: actor.Role, Name: actor.Name, MemberID: actor.MemberID, InvitationID: actor.InvitationID},
		Members: wedding.CommitteeMembers,
		Roles:   roles,
		Tasks:   wedding.PlanningTasks,
	}
	for _, item := range wedding.Announcements {
		if item.Status == models.StatusPublished {
			view.Announcements = append(view.Announcements, item)
		}
	}
	view.GuestStats = computeGuestStats(wedding)
	writeJSON(w, http.StatusOK, view)
}

// computeGuestStats aggregates the guest group only, so the committee sees an
// RSVP overview without exposing individual guest records.
func computeGuestStats(w models.Wedding) guestStats {
	var stats guestStats
	guestInvitations := make(map[string]bool)
	for _, inv := range w.Invitations {
		if inv.Type.Normalized() != models.InvitationGuest {
			continue
		}
		guestInvitations[inv.ID] = true
		stats.Invited++
		switch inv.Status {
		case models.InvitationAccepted:
			stats.Accepted++
		case models.InvitationPending:
			stats.Pending++
		case models.InvitationDeclined:
			stats.Declined++
		}
	}
	for _, response := range w.RSVPs {
		if response.Status == models.RSVPAttending && guestInvitations[response.InvitationID] {
			stats.Attending += response.PartySize
		}
	}
	return stats
}

// committeeChat returns the private planning conversation, optionally only messages
// created after the given RFC3339 timestamp so clients can poll for new messages.
func (a *API) committeeChat(w http.ResponseWriter, r *http.Request) {
	wedding, _, ok := a.requireCommittee(w, r, r.PathValue("weddingID"))
	if !ok {
		return
	}
	var since time.Time
	if raw := strings.TrimSpace(r.URL.Query().Get("since")); raw != "" {
		if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
			since = parsed
		}
	}
	messages, err := a.repo.CommitteeMessages(wedding.ID, since)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, messages)
}

type committeeMessageRequest struct {
	Message string `json:"message"`
	Body    string `json:"body"`
}

func (a *API) sendCommitteeMessage(w http.ResponseWriter, r *http.Request) {
	wedding, actor, ok := a.requireCommittee(w, r, r.PathValue("weddingID"))
	if !ok {
		return
	}
	var input committeeMessageRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	input.Message = strings.TrimSpace(input.Message)
	input.Body = strings.TrimSpace(input.Body)
	body := input.Message
	if body == "" {
		body = input.Body
	}
	if body == "" {
		writeError(w, http.StatusBadRequest, "message cannot be empty")
		return
	}
	if utf8.RuneCountInString(body) > maxGuestMessageLength {
		writeError(w, http.StatusBadRequest, "message must be at most 2000 characters")
		return
	}
	authorID := actor.MemberID
	if authorID == "" {
		authorID = string(actor.Role)
	}
	message := models.CommitteeMessage{ID: mustID(w), AuthorID: authorID, AuthorName: actor.Name, AuthorRole: actor.Role, Body: body, CreatedAt: a.now()}
	if message.ID == "" {
		return
	}
	created, err := a.repo.AddCommitteeMessage(wedding.ID, message)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

type planningTaskRequest struct {
	Title      string            `json:"title"`
	Details    string            `json:"details"`
	AssignedTo string            `json:"assigned_to"`
	DueOn      string            `json:"due_on"`
	Status     models.TaskStatus `json:"status"`
}

func (a *API) createCommitteeTask(w http.ResponseWriter, r *http.Request) {
	wedding, actor, ok := a.requireCommittee(w, r, r.PathValue("weddingID"))
	if !ok {
		return
	}
	input, valid := parseTaskInput(w, r)
	if !valid {
		return
	}
	now := a.now()
	taskID := mustID(w)
	if taskID == "" {
		return
	}
	task := models.PlanningTask{ID: taskID, Title: input.Title, Details: input.Details, AssignedTo: input.AssignedTo,
		DueOn: input.DueOn, Status: input.Status, CreatedBy: actor.Name, CreatedAt: now, UpdatedAt: now}
	created, err := a.repo.AddPlanningTask(wedding.ID, task)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (a *API) updateCommitteeTask(w http.ResponseWriter, r *http.Request) {
	wedding, _, ok := a.requireCommittee(w, r, r.PathValue("weddingID"))
	if !ok {
		return
	}
	input, valid := parseTaskInput(w, r)
	if !valid {
		return
	}
	now := a.now()
	task := models.PlanningTask{ID: r.PathValue("taskID"), Title: input.Title, Details: input.Details, AssignedTo: input.AssignedTo,
		DueOn: input.DueOn, Status: input.Status, UpdatedAt: now}
	updated, err := a.repo.UpdatePlanningTask(wedding.ID, task)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *API) deleteCommitteeTask(w http.ResponseWriter, r *http.Request) {
	wedding, _, ok := a.requireCommittee(w, r, r.PathValue("weddingID"))
	if !ok {
		return
	}
	if err := a.repo.DeletePlanningTask(wedding.ID, r.PathValue("taskID")); err != nil {
		writeRepositoryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// parseTaskInput validates a shared planning-task payload. The caller owns the
// repository write.
func parseTaskInput(w http.ResponseWriter, r *http.Request) (planningTaskRequest, bool) {
	var input planningTaskRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return input, false
	}
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return input, false
	}
	if input.Status == "" {
		input.Status = models.TaskTodo
	}
	if !input.Status.Valid() {
		writeError(w, http.StatusBadRequest, "status must be todo, in_progress, or done")
		return input, false
	}
	return input, true
}

type announcementRequest struct {
	Title    string             `json:"title"`
	Body     string             `json:"body"`
	Audience models.Audience    `json:"audience"`
	Status   models.PublicationStatus `json:"status"`
}

func parseAnnouncementInput(w http.ResponseWriter, r *http.Request) (announcementRequest, bool) {
	var input announcementRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return input, false
	}
	input.Title = strings.TrimSpace(input.Title)
	input.Body = strings.TrimSpace(input.Body)
	if input.Title == "" || input.Body == "" {
		writeError(w, http.StatusBadRequest, "title and body are required")
		return input, false
	}
	if input.Audience == "" {
		input.Audience = models.AudiencePublic
	}
	if !input.Audience.Valid() {
		writeError(w, http.StatusBadRequest, "audience must be public or committee")
		return input, false
	}
	if input.Status == "" {
		input.Status = models.StatusDraft
	}
	if !input.Status.Valid() {
		writeError(w, http.StatusBadRequest, "status must be draft, published, or hidden")
		return input, false
	}
	return input, true
}

func (a *API) createCommitteeAnnouncement(w http.ResponseWriter, r *http.Request) {
	wedding, actor, ok := a.requireCommittee(w, r, r.PathValue("weddingID"))
	if !ok {
		return
	}
	input, valid := parseAnnouncementInput(w, r)
	if !valid {
		return
	}
	announcementID := mustID(w)
	if announcementID == "" {
		return
	}
	now := a.now()
	publishedAt := (*time.Time)(nil)
	if input.Status == models.StatusPublished {
		value := now
		publishedAt = &value
	}
	announcement := models.Announcement{ID: announcementID, Title: input.Title, Body: input.Body,
		Audience: input.Audience.Normalized(), Status: input.Status, PublishedAt: publishedAt, AuthorName: actor.Name, CreatedAt: now}
	created, err := a.repo.AddAnnouncement(wedding.ID, announcement)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (a *API) updateCommitteeAnnouncement(w http.ResponseWriter, r *http.Request) {
	wedding, _, ok := a.requireCommittee(w, r, r.PathValue("weddingID"))
	if !ok {
		return
	}
	input, valid := parseAnnouncementInput(w, r)
	if !valid {
		return
	}
	now := a.now()
	publishedAt := (*time.Time)(nil)
	if input.Status == models.StatusPublished {
		value := now
		publishedAt = &value
	}
	announcement := models.Announcement{ID: r.PathValue("announcementID"), Title: input.Title, Body: input.Body,
		Audience: input.Audience.Normalized(), Status: input.Status, PublishedAt: publishedAt}
	updated, err := a.repo.UpdateAnnouncement(wedding.ID, announcement)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *API) deleteCommitteeAnnouncement(w http.ResponseWriter, r *http.Request) {
	wedding, _, ok := a.requireCommittee(w, r, r.PathValue("weddingID"))
	if !ok {
		return
	}
	if err := a.repo.DeleteAnnouncement(wedding.ID, r.PathValue("announcementID")); err != nil {
		writeRepositoryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listTemplates(w http.ResponseWriter, _ *http.Request) {
	templates := getTemplatesCatalog()
	writeJSON(w, http.StatusOK, templates)
}

func (a *API) listFonts(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, getFontsCatalog())
}

func (a *API) getCardConfig(w http.ResponseWriter, r *http.Request) {
	weddingID := r.PathValue("weddingID")
	wedding, err := a.repo.GetWedding(weddingID)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	if wedding.CardConfig != nil {
		writeJSON(w, http.StatusOK, wedding.CardConfig)
		return
	}
	defaultConfig := models.CardConfig{
		TemplateID: wedding.TemplateID,
		Fonts: models.CardFonts{
			Couple:  "Great Vibes",
			Heading: "Playfair Display",
			Body:    "Cormorant Garamond",
		},
		Colors: models.CardColors{
			Background: "#2d4030",
			Text:       "#f7f4ed",
			Accent:     "#d4af37",
			Border:     "#e5c158",
		},
		Decorations: models.CardDecorations{
			FloralStyle: "sage-botanical-corners",
			BorderStyle: "double-gold",
			Layout:      "centered-classic",
		},
	}
	writeJSON(w, http.StatusOK, defaultConfig)
}

func (a *API) updateCardConfig(w http.ResponseWriter, r *http.Request) {
	wedding, ok := a.requireAdmin(w, r, r.PathValue("weddingID"))
	if !ok {
		return
	}
	var config models.CardConfig
	if err := decodeJSON(w, r, &config); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	config.TemplateID = strings.TrimSpace(config.TemplateID)
	if config.TemplateID == "" {
		config.TemplateID = wedding.TemplateID
	}
	updated, err := a.repo.UpdateCardConfig(wedding.ID, config)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *API) listCommitteeRoles(w http.ResponseWriter, r *http.Request) {
	wedding, _, ok := a.requireCommittee(w, r, r.PathValue("weddingID"))
	if !ok {
		return
	}
	roles := wedding.CommitteeRoles
	if roles == nil {
		roles = []models.CommitteeRole{}
	}
	writeJSON(w, http.StatusOK, roles)
}

func (a *API) createCommitteeRole(w http.ResponseWriter, r *http.Request) {
	wedding, ok := a.requireAdmin(w, r, r.PathValue("weddingID"))
	if !ok {
		return
	}
	var role models.CommitteeRole
	if err := decodeJSON(w, r, &role); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	role.Name = strings.TrimSpace(role.Name)
	if role.Name == "" {
		writeError(w, http.StatusBadRequest, "role name is required")
		return
	}
	role.ID = mustID(w)
	if role.ID == "" {
		return
	}
	role.WeddingID = wedding.ID
	role.CreatedAt = a.now()
	created, err := a.repo.AddCommitteeRole(wedding.ID, role)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (a *API) deleteCommitteeRole(w http.ResponseWriter, r *http.Request) {
	wedding, ok := a.requireAdmin(w, r, r.PathValue("weddingID"))
	if !ok {
		return
	}
	roleID := r.PathValue("roleID")
	if err := a.repo.DeleteCommitteeRole(wedding.ID, roleID); err != nil {
		writeRepositoryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) updateCommitteeMember(w http.ResponseWriter, r *http.Request) {
	wedding, ok := a.requireAdmin(w, r, r.PathValue("weddingID"))
	if !ok {
		return
	}
	var member models.CommitteeMember
	if err := decodeJSON(w, r, &member); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	member.ID = r.PathValue("memberID")
	member.Name = strings.TrimSpace(member.Name)
	if member.Name == "" {
		writeError(w, http.StatusBadRequest, "member name is required")
		return
	}
	updated, err := a.repo.UpdateCommitteeMember(wedding.ID, member)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *API) deleteCommitteeMember(w http.ResponseWriter, r *http.Request) {
	wedding, ok := a.requireAdmin(w, r, r.PathValue("weddingID"))
	if !ok {
		return
	}
	memberID := r.PathValue("memberID")
	if err := a.repo.DeleteCommitteeMember(wedding.ID, memberID); err != nil {
		writeRepositoryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func getFontsCatalog() []models.FontItem {
	return []models.FontItem{
		{ID: "great-vibes", Name: "Great Vibes", Category: "couple", Family: "'Great Vibes', cursive"},
		{ID: "allura", Name: "Allura", Category: "couple", Family: "'Allura', cursive"},
		{ID: "alex-brush", Name: "Alex Brush", Category: "couple", Family: "'Alex Brush', cursive"},
		{ID: "dancing-script", Name: "Dancing Script", Category: "couple", Family: "'Dancing Script', cursive"},
		{ID: "parisienne", Name: "Parisienne", Category: "couple", Family: "'Parisienne', cursive"},
		{ID: "cinzel", Name: "Cinzel Decorative", Category: "couple", Family: "'Cinzel Decorative', serif"},
		{ID: "pinyon-script", Name: "Pinyon Script", Category: "couple", Family: "'Pinyon Script', cursive"},
		{ID: "playfair-display", Name: "Playfair Display", Category: "heading", Family: "'Playfair Display', serif"},
		{ID: "cormorant-garamond", Name: "Cormorant Garamond", Category: "heading", Family: "'Cormorant Garamond', serif"},
		{ID: "libre-baskerville", Name: "Libre Baskerville", Category: "heading", Family: "'Libre Baskerville', serif"},
		{ID: "marcellus", Name: "Marcellus", Category: "heading", Family: "'Marcellus', serif"},
		{ID: "bodoni-moda", Name: "Bodoni Moda", Category: "heading", Family: "'Bodoni Moda', serif"},
		{ID: "prata", Name: "Prata", Category: "heading", Family: "'Prata', serif"},
		{ID: "montserrat", Name: "Montserrat", Category: "body", Family: "'Montserrat', sans-serif"},
		{ID: "lato", Name: "Lato", Category: "body", Family: "'Lato', sans-serif"},
	}
}

func getTemplatesCatalog() []models.Template {
	// Try loading templates from JSON file paths
	paths := []string{"templates/templates.json", "../templates/templates.json", "/home/student/weddinghub/templates/templates.json"}
	for _, p := range paths {
		if data, err := os.ReadFile(p); err == nil {
			var list []models.Template
			if err := json.Unmarshal(data, &list); err == nil && len(list) > 0 {
				return list
			}
		}
	}
	// Fallback seed template matching download.webp
	return []models.Template{
		{
			ID:          "luxury-sage-download",
			Name:        "Sage Botanical Luxe",
			Category:    "luxury-floral",
			Description: "Deep sage green with 3D gold botanical flourishes and double gold frame",
			Fonts: models.CardFonts{
				Couple:  "Great Vibes",
				Heading: "Playfair Display",
				Body:    "Cormorant Garamond",
			},
			Colors: models.CardColors{
				Background: "#2d4030",
				Text:       "#f7f4ed",
				Accent:     "#d4af37",
				Border:     "#e5c158",
			},
			Decorations: models.CardDecorations{
				FloralStyle: "sage-botanical-corners",
				BorderStyle: "double-gold",
				Layout:      "centered-classic",
				FrameGlow:   true,
				DatePill:    true,
			},
			Layout:  "centered-classic",
			Premium: true,
		},
	}
}

func (a *API) invitation(token string) (models.Wedding, models.Invitation, error) {
	if !validToken(token) {
		return models.Wedding{}, models.Invitation{}, repository.ErrNotFound
	}
	return a.repo.InvitationByHash(models.HashToken(token))
}

func validToken(token string) bool {
	// Generated tokens encode exactly 32 random bytes using unpadded URL-safe base64.
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(decoded) == 32
}

func publishedWedding(w models.Wedding) publicWedding {
	view := publicWedding{
		ID: w.ID, Slug: w.Slug, Title: w.Title, PartnerOne: w.PartnerOne, PartnerTwo: w.PartnerTwo, Date: w.Date,
		Venue: w.Venue, Address: w.Address, City: w.City, State: w.State, Country: w.Country,
		Message: w.Message, Verse: w.Verse, DressCode: w.DressCode, HeroImage: w.HeroImage, TemplateID: w.TemplateID,
		CardConfig: w.CardConfig,
		Events: []models.Event{}, Photos: []models.Photo{}, StorySections: []models.StorySection{}, Announcements: []models.Announcement{},
	}
	for _, item := range w.Events {
		if item.Status == models.StatusPublished {
			view.Events = append(view.Events, item)
		}
	}
	for _, item := range w.Photos {
		if item.Status == models.StatusPublished {
			view.Photos = append(view.Photos, item)
		}
	}
	for _, item := range w.StorySections {
		if item.Status == models.StatusPublished {
			view.StorySections = append(view.StorySections, item)
		}
	}
	for _, item := range w.Announcements {
		if item.Status == models.StatusPublished && item.Audience.GuestVisible() {
			view.Announcements = append(view.Announcements, item)
		}
	}
	return view
}

func prepareWedding(w *models.Wedding) error {
	if w.Slug == "" || w.Title == "" || !w.Status.Valid() {
		return errors.New("slug, title, and a valid status are required")
	}
	for i := range w.Events {
		if !w.Events[i].Status.Valid() {
			return errors.New("every event requires a valid status")
		}
		if err := ensureID(&w.Events[i].ID); err != nil {
			return err
		}
	}
	for i := range w.Photos {
		if !w.Photos[i].Status.Valid() {
			return errors.New("every photo requires a valid status")
		}
		if err := ensureID(&w.Photos[i].ID); err != nil {
			return err
		}
	}
	for i := range w.StorySections {
		if !w.StorySections[i].Status.Valid() {
			return errors.New("every story section requires a valid status")
		}
		if err := ensureID(&w.StorySections[i].ID); err != nil {
			return err
		}
	}
	for i := range w.Announcements {
		if !w.Announcements[i].Status.Valid() {
			return errors.New("every announcement requires a valid status")
		}
		if err := ensureID(&w.Announcements[i].ID); err != nil {
			return err
		}
	}
	return nil
}

func ensureID(id *string) error {
	if *id != "" {
		return nil
	}
	generated, err := models.NewID()
	if err != nil {
		return errors.New("could not generate content identifier")
	}
	*id = generated
	return nil
}

func mustID(w http.ResponseWriter) string {
	id, err := models.NewID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not generate identifier")
		return ""
	}
	return id
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return errors.New("invalid JSON body")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("body must contain one JSON object")
	}
	return nil
}

func writeRepositoryError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		writeError(w, http.StatusNotFound, "resource not found")
	case errors.Is(err, repository.ErrConflict):
		writeError(w, http.StatusConflict, "resource already exists")
	case errors.Is(err, repository.ErrExpired):
		writeError(w, http.StatusGone, "invitation expired")
	case errors.Is(err, repository.ErrInvalidStatus):
		writeError(w, http.StatusConflict, "operation is not valid in the current state")
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func parseAllowedOrigins(configured string) map[string]struct{} {
	origins := make(map[string]struct{})
	for _, value := range strings.Split(configured, ",") {
		origin := strings.TrimSpace(value)
		if validOrigin(origin) {
			origins[origin] = struct{}{}
		}
	}
	return origins
}

func validOrigin(origin string) bool {
	parsed, err := url.Parse(origin)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && parsed.User == nil && parsed.Path == "" && parsed.RawQuery == "" && parsed.Fragment == ""
}

func loopbackOrigin(origin string) bool {
	if !validOrigin(origin) {
		return false
	}
	parsed, _ := url.Parse(origin)
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func corsMiddleware(allowedOrigins map[string]struct{}, allowLoopback bool, next http.Handler) http.Handler {
	const (
		allowedMethods = "GET, POST, PUT, DELETE"
		allowedHeaders = "Accept, Authorization, Content-Type"
	)
	methods := map[string]bool{http.MethodGet: true, http.MethodPost: true, http.MethodPut: true, http.MethodDelete: true}
	requestHeaders := map[string]bool{"accept": true, "authorization": true, "content-type": true}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Add("Vary", "Origin")
		_, explicitlyAllowed := allowedOrigins[origin]
		if !explicitlyAllowed && !(allowLoopback && loopbackOrigin(origin)) {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if r.Method == http.MethodOptions {
			w.Header().Add("Vary", "Access-Control-Request-Method")
			w.Header().Add("Vary", "Access-Control-Request-Headers")
			requestedMethod := r.Header.Get("Access-Control-Request-Method")
			if requestedMethod != "" && !methods[requestedMethod] {
				writeError(w, http.StatusForbidden, "CORS method not allowed")
				return
			}
			for _, header := range strings.Split(r.Header.Get("Access-Control-Request-Headers"), ",") {
				header = strings.ToLower(strings.TrimSpace(header))
				if header != "" && !requestHeaders[header] {
					writeError(w, http.StatusForbidden, "CORS header not allowed")
					return
				}
			}
			w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
			w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
