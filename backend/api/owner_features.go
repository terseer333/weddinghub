package api

import (
	"net/http"
	"net/url"
	"os"
	"strings"

	"weddinghub/delivery"
	"weddinghub/models"
)

func (a *API) createOwnerInvitation(w http.ResponseWriter, r *http.Request) {
	_, wedding, ok := a.ownerWedding(w, r)
	if !ok {
		return
	}
	var input invitationRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !input.Type.Valid() {
		writeError(w, http.StatusBadRequest, "type must be guest or committee")
		return
	}
	input.GuestName = strings.TrimSpace(input.GuestName)
	input.GuestEmail = strings.TrimSpace(input.GuestEmail)
	input.GuestPhone = strings.TrimSpace(input.GuestPhone)
	input.CommitteeTitle = strings.TrimSpace(input.CommitteeTitle)
	if input.GuestName == "" || !ownerTextLength(input.GuestName, 160) || !ownerTextLength(input.GuestEmail, 254) ||
		!ownerTextLength(input.GuestPhone, 40) || !ownerTextLength(input.CommitteeTitle, 100) {
		writeError(w, http.StatusBadRequest, "guest name is required and invitation fields must be within their length limits")
		return
	}
	if input.GuestEmail != "" && !validEmail(input.GuestEmail) {
		writeError(w, http.StatusBadRequest, "guest_email must be a valid email address")
		return
	}
	typ := input.Type.Normalized()
	if typ == models.InvitationGuest && (input.MaxPartySize < 1 || input.MaxPartySize > 20) {
		writeError(w, http.StatusBadRequest, "max_party_size must be between 1 and 20")
		return
	}
	if typ == models.InvitationCommittee {
		input.MaxPartySize = 1
	}
	token, hash, err := models.NewOpaqueToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not generate invitation")
		return
	}
	code, codeHash, err := models.NewShortCode()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not generate invitation")
		return
	}
	id := mustID(w)
	if id == "" {
		return
	}
	invitation := models.Invitation{
		ID: id, Type: typ, GuestName: input.GuestName, GuestEmail: input.GuestEmail,
		GuestPhone: input.GuestPhone, CommitteeTitle: input.CommitteeTitle, MaxPartySize: input.MaxPartySize,
		Status: models.InvitationPending, TokenHash: hash, ShortCodeHash: codeHash, ExpiresAt: input.ExpiresAt, CreatedAt: a.now(),
	}
	created, err := a.repo.AddInvitation(wedding.ID, invitation)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"invitation": created, "token": token, "short_code": code})
}

func (a *API) updateOwnerInvitation(w http.ResponseWriter, r *http.Request) {
	_, wedding, ok := a.ownerWedding(w, r)
	if !ok {
		return
	}
	var input invitationRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("invitationID")
	var current models.Invitation
	found := false
	for _, invitation := range wedding.Invitations {
		if invitation.ID == id {
			current, found = invitation, true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "invitation not found")
		return
	}
	name := strings.TrimSpace(input.GuestName)
	if name == "" || !ownerTextLength(name, 160) || !ownerTextLength(input.GuestEmail, 254) ||
		!ownerTextLength(input.GuestPhone, 40) || !ownerTextLength(input.CommitteeTitle, 100) {
		writeError(w, http.StatusBadRequest, "guest name is required and invitation fields must be within their length limits")
		return
	}
	if input.GuestEmail != "" && !validEmail(strings.TrimSpace(input.GuestEmail)) {
		writeError(w, http.StatusBadRequest, "guest_email must be a valid email address")
		return
	}
	party := input.MaxPartySize
	if current.Type.Normalized() == models.InvitationGuest && (party < 1 || party > 20) {
		writeError(w, http.StatusBadRequest, "max_party_size must be between 1 and 20")
		return
	}
	if current.Type.Normalized() == models.InvitationCommittee {
		party = 1
	}
	current.GuestName = name
	current.GuestEmail = strings.TrimSpace(input.GuestEmail)
	current.GuestPhone = strings.TrimSpace(input.GuestPhone)
	current.CommitteeTitle = strings.TrimSpace(input.CommitteeTitle)
	current.MaxPartySize = party
	updated, err := a.repo.UpdateInvitation(wedding.ID, current)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *API) deleteOwnerInvitation(w http.ResponseWriter, r *http.Request) {
	_, wedding, ok := a.ownerWedding(w, r)
	if !ok {
		return
	}
	if err := a.repo.DeleteInvitation(wedding.ID, r.PathValue("invitationID")); err != nil {
		writeRepositoryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) refreshOwnerInvitationLink(w http.ResponseWriter, r *http.Request) {
	_, wedding, ok := a.ownerWedding(w, r)
	if !ok {
		return
	}
	id := r.PathValue("invitationID")
	for _, invitation := range wedding.Invitations {
		if invitation.ID != id {
			continue
		}
		token, hash, err := models.NewOpaqueToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create invitation link")
			return
		}
		code, codeHash, err := models.NewShortCode()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create invitation link")
			return
		}
		invitation.TokenHash = hash
		invitation.ShortCodeHash = codeHash
		updated, err := a.repo.UpdateInvitation(wedding.ID, invitation)
		if err != nil {
			writeRepositoryError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"invitation": updated, "token": token, "short_code": code})
		return
	}
	writeError(w, http.StatusNotFound, "invitation not found")
}

