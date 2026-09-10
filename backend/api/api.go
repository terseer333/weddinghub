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
	mux.HandleFunc("GET /api/weddings/{weddingID}", a.getWedding)
	mux.HandleFunc("PUT /api/weddings/{weddingID}", a.updateWedding)
	mux.HandleFunc("DELETE /api/weddings/{weddingID}", a.deleteWedding)
	mux.HandleFunc("POST /api/weddings/{weddingID}/invitations", a.createInvitation)
	mux.HandleFunc("GET /api/weddings/{weddingID}/admin/overview", a.adminOverview)
	mux.HandleFunc("GET /api/invitations/{token}", a.getInvitation)
	mux.HandleFunc("POST /api/invitations/{token}/accept", a.acceptInvitation)
	mux.HandleFunc("POST /api/invitations/{token}/decline", a.declineInvitation)
	mux.HandleFunc("GET /api/guest/{token}/dashboard", a.guestDashboard)
	mux.HandleFunc("PUT /api/guest/{token}/rsvp", a.updateRSVP)
	mux.HandleFunc("POST /api/guest/{token}/messages", a.createGuestMessage)
	return securityHeaders(corsMiddleware(parseAllowedOrigins(allowedOrigins), allowLoopback, mux))
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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
	created, err := a.repo.CreateWedding(wedding)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (a *API) listWeddings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.repo.ListWeddings())
}

func (a *API) getWedding(w http.ResponseWriter, r *http.Request) {
	wedding, err := a.repo.GetWedding(r.PathValue("weddingID"))
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, wedding)
}

func (a *API) updateWedding(w http.ResponseWriter, r *http.Request) {
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
	if err := a.repo.DeleteWedding(r.PathValue("weddingID")); err != nil {
		writeRepositoryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type invitationRequest struct {
	GuestName    string     `json:"guest_name"`
	GuestEmail   string     `json:"guest_email"`
	MaxPartySize int        `json:"max_party_size"`
	ExpiresAt    *time.Time `json:"expires_at"`
}

type invitationCreated struct {
	Invitation models.Invitation `json:"invitation"`
	Token      string            `json:"token"`
}

func (a *API) createInvitation(w http.ResponseWriter, r *http.Request) {
	var input invitationRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	input.GuestName = strings.TrimSpace(input.GuestName)
	if input.GuestName == "" || input.MaxPartySize < 1 || input.MaxPartySize > 20 {
		writeError(w, http.StatusBadRequest, "guest_name and max_party_size between 1 and 20 are required")
		return
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
	inv := models.Invitation{ID: id, GuestName: input.GuestName, GuestEmail: strings.TrimSpace(input.GuestEmail), MaxPartySize: input.MaxPartySize, Status: models.InvitationPending, TokenHash: hash, ExpiresAt: input.ExpiresAt, CreatedAt: a.now()}
	created, err := a.repo.AddInvitation(r.PathValue("weddingID"), inv)
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
	a.respond(w, r.PathValue("token"), models.InvitationAccepted)
}

func (a *API) declineInvitation(w http.ResponseWriter, r *http.Request) {
	a.respond(w, r.PathValue("token"), models.InvitationDeclined)
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
	WeddingID          string `json:"wedding_id"`
	InvitationsTotal   int    `json:"invitations_total"`
	InvitationsPending int    `json:"invitations_pending"`
	Accepted           int    `json:"accepted"`
	Declined           int    `json:"declined"`
	AttendingPartySize int    `json:"attending_party_size"`
	GuestMessages      int    `json:"guest_messages"`
}

func (a *API) adminOverview(w http.ResponseWriter, r *http.Request) {
	wedding, err := a.repo.GetWedding(r.PathValue("weddingID"))
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	view := overviewView{WeddingID: wedding.ID, InvitationsTotal: len(wedding.Invitations), GuestMessages: len(wedding.GuestMessages)}
	for _, inv := range wedding.Invitations {
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
		if item.Status == models.StatusPublished {
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
