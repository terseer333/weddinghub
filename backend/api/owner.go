package api

import (
	"net/http"
	"strings"

	"weddinghub/models"
	"weddinghub/repository"
)

func (a *API) ownerWedding(w http.ResponseWriter, r *http.Request) (models.User, models.Wedding, bool) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return models.User{}, models.Wedding{}, false
	}
	weddings := a.repo.WeddingsForUser(user.ID)
	if len(weddings) == 0 {
		writeError(w, http.StatusNotFound, "no wedding is linked to this account")
		return models.User{}, models.Wedding{}, false
	}
	wedding, err := a.repo.GetWedding(weddings[0].ID)
	if err != nil {
		writeRepositoryError(w, err)
		return models.User{}, models.Wedding{}, false
	}
	return user, wedding, true
}

func (a *API) ownerWorkspace(w http.ResponseWriter, r *http.Request) {
	_, wedding, ok := a.ownerWedding(w, r)
	if !ok {
		return
	}
	if wedding.Admins == nil {
		wedding.Admins = []models.Admin{}
	}
	if wedding.Guests == nil {
		wedding.Guests = []models.Guest{}
	}
	if wedding.CommitteeMembers == nil {
		wedding.CommitteeMembers = []models.CommitteeMember{}
	}
	if wedding.CommitteeRoles == nil {
		wedding.CommitteeRoles = []models.CommitteeRole{}
	}
	if wedding.Invitations == nil {
		wedding.Invitations = []models.Invitation{}
	}
	if wedding.Events == nil {
		wedding.Events = []models.Event{}
	}
	if wedding.Photos == nil {
		wedding.Photos = []models.Photo{}
	}
	if wedding.StorySections == nil {
		wedding.StorySections = []models.StorySection{}
	}
	if wedding.Announcements == nil {
		wedding.Announcements = []models.Announcement{}
	}
	if wedding.PlanningTasks == nil {
		wedding.PlanningTasks = []models.PlanningTask{}
	}
	if wedding.CommitteeChat == nil {
		wedding.CommitteeChat = []models.CommitteeMessage{}
	}
	if wedding.RSVPs == nil {
		wedding.RSVPs = []models.RSVP{}
	}
	if wedding.GuestMessages == nil {
		wedding.GuestMessages = []models.GuestMessage{}
	}
	writeJSON(w, http.StatusOK, wedding)
}

func (a *API) updateOwnerWorkspace(w http.ResponseWriter, r *http.Request) {
	_, current, ok := a.ownerWedding(w, r)
	if !ok {
		return
	}
	var input models.Wedding
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	input.ID = current.ID
	input.Slug = current.Slug
	input.AdminTokenHash = current.AdminTokenHash
	input.CreatedAt = current.CreatedAt
	input.Admins = current.Admins
	input.Guests = current.Guests
	input.CommitteeMembers = current.CommitteeMembers
	input.CommitteeRoles = current.CommitteeRoles
	input.Invitations = current.Invitations
	input.PlanningTasks = current.PlanningTasks
	input.CommitteeChat = current.CommitteeChat
	input.RSVPs = current.RSVPs
	input.GuestMessages = current.GuestMessages
	if input.Events == nil {
		input.Events = current.Events
	}
	if input.Photos == nil {
		input.Photos = current.Photos
	}
	if input.StorySections == nil {
		input.StorySections = current.StorySections
	}
	if input.Announcements == nil {
		input.Announcements = current.Announcements
	}
	input.PartnerOne = strings.TrimSpace(input.PartnerOne)
	input.PartnerTwo = strings.TrimSpace(input.PartnerTwo)
	input.Title = strings.TrimSpace(input.PartnerOne + " & " + input.PartnerTwo)
	if input.Title == "&" {
		writeError(w, http.StatusBadRequest, "both couple names are required")
		return
	}
	input.UpdatedAt = a.now()
	if err := prepareWedding(&input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := a.repo.UpdateWedding(input)
	if err != nil {
		if err == repository.ErrNotFound {
			writeError(w, http.StatusNotFound, "wedding not found")
			return
		}
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}