func (a *API) sendOwnerInvitation(w http.ResponseWriter, r *http.Request) {
	_, wedding, ok := a.ownerWedding(w, r)
	if !ok {
		return
	}
	var input sendInvitationRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	input.Token = strings.TrimSpace(input.Token)
	if input.Token == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}
	var invitation *models.Invitation
	for i := range wedding.Invitations {
		if wedding.Invitations[i].ID == r.PathValue("invitationID") {
			invitation = &wedding.Invitations[i]
			break
		}
	}
	if invitation == nil {
		writeError(w, http.StatusNotFound, "invitation not found")
		return
	}
	if !constantTimeMatch(invitation.TokenHash, models.HashToken(input.Token)) {
		writeError(w, http.StatusForbidden, "the supplied token does not match this invitation")
		return
	}
	channels, valid := normalizeDeliveryChannels(input.Channels)
	if !valid {
		writeError(w, http.StatusBadRequest, "channels must contain email or whatsapp")
		return
	}
	if a.sender == nil || len(a.sender.Channels()) == 0 {
		writeError(w, http.StatusServiceUnavailable, "no delivery channel is configured")
		return
	}
	link := a.sender.Link(input.Token)
	if validShortCode(input.ShortCode) && constantTimeMatch(invitation.ShortCodeHash, models.HashToken(input.ShortCode)) {
		if base := strings.TrimRight(strings.TrimSpace(os.Getenv("WEDDINGHUB_PUBLIC_BASE_URL")), "/"); base != "" {
			link = base + "/i/" + input.ShortCode
		}
	}
	if link == "" {
		writeError(w, http.StatusServiceUnavailable, "WEDDINGHUB_PUBLIC_BASE_URL is required to build invitation links")
		return
	}
	active := make(map[string]bool)
	for _, channel := range a.sender.Channels() {
		active[channel] = true
	}
	results := make([]deliveryResult, 0, len(channels))
	for _, channel := range channels {
		result := deliveryResult{Channel: channel}
		switch channel {
		case delivery.ChannelEmail:
			result.To = invitation.GuestEmail
		case delivery.ChannelWhatsApp:
			result.To = invitation.GuestPhone
		}
		switch {
		case !active[channel]:
			result.Status, result.Error = "skipped", "channel is not configured"
		case strings.TrimSpace(result.To) == "":
			result.Status, result.Error = "skipped", "no recipient on file"
		default:
			base := strings.TrimRight(strings.TrimSpace(os.Getenv("WEDDINGHUB_PUBLIC_BASE_URL")), "/")
			bannerURL := ""
			if base != "" {
				bannerURL = base + "/og/" + url.PathEscape(wedding.ID) + ".png?v=" + bannerVersion(wedding)[:12]
			}
			message := delivery.Invitation{Channel: channel, To: result.To, Couple: weddingCouple(wedding), Name: invitation.GuestName, Link: link, BannerURL: bannerURL}
			if err := a.sender.Send(r.Context(), message); err != nil {
				result.Status, result.Error = "failed", "delivery failed"
			} else {
				result.Status = "sent"
			}
		}
		results = append(results, result)
	}
	writeJSON(w, http.StatusOK, sendInvitationResponse{WeddingID: wedding.ID, InvitationID: invitation.ID, Results: results})
}

func (a *API) createOwnerTask(w http.ResponseWriter, r *http.Request) {
	_, wedding, ok := a.ownerWedding(w, r)
	if !ok {
		return
	}
	input, valid := parseTaskInput(w, r)
	if !valid {
		return
	}
	if !ownerTextLength(input.Title, 160) || !ownerTextLength(input.Details, 4000) || !ownerTextLength(input.AssignedTo, 160) || len(input.DueOn) > 10 {
		writeError(w, http.StatusBadRequest, "task fields exceed the supported length")
		return
	}
	id := mustID(w)
	if id == "" {
		return
	}
	now := a.now()
	task := models.PlanningTask{
		ID: id, Title: input.Title, Details: input.Details, AssignedTo: input.AssignedTo,
		DueOn: input.DueOn, Status: input.Status, CreatedBy: "Wedding owner", CreatedAt: now, UpdatedAt: now,
	}
	created, err := a.repo.AddPlanningTask(wedding.ID, task)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (a *API) updateOwnerTask(w http.ResponseWriter, r *http.Request) {
	_, wedding, ok := a.ownerWedding(w, r)
	if !ok {
		return
	}
	input, valid := parseTaskInput(w, r)
	if !valid {
		return
	}
	if !ownerTextLength(input.Title, 160) || !ownerTextLength(input.Details, 4000) || !ownerTextLength(input.AssignedTo, 160) || len(input.DueOn) > 10 {
		writeError(w, http.StatusBadRequest, "task fields exceed the supported length")
		return
	}
	task := models.PlanningTask{
		ID: r.PathValue("taskID"), Title: input.Title, Details: input.Details,
		AssignedTo: input.AssignedTo, DueOn: input.DueOn, Status: input.Status, UpdatedAt: a.now(),
	}
	updated, err := a.repo.UpdatePlanningTask(wedding.ID, task)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *API) deleteOwnerTask(w http.ResponseWriter, r *http.Request) {
	_, wedding, ok := a.ownerWedding(w, r)
	if !ok {
		return
	}
	if err := a.repo.DeletePlanningTask(wedding.ID, r.PathValue("taskID")); err != nil {
		writeRepositoryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) createOwnerCommitteeRole(w http.ResponseWriter, r *http.Request) {
	_, wedding, ok := a.ownerWedding(w, r)
	if !ok {
		return
	}
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	if input.Name == "" || !ownerTextLength(input.Name, 100) || !ownerTextLength(input.Description, 1000) {
		writeError(w, http.StatusBadRequest, "role name is required and fields must be within their length limits")
		return
	}
	id := mustID(w)
	if id == "" {
		return
	}
	role := models.CommitteeRole{ID: id, Name: input.Name, Description: input.Description, IsCustom: true, CreatedAt: a.now()}
	created, err := a.repo.AddCommitteeRole(wedding.ID, role)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (a *API) deleteOwnerCommitteeRole(w http.ResponseWriter, r *http.Request) {
	_, wedding, ok := a.ownerWedding(w, r)
	if !ok {
		return
	}
	if err := a.repo.DeleteCommitteeRole(wedding.ID, r.PathValue("roleID")); err != nil {
		writeRepositoryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) updateOwnerCommitteeMember(w http.ResponseWriter, r *http.Request) {
	_, wedding, ok := a.ownerWedding(w, r)
	if !ok {
		return
	}
	var input models.CommitteeMember
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	input.ID = r.PathValue("memberID")
	input.Name = strings.TrimSpace(input.Name)
	input.Email = strings.TrimSpace(input.Email)
	input.Phone = strings.TrimSpace(input.Phone)
	input.Title = strings.TrimSpace(input.Title)
	if input.Name == "" || !ownerTextLength(input.Name, 160) || !ownerTextLength(input.Email, 254) || !ownerTextLength(input.Phone, 40) || !ownerTextLength(input.Title, 100) {
		writeError(w, http.StatusBadRequest, "member name is required and fields must be within their length limits")
		return
	}
	updated, err := a.repo.UpdateCommitteeMember(wedding.ID, input)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *API) deleteOwnerCommitteeMember(w http.ResponseWriter, r *http.Request) {
	_, wedding, ok := a.ownerWedding(w, r)
	if !ok {
		return
	}
	memberID := r.PathValue("memberID")
	for _, member := range wedding.CommitteeMembers {
		if member.ID != memberID {
			continue
		}
		if member.InvitationID != "" {
			err := a.repo.DeleteInvitation(wedding.ID, member.InvitationID)
			if err != nil {
				writeRepositoryError(w, err)
				return
			}
		} else if err := a.repo.DeleteCommitteeMember(wedding.ID, memberID); err != nil {
			writeRepositoryError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeError(w, http.StatusNotFound, "committee member not found")
}

func (a *API) setOwnerMessageRead(w http.ResponseWriter, r *http.Request) {
	_, wedding, ok := a.ownerWedding(w, r)
	if !ok {
		return
	}
	var input struct {
		Read bool `json:"read"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.repo.SetGuestMessageRead(wedding.ID, r.PathValue("messageID"), input.Read); err != nil {
		writeRepositoryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) deleteOwnerMessage(w http.ResponseWriter, r *http.Request) {
	_, wedding, ok := a.ownerWedding(w, r)
	if !ok {
		return
	}
	if err := a.repo.DeleteGuestMessage(wedding.ID, r.PathValue("messageID")); err != nil {
		writeRepositoryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
